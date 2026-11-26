// Package kcadmin changes the one thing the hub writes in Keycloak: a
// person's picture attribute, through the admin API as the hub client's
// service account (realm-management manage-users).
package kcadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("keycloak %s %s: %s %s", method, u, resp.Status, msg)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
