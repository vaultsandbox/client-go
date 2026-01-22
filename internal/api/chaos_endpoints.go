package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/vaultsandbox/client-go/internal/apierrors"
)

// GetInboxChaosConfig retrieves the chaos configuration for an inbox.
func (c *Client) GetInboxChaosConfig(ctx context.Context, emailAddress string) (*ChaosConfigDTO, error) {
	var result ChaosConfigDTO
	path := fmt.Sprintf("/api/inboxes/%s/chaos", url.PathEscape(emailAddress))
	if err := c.Do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceInbox)
	}
	return &result, nil
}

// SetInboxChaosConfig creates or updates the chaos configuration for an inbox.
func (c *Client) SetInboxChaosConfig(ctx context.Context, emailAddress string, req *ChaosConfigRequest) (*ChaosConfigDTO, error) {
	var result ChaosConfigDTO
	path := fmt.Sprintf("/api/inboxes/%s/chaos", url.PathEscape(emailAddress))
	if err := c.Do(ctx, http.MethodPost, path, req, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceInbox)
	}
	return &result, nil
}

// DisableInboxChaos disables all chaos for an inbox.
func (c *Client) DisableInboxChaos(ctx context.Context, emailAddress string) error {
	path := fmt.Sprintf("/api/inboxes/%s/chaos", url.PathEscape(emailAddress))
	return apierrors.WithResourceType(c.Do(ctx, http.MethodDelete, path, nil, nil), apierrors.ResourceInbox)
}
