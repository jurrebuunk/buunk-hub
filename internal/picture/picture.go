// Package picture sets and removes a person's profile picture: the upload
// is re-encoded (internal/avatar), stored in S3 as <sub>/<hash>.jpg, served
// publicly at /avatars/<sub>/<hash>.jpg, and that address is written to the
// person's picture attribute in Keycloak, where other apps read it.
package picture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/jurrebuunk/buunk-hub/internal/avatar"
	"github.com/jurrebuunk/buunk-hub/internal/objects"
)

type Objects interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

type Keycloak interface {
	SetPicture(ctx context.Context, sub, pictureURL string) error
}

// Users is the hub's own cache of the person (users.picture).
type Users interface {
	SetPicture(ctx context.Context, sub, pictureURL string) error
}

type Service struct {
	Objects   Objects
	Keycloak  Keycloak
	Users     Users
	PublicURL *url.URL
	Log       *slog.Logger
}

var (
	subPattern  = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
	filePattern = regexp.MustCompile(`^[0-9a-f]{32}\.jpg$`)
)

// Upload stores a new picture for sub and returns its address. old is the
// current picture, removed afterwards if it is one of ours.
func (s *Service) Upload(ctx context.Context, sub, old string, r io.Reader) (string, error) {
	if !subPattern.MatchString(sub) {
		return "", errors.New("unexpected user id")
	}
	jpg, err := avatar.Process(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(jpg)
	file := hex.EncodeToString(sum[:16]) + ".jpg"
	key := sub + "/" + file
	if err := s.Objects.Put(ctx, key, jpg, "image/jpeg"); err != nil {
		return "", err
	}
	pic := s.PublicURL.JoinPath("/avatars", sub, file).String()
	if err := s.Keycloak.SetPicture(ctx, sub, pic); err != nil {
		s.Objects.Delete(context.WithoutCancel(ctx), key)
		return "", err
	}
	if err := s.Users.SetPicture(ctx, sub, pic); err != nil {
		s.Log.Warn("cache picture", "sub", sub, "err", err)
	}
	if old != pic {
		s.deleteOld(ctx, sub, old)
	}
	return pic, nil
}

// Remove takes the picture away in Keycloak and deletes ours.
func (s *Service) Remove(ctx context.Context, sub, old string) error {
	if err := s.Keycloak.SetPicture(ctx, sub, ""); err != nil {
		return err
	}
	if err := s.Users.SetPicture(ctx, sub, ""); err != nil {
		s.Log.Warn("cache picture", "sub", sub, "err", err)
	}
	s.deleteOld(ctx, sub, old)
	return nil
}

// deleteOld removes the previous picture only if it is this person's own
// file in our bucket; anything else (set by an admin elsewhere) is left.
func (s *Service) deleteOld(ctx context.Context, sub, old string) {
	if key, ok := s.keyOf(sub, old); ok {
		if err := s.Objects.Delete(context.WithoutCancel(ctx), key); err != nil {
			s.Log.Warn("delete old picture", "key", key, "err", err)
		}
	}
}

func (s *Service) keyOf(sub, pic string) (string, bool) {
	prefix := s.PublicURL.JoinPath("/avatars", sub).String() + "/"
	file, ok := strings.CutPrefix(pic, prefix)
	if !ok || !filePattern.MatchString(file) {
		return "", false
	}
	return sub + "/" + file, true
}

// Serve answers GET /avatars/{sub}/{file}: public, no sign-in, so other
// apps and browsers can show the picture. The name is the content's hash,
// so it can be cached forever.
func (s *Service) Serve(w http.ResponseWriter, r *http.Request) {
	sub, file := r.PathValue("sub"), r.PathValue("file")
	if !subPattern.MatchString(sub) || !filePattern.MatchString(file) {
		http.NotFound(w, r)
		return
	}
	data, err := s.Objects.Get(r.Context(), sub+"/"+file)
	if errors.Is(err, objects.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.Log.Error("get picture", "err", err)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/jpeg")
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	// Other sites (Cloudreve) show it.
	h.Set("Cross-Origin-Resource-Policy", "cross-origin")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Write(data)
}
