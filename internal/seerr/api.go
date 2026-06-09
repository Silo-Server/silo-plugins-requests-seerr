package seerr

import (
	"context"
	"fmt"

	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/httpclient"
)

// CreateRequest submits a new media request. A 409 surfaces as a
// *httpclient.StatusError{StatusCode: 409}; an empty 2xx body yields a
// zero-value MediaRequest (ID 0) with nil error.
func CreateRequest(ctx context.Context, c *httpclient.Client, body CreateRequestBody) (*MediaRequest, error) {
	var out MediaRequest
	if err := c.PostJSON(ctx, "/api/v1/request", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRequest fetches a single request by its Seerr id.
func GetRequest(ctx context.Context, c *httpclient.Client, id int) (*MediaRequest, error) {
	var out MediaRequest
	if err := c.GetJSON(ctx, fmt.Sprintf("/api/v1/request/%d", id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FindExistingRequest scans the most recent requests for (tmdbID, is4k). Used on
// the 409 / empty-body recovery path. Returns ErrNotFound if no match is in the
// scanned page.
func FindExistingRequest(ctx context.Context, c *httpclient.Client, tmdbID int, is4k bool) (*MediaRequest, error) {
	var page requestPage
	if err := c.GetJSON(ctx, "/api/v1/request?take=100&sort=added", &page); err != nil {
		return nil, err
	}
	for i := range page.Results {
		r := page.Results[i]
		if r.Media.TMDBID == tmdbID && r.Is4K == is4k {
			return &r, nil
		}
	}
	return nil, ErrNotFound
}

// Me validates the base URL + API key by calling the authenticated /auth/me and
// confirming it returns a real user (a 200 from a login wall / proxy with a
// non-user body must NOT pass).
func Me(ctx context.Context, c *httpclient.Client) error {
	var me struct {
		ID int `json:"id"`
	}
	if err := c.GetJSON(ctx, "/api/v1/auth/me", &me); err != nil {
		return err
	}
	if me.ID <= 0 {
		return fmt.Errorf("seerr: /auth/me did not return an authenticated user")
	}
	return nil
}
