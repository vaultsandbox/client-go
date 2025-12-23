package vaultsandbox

import (
	"context"
)

// GetEmails fetches all emails in the inbox.
func (i *Inbox) GetEmails(ctx context.Context) ([]*Email, error) {
	resp, err := i.client.apiClient.GetEmails(ctx, i.emailAddress)
	if err != nil {
		return nil, err
	}

	emails := make([]*Email, 0, len(resp.Emails))
	for _, e := range resp.Emails {
		email, err := i.decryptEmail(ctx, e)
		if err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}

	return emails, nil
}

// GetEmail fetches a specific email by ID.
func (i *Inbox) GetEmail(ctx context.Context, emailID string) (*Email, error) {
	resp, err := i.client.apiClient.GetEmail(ctx, i.emailAddress, emailID)
	if err != nil {
		return nil, err
	}

	return i.decryptEmail(ctx, resp)
}

// GetRawEmail fetches the raw email content for a specific email.
func (i *Inbox) GetRawEmail(ctx context.Context, emailID string) (string, error) {
	raw, err := i.client.apiClient.GetEmailRaw(ctx, i.emailAddress, emailID)
	if err != nil {
		return "", err
	}
	return raw, nil
}

// MarkEmailAsRead marks a specific email as read.
func (i *Inbox) MarkEmailAsRead(ctx context.Context, emailID string) error {
	return i.client.apiClient.MarkEmailAsRead(ctx, i.emailAddress, emailID)
}

// DeleteEmail deletes a specific email.
func (i *Inbox) DeleteEmail(ctx context.Context, emailID string) error {
	return i.client.apiClient.DeleteEmail(ctx, i.emailAddress, emailID)
}
