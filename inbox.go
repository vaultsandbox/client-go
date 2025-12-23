package vaultsandbox

import (
	"context"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
	"github.com/vaultsandbox/client-go/internal/crypto"
)

// Inbox represents a temporary email inbox.
type Inbox struct {
	emailAddress string
	expiresAt    time.Time
	inboxHash    string
	serverSigPk  []byte
	keypair      *crypto.Keypair
	client       *Client
}

// SyncStatus is a type alias for api.SyncStatus.
// It represents the synchronization status of an inbox.
type SyncStatus = api.SyncStatus

// EmailAddress returns the inbox email address.
func (i *Inbox) EmailAddress() string {
	return i.emailAddress
}

// ExpiresAt returns when the inbox expires.
func (i *Inbox) ExpiresAt() time.Time {
	return i.expiresAt
}

// InboxHash returns the SHA-256 hash of the public key.
func (i *Inbox) InboxHash() string {
	return i.inboxHash
}

// IsExpired checks if the inbox has expired.
func (i *Inbox) IsExpired() bool {
	return time.Now().After(i.expiresAt)
}

// GetSyncStatus retrieves the synchronization status of the inbox.
// This includes the number of emails and a hash of the email list,
// which can be used to efficiently check for changes.
func (i *Inbox) GetSyncStatus(ctx context.Context) (*SyncStatus, error) {
	return i.client.apiClient.GetInboxSync(ctx, i.emailAddress)
}

// Delete deletes the inbox.
func (i *Inbox) Delete(ctx context.Context) error {
	return i.client.DeleteInbox(ctx, i.emailAddress)
}

func newInboxFromResult(resp *api.CreateInboxResult, c *Client) *Inbox {
	return &Inbox{
		emailAddress: resp.EmailAddress,
		expiresAt:    resp.ExpiresAt,
		inboxHash:    resp.InboxHash,
		serverSigPk:  resp.ServerSigPk,
		keypair:      resp.Keypair,
		client:       c,
	}
}
