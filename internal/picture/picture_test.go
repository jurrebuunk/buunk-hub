package picture

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jurrebuunk/buunk-hub/internal/objects"
)

type memObjects map[string][]byte

func (m memObjects) Put(_ context.Context, k string, d []byte, _ string) error { m[k] = d; return nil }
func (m memObjects) Delete(_ context.Context, k string) error                  { delete(m, k); return nil }
func (m memObjects) Get(_ context.Context, k string) ([]byte, error) {
	if d, ok := m[k]; ok {
		return d, nil
	}
	return nil, objects.ErrNotFound
}

type memPictures struct {
	m    map[string]string
	fail bool
}

func (p *memPictures) SetPicture(_ context.Context, sub, u string) error {
	if p.fail {
		return errors.New("keycloak down")
	}
	p.m[sub] = u
	return nil
}

func newService() (*Service, memObjects, *memPictures, *memPictures) {
	pub, _ := url.Parse("https://hub.example.test")
	obj, kc, users := memObjects{}, &memPictures{m: map[string]string{}}, &memPictures{m: map[string]string{}}
	return &Service{Objects: obj, Keycloak: kc, Users: users, PublicURL: pub,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, obj, kc, users
}

func pngPicture(c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func TestUploadReplaceRemove(t *testing.T) {
	s, obj, kc, users := newService()
	ctx := context.Background()
	first, err := s.Upload(ctx, "ann-1", "", bytes.NewReader(pngPicture(color.White)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first, "https://hub.example.test/avatars/ann-1/") || kc.m["ann-1"] != first || users.m["ann-1"] != first {
		t.Fatalf("first: %s, keycloak %q, cache %q", first, kc.m["ann-1"], users.m["ann-1"])
	}
	second, err := s.Upload(ctx, "ann-1", first, bytes.NewReader(pngPicture(color.Black)))
	if err != nil || second == first {
		t.Fatal(second, err)
	}
	if len(obj) != 1 {
		t.Errorf("old picture not deleted: %d objects", len(obj))
	}
	if err := s.Remove(ctx, "ann-1", second); err != nil {
		t.Fatal(err)
	}
	if len(obj) != 0 || kc.m["ann-1"] != "" {
		t.Errorf("after remove: %d objects, keycloak %q", len(obj), kc.m["ann-1"])
	}
}

// Someone else's picture, or one set elsewhere, is never deleted.
func TestOnlyOwnFilesDeleted(t *testing.T) {
	s, obj, _, _ := newService()
	obj["bob/0123456789abcdef0123456789abcdef.jpg"] = []byte("bob")
	for _, old := range []string{
		"https://hub.example.test/avatars/bob/0123456789abcdef0123456789abcdef.jpg",
		"https://elsewhere.test/avatars/ann/0123456789abcdef0123456789abcdef.jpg",
		"https://hub.example.test/avatars/ann/../bob/0123456789abcdef0123456789abcdef.jpg",
	} {
		if err := s.Remove(context.Background(), "ann", old); err != nil {
			t.Fatal(err)
		}
	}
	if len(obj) != 1 {
		t.Error("deleted a file that isn't ann's")
	}
}

func TestKeycloakFailureLeavesNothing(t *testing.T) {
	s, obj, kc, _ := newService()
	kc.fail = true
	if _, err := s.Upload(context.Background(), "ann", "", bytes.NewReader(pngPicture(color.White))); err == nil {
		t.Fatal("no error")
	}
	if len(obj) != 0 {
		t.Error("stored a picture Keycloak doesn't know")
	}
}

func TestServe(t *testing.T) {
	s, obj, _, _ := newService()
	obj["ann/0123456789abcdef0123456789abcdef.jpg"] = []byte("jpg")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /avatars/{sub}/{file}", s.Serve)
	for path, want := range map[string]int{
		"/avatars/ann/0123456789abcdef0123456789abcdef.jpg":   200,
		"/avatars/ann/ffffffffffffffffffffffffffffffff.jpg":   404,
		"/avatars/ann/0123456789abcdef0123456789abcdef.png":   404,
		"/avatars/ann/..%2F..%2Fsecret":                       404,
		"/avatars/a%20b/0123456789abcdef0123456789abcdef.jpg": 404,
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != want {
			t.Errorf("%s: %d, want %d", path, rec.Code, want)
		}
		if want == 200 && (rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || rec.Header().Get("Content-Type") != "image/jpeg") {
			t.Errorf("%s: headers %v", path, rec.Header())
		}
	}
}
