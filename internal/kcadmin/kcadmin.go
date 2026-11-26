// Package kcadmin is what the hub reads and writes of a person in Keycloak,
// through the admin API as the hub client's service account
// (realm-management manage-users): their picture attribute, their name and
// email (Settings and the first-visit dialog), and the groups they are in.
package kcadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type Client struct {
	// AdminURL is the realm's admin API, like
	// http://keycloak-service.keycloak.svc.cluster.local:8080/admin/realms/hgd1
	// (/admin is not public).
	AdminURL string
	http     *http.Client
}

func New(ctx context.Context, adminURL, tokenURL, clientID, clientSecret string) *Client {
	cc := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		// The admin API's audience (Keycloak 26.8); only in this token.
		Scopes:    []string{"kc-admin-api"},
		AuthStyle: oauth2.AuthStyleInHeader,
	}
	base := &http.Client{Timeout: 10 * time.Second}
	ctx = context.WithValue(context.WithoutCancel(ctx), oauth2.HTTPClient, base)
	h := oauth2.NewClient(ctx, cc.TokenSource(ctx))
	h.Timeout = 10 * time.Second
	return &Client{AdminURL: adminURL, http: h}
}

// SetPicture sets the picture attribute of the user with id sub; an empty
// URL removes it. The user is read and written back whole: the admin API
// replaces the attributes with what it gets.
func (c *Client) SetPicture(ctx context.Context, sub, pictureURL string) error {
	u := c.AdminURL + "/users/" + url.PathEscape(sub)
	var user map[string]any
	if err := c.do(ctx, http.MethodGet, u, nil, &user); err != nil {
		return err
	}
	attrs, _ := user["attributes"].(map[string]any)
	if attrs == nil {
		attrs = map[string]any{}
	}
	if pictureURL == "" {
		delete(attrs, "picture")
	} else {
		attrs["picture"] = []string{pictureURL}
	}
	user["attributes"] = attrs
	return c.do(ctx, http.MethodPut, u, user, nil)
}

var (
	// ErrEmailTaken: another account has that email.
	ErrEmailTaken = errors.New("email in use by another account")
	// ErrInvalid: Keycloak's user profile refused a value.
	ErrInvalid = errors.New("refused by the user profile")
)

// Profile is the part of a person's account they can change in the hub.
type Profile struct {
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
}

func (c *Client) userURL(sub string) string { return c.AdminURL + "/users/" + url.PathEscape(sub) }

// Profile reads the person's name and email.
func (c *Client) Profile(ctx context.Context, sub string) (Profile, error) {
	var p Profile
	err := c.do(ctx, http.MethodGet, c.userURL(sub), nil, &p)
	return p, err
}

// SetProfile changes the name and email. A new email is unverified until
// the person follows the link Keycloak mails them (realm verifyEmail), so
// it can't be used to take over someone else's address unnoticed.
func (c *Client) SetProfile(ctx context.Context, sub string, p Profile) (emailChanged bool, err error) {
	u := c.userURL(sub)
	var user map[string]any
	if err := c.do(ctx, http.MethodGet, u, nil, &user); err != nil {
		return false, err
	}
	old, _ := user["email"].(string)
	emailChanged = !strings.EqualFold(old, p.Email)
	user["firstName"], user["lastName"], user["email"] = p.FirstName, p.LastName, p.Email
	if emailChanged {
		user["emailVerified"] = false
	}
	if err := c.do(ctx, http.MethodPut, u, user, nil); err != nil {
		return false, err
	}
	if emailChanged {
		// The change stands either way; the person can ask for the mail again
		// at sign-in.
		err = c.do(ctx, http.MethodPut, u+"/send-verify-email?client_id=hub", nil, nil)
	}
	return emailChanged, err
}

// Groups returns the names of the groups the person is in.
func (c *Client) Groups(ctx context.Context, sub string) ([]string, error) {
	var gs []struct {
		Name string `json:"name"`
	}
	if err := c.do(ctx, http.MethodGet, c.userURL(sub)+"/groups?briefRepresentation=true", nil, &gs); err != nil {
		return nil, err
	}
	names := make([]string, len(gs))
	for i, g := range gs {
		names[i] = g.Name
	}
	return names, nil
}

func (c *Client) do(ctx context.Context, method, u string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusConflict:
		return ErrEmailTaken
	case http.StatusBadRequest:
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%w: %s", ErrInvalid, msg)
	}
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("keycloak %s %s: %s %s", method, u, resp.Status, msg)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
