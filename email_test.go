package vaultsandbox

import (
	"testing"
	"time"
)

func TestEmail_Fields(t *testing.T) {
	email := &Email{
		ID:         "email123",
		From:       "sender@example.com",
		To:         []string{"recipient@example.com"},
		Subject:    "Test Subject",
		Text:       "Plain text body",
		HTML:       "<p>HTML body</p>",
		ReceivedAt: time.Now(),
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Links:      []string{"https://example.com"},
		IsRead:     false,
	}

	if email.ID != "email123" {
		t.Errorf("ID = %s, want email123", email.ID)
	}
	if email.From != "sender@example.com" {
		t.Errorf("From = %s, want sender@example.com", email.From)
	}
	if len(email.To) != 1 || email.To[0] != "recipient@example.com" {
		t.Errorf("To = %v, want [recipient@example.com]", email.To)
	}
	if email.Subject != "Test Subject" {
		t.Errorf("Subject = %s, want Test Subject", email.Subject)
	}
	if email.Text != "Plain text body" {
		t.Errorf("Text = %s, want Plain text body", email.Text)
	}
	if email.HTML != "<p>HTML body</p>" {
		t.Errorf("HTML = %s, want <p>HTML body</p>", email.HTML)
	}
	if email.IsRead != false {
		t.Error("IsRead = true, want false")
	}
}

func TestAttachment_Fields(t *testing.T) {
	attachment := Attachment{
		Filename:           "document.pdf",
		ContentType:        "application/pdf",
		Size:               1024,
		ContentID:          "cid123",
		ContentDisposition: "attachment",
		Content:            []byte("fake pdf content"),
		Checksum:           "abc123",
	}

	if attachment.Filename != "document.pdf" {
		t.Errorf("Filename = %s, want document.pdf", attachment.Filename)
	}
	if attachment.ContentType != "application/pdf" {
		t.Errorf("ContentType = %s, want application/pdf", attachment.ContentType)
	}
	if attachment.Size != 1024 {
		t.Errorf("Size = %d, want 1024", attachment.Size)
	}
	if attachment.ContentID != "cid123" {
		t.Errorf("ContentID = %s, want cid123", attachment.ContentID)
	}
	if attachment.ContentDisposition != "attachment" {
		t.Errorf("ContentDisposition = %s, want attachment", attachment.ContentDisposition)
	}
	if string(attachment.Content) != "fake pdf content" {
		t.Errorf("Content = %s, want fake pdf content", string(attachment.Content))
	}
	if attachment.Checksum != "abc123" {
		t.Errorf("Checksum = %s, want abc123", attachment.Checksum)
	}
}

func TestEmail_WithAttachments(t *testing.T) {
	email := &Email{
		ID:      "email123",
		Subject: "With Attachments",
		Attachments: []Attachment{
			{Filename: "file1.txt", Size: 100},
			{Filename: "file2.pdf", Size: 2000},
		},
	}

	if len(email.Attachments) != 2 {
		t.Errorf("Attachments length = %d, want 2", len(email.Attachments))
	}
	if email.Attachments[0].Filename != "file1.txt" {
		t.Errorf("Attachments[0].Filename = %s, want file1.txt", email.Attachments[0].Filename)
	}
	if email.Attachments[1].Filename != "file2.pdf" {
		t.Errorf("Attachments[1].Filename = %s, want file2.pdf", email.Attachments[1].Filename)
	}
}

// Note: Full email tests require a real API connection
// These tests verify the data structures
// Integration tests are in the integration/ directory
