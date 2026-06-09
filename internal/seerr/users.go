package seerr

import (
	"context"
	"fmt"
	"strings"

	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/httpclient"
)

// FindUserByEmail scans Seerr's user list for a case-insensitive email match.
// Returns nil (no error) when no user matches.
func FindUserByEmail(ctx context.Context, c *httpclient.Client, email string) (*User, error) {
	target := strings.ToLower(strings.TrimSpace(email))
	if target == "" {
		return nil, nil
	}
	const take = 100
	for skip := 0; skip < 10000; skip += take {
		var page userPage
		if err := c.GetJSON(ctx, fmt.Sprintf("/api/v1/user?take=%d&skip=%d", take, skip), &page); err != nil {
			return nil, err
		}
		for i := range page.Results {
			if strings.ToLower(strings.TrimSpace(page.Results[i].Email)) == target {
				return &page.Results[i], nil
			}
		}
		if len(page.Results) < take {
			break
		}
	}
	return nil, nil
}

// CreateUser creates a Seerr local user with the given email + permission bitfield.
func CreateUser(ctx context.Context, c *httpclient.Client, email string, permissions int) (*User, error) {
	var out User
	body := map[string]any{"email": email, "permissions": permissions}
	if err := c.PostJSON(ctx, "/api/v1/user", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
