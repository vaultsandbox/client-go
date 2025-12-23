package vaultsandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vaultsandbox/client-go/authresults"
	"github.com/vaultsandbox/client-go/internal/api"
	"github.com/vaultsandbox/client-go/internal/crypto"
)

func (i *Inbox) decryptEmail(ctx context.Context, raw *api.RawEmail) (*Email, error) {
	if raw.EncryptedMetadata == nil {
		return nil, fmt.Errorf("email has no encrypted metadata")
	}

	// Fetch full email if we don't have parsed content
	emailData := raw
	if raw.EncryptedParsed == nil {
		fullEmail, err := i.client.apiClient.GetEmail(ctx, i.emailAddress, raw.ID)
		if err != nil {
			return nil, fmt.Errorf("fetch full email: %w", err)
		}
		emailData = fullEmail
	}

	// Verify and decrypt metadata
	metadataPlaintext, err := i.verifyAndDecrypt(emailData.EncryptedMetadata)
	if err != nil {
		return nil, err
	}

	metadata, err := parseMetadata(metadataPlaintext)
	if err != nil {
		return nil, err
	}

	// Build decrypted email from metadata
	decrypted := buildDecryptedEmail(emailData, metadata)

	// Decrypt and apply parsed content if available
	if emailData.EncryptedParsed != nil {
		if err := i.applyParsedContent(emailData.EncryptedParsed, decrypted); err != nil {
			return nil, err
		}
	}

	return i.convertDecryptedEmail(decrypted), nil
}

// applyParsedContent decrypts parsed content and applies it to the decrypted email.
func (i *Inbox) applyParsedContent(encrypted *crypto.EncryptedPayload, decrypted *crypto.DecryptedEmail) error {
	parsedPlaintext, err := i.verifyAndDecrypt(encrypted)
	if err != nil {
		return err
	}

	parsed, headers, err := parseParsedContent(parsedPlaintext)
	if err != nil {
		return err
	}

	decrypted.Text = parsed.Text
	decrypted.HTML = parsed.HTML
	decrypted.Attachments = parsed.Attachments
	decrypted.Links = parsed.Links
	decrypted.AuthResults = parsed.AuthResults
	decrypted.Headers = headers

	return nil
}

func (i *Inbox) convertDecryptedEmail(d *crypto.DecryptedEmail) *Email {
	attachments := make([]Attachment, len(d.Attachments))
	for j, a := range d.Attachments {
		attachments[j] = Attachment{
			Filename:           a.Filename,
			ContentType:        a.ContentType,
			Size:               a.Size,
			ContentID:          a.ContentID,
			ContentDisposition: a.ContentDisposition,
			Content:            a.Content,
			Checksum:           a.Checksum,
		}
	}

	email := &Email{
		ID:          d.ID,
		From:        d.From,
		To:          d.To,
		Subject:     d.Subject,
		Text:        d.Text,
		HTML:        d.HTML,
		ReceivedAt:  d.ReceivedAt,
		Headers:     d.Headers,
		Attachments: attachments,
		Links:       d.Links,
		IsRead:      d.IsRead,
	}

	// Unmarshal AuthResults if present
	if len(d.AuthResults) > 0 {
		var ar authresults.AuthResults
		if err := json.Unmarshal(d.AuthResults, &ar); err == nil {
			email.AuthResults = &ar
		}
	}

	return email
}

// verifyAndDecrypt verifies the signature and decrypts an encrypted payload.
// It returns the decrypted plaintext or an error if verification/decryption fails.
func (i *Inbox) verifyAndDecrypt(payload *crypto.EncryptedPayload) ([]byte, error) {
	if err := crypto.VerifySignature(payload, i.serverSigPk); err != nil {
		return nil, wrapCryptoError(err)
	}
	return crypto.Decrypt(payload, i.keypair)
}

// parseMetadata unmarshals decrypted metadata JSON into a DecryptedMetadata struct.
func parseMetadata(plaintext []byte) (*crypto.DecryptedMetadata, error) {
	var metadata crypto.DecryptedMetadata
	if err := json.Unmarshal(plaintext, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted metadata: %w", err)
	}
	return &metadata, nil
}

// parseParsedContent unmarshals decrypted parsed content JSON and converts headers.
// Headers are converted from interface{} to string map, preserving only string values.
func parseParsedContent(plaintext []byte) (*crypto.DecryptedParsed, map[string]string, error) {
	var parsed crypto.DecryptedParsed
	if err := json.Unmarshal(plaintext, &parsed); err != nil {
		return nil, nil, fmt.Errorf("failed to parse decrypted parsed content: %w", err)
	}

	// Convert headers from interface{} to string map.
	// The server may send headers with non-string values, but for type safety
	// we only preserve string-typed values.
	var headers map[string]string
	if len(parsed.Headers) > 0 {
		headers = make(map[string]string)
		for k, v := range parsed.Headers {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
	}

	return &parsed, headers, nil
}

// buildDecryptedEmail constructs a DecryptedEmail from raw email data and metadata.
// It handles receivedAt fallback logic when metadata timestamp is missing or invalid.
func buildDecryptedEmail(emailData *api.RawEmail, metadata *crypto.DecryptedMetadata) *crypto.DecryptedEmail {
	decrypted := &crypto.DecryptedEmail{
		ID:      emailData.ID,
		From:    metadata.From,
		To:      []string{metadata.To},
		Subject: metadata.Subject,
		IsRead:  emailData.IsRead,
	}

	// Parse receivedAt from metadata, fallback to API timestamp
	if metadata.ReceivedAt != "" {
		if t, err := time.Parse(time.RFC3339, metadata.ReceivedAt); err == nil {
			decrypted.ReceivedAt = t
		}
	}
	if decrypted.ReceivedAt.IsZero() {
		decrypted.ReceivedAt = emailData.ReceivedAt
	}

	return decrypted
}

// wrapCryptoError converts internal crypto errors to public sentinel errors
// so that errors.Is() checks work correctly.
func wrapCryptoError(err error) error {
	if err == nil {
		return nil
	}

	// Map internal crypto errors to public sentinel errors
	if errors.Is(err, crypto.ErrServerKeyMismatch) {
		return &SignatureVerificationError{Message: err.Error(), IsKeyMismatch: true}
	}
	if errors.Is(err, crypto.ErrSignatureVerificationFailed) {
		return &SignatureVerificationError{Message: err.Error(), IsKeyMismatch: false}
	}

	return err
}
