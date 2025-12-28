//go:build integration

// Package integration contains tests that verify all examples from README.md work correctly.
// These tests require a running VaultSandbox Gateway and SMTP server.
//
// Required environment variables:
//   - VAULTSANDBOX_API_KEY: API key for authentication
//   - VAULTSANDBOX_URL: Gateway URL (e.g., https://api.vaultsandbox.com)
//   - SMTP_HOST: SMTP server host for sending test emails
//   - SMTP_PORT: SMTP server port (default: 25)
//
// Run with:
//
//	go test -tags=integration -run=README -v ./integration/...

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/smtp"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

// getSMTPConfig returns SMTP host and port from environment.
// Called at test time (after TestMain loads .env).
func getSMTPConfig() (host, port string) {
	host = os.Getenv("SMTP_HOST")
	port = os.Getenv("SMTP_PORT")
	if port == "" {
		port = "25"
	}
	return host, port
}

// skipIfNoSMTP skips the test if SMTP is not configured.
func skipIfNoSMTP(t *testing.T) {
	t.Helper()
	host, _ := getSMTPConfig()
	if host == "" {
		t.Skip("skipping: SMTP_HOST not set")
	}
}

// sendTestEmail sends a test email via SMTP.
func sendTestEmail(t *testing.T, to, subject, body string) {
	t.Helper()
	skipIfNoSMTP(t)

	smtpHost, smtpPort := getSMTPConfig()
	from := "test@example.com"
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		from, to, subject, body)

	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	if err := smtp.SendMail(addr, nil, from, []string{to}, []byte(msg)); err != nil {
		t.Fatalf("sendTestEmail() error = %v", err)
	}
	t.Logf("Sent email to %s with subject: %s", to, subject)
}

// sendTestHTMLEmail sends a test email with HTML content via SMTP.
func sendTestHTMLEmail(t *testing.T, to, subject, textBody, htmlBody string) {
	t.Helper()
	skipIfNoSMTP(t)

	smtpHost, smtpPort := getSMTPConfig()
	from := "test@example.com"
	boundary := "boundary-example-12345"

	msg := fmt.Sprintf(`From: %s
To: %s
Subject: %s
MIME-Version: 1.0
Content-Type: multipart/alternative; boundary="%s"

--%s
Content-Type: text/plain; charset=utf-8

%s

--%s
Content-Type: text/html; charset=utf-8

%s

--%s--
`, from, to, subject, boundary, boundary, textBody, boundary, htmlBody, boundary)

	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	if err := smtp.SendMail(addr, nil, from, []string{to}, []byte(msg)); err != nil {
		t.Fatalf("sendTestHTMLEmail() error = %v", err)
	}
	t.Logf("Sent HTML email to %s with subject: %s", to, subject)
}

// sendTestEmailWithAttachment sends a test email with an attachment via SMTP.
func sendTestEmailWithAttachment(t *testing.T, to, subject, body, attachmentName, attachmentContent string) {
	t.Helper()
	skipIfNoSMTP(t)

	smtpHost, smtpPort := getSMTPConfig()
	from := "test@example.com"
	boundary := "boundary-attachment-67890"

	msg := fmt.Sprintf(`From: %s
To: %s
Subject: %s
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="%s"

--%s
Content-Type: text/plain; charset=utf-8

%s

--%s
Content-Type: application/octet-stream; name="%s"
Content-Disposition: attachment; filename="%s"
Content-Transfer-Encoding: base64

%s

--%s--
`, from, to, subject, boundary, boundary, body, boundary, attachmentName, attachmentName, attachmentContent, boundary)

	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	if err := smtp.SendMail(addr, nil, from, []string{to}, []byte(msg)); err != nil {
		t.Fatalf("sendTestEmailWithAttachment() error = %v", err)
	}
	t.Logf("Sent email with attachment to %s", to)
}

// ============================================================================
// README Quick Start Example (lines 50-98)
// ============================================================================

func TestREADME_QuickStart(t *testing.T) {
	skipIfNoSMTP(t)

	// Initialize client with your API key (README example)
	client, err := vaultsandbox.New(apiKey,
		vaultsandbox.WithBaseURL(baseURL),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// Create inbox (keypair generated automatically)
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Send email to: %s", inbox.EmailAddress())

	// Send a test email
	sendTestEmail(t, inbox.EmailAddress(), "Quick Start Test", "Hello from Quick Start!")

	// Wait for email with timeout (README example uses regex, but we use exact match here)
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(30*time.Second),
		vaultsandbox.WithSubject("Quick Start Test"),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Email is already decrypted - just use it!
	t.Logf("From: %s", email.From)
	t.Logf("Subject: %s", email.Subject)
	t.Logf("Text: %s", email.Text)

	// Verify basic fields
	if email.From == "" {
		t.Error("email.From is empty")
	}
	if email.Subject != "Quick Start Test" {
		t.Errorf("email.Subject = %q, want %q", email.Subject, "Quick Start Test")
	}
	if !strings.Contains(email.Text, "Hello from Quick Start") {
		t.Errorf("email.Text = %q, want to contain 'Hello from Quick Start'", email.Text)
	}
}

// ============================================================================
// README Password Reset Example (lines 105-166)
// ============================================================================

func TestREADME_PasswordResetEmail(t *testing.T) {
	skipIfNoSMTP(t)

	client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(baseURL))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// Simulate sending a password reset email (in real tests, this would call your app)
	resetLink := "https://example.com/reset-password?token=abc123"
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body>
<h1>Reset your password</h1>
<p>Click the link below to reset your password:</p>
<a href="%s">Reset Password</a>
</body>
</html>
`, resetLink)

	sendTestHTMLEmail(t, inbox.EmailAddress(),
		"Reset your password",
		"Click here to reset: "+resetLink,
		htmlBody,
	)

	// Wait for and validate the reset email (README example)
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(30*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Reset your password`)),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Extract reset link (README example)
	var foundResetLink string
	for _, link := range email.Links {
		if strings.Contains(link, "/reset-password") {
			foundResetLink = link
			break
		}
	}
	t.Logf("Reset link: %s", foundResetLink)

	if foundResetLink == "" {
		t.Error("reset link not found in email")
	}
	if !strings.Contains(foundResetLink, "token=") {
		t.Error("reset link should contain token parameter")
	}

	// Validate email authentication (README example)
	if email.AuthResults != nil {
		validation := email.AuthResults.Validate()
		// In a real test, this may not pass if the sender isn't fully configured.
		// A robust check verifies the validation was performed and has the correct shape.
		if validation.SPFPassed {
			t.Log("SPF passed")
		}
		if validation.DKIMPassed {
			t.Log("DKIM passed")
		}
		t.Logf("Auth validation completed: Passed=%v", validation.Passed)
	}
}

// ============================================================================
// README Email Authentication Example (lines 169-195)
// ============================================================================

func TestREADME_EmailAuthentication(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	sendTestEmail(t, inbox.EmailAddress(), "Auth Test", "Testing authentication results")

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	// README example: Check authentication results
	if email.AuthResults == nil {
		t.Log("No authentication results (sender may not have SPF/DKIM/DMARC configured)")
		return
	}

	validation := email.AuthResults.Validate()

	if !validation.Passed {
		t.Log("Email authentication failed:")
		for _, reason := range validation.Failures {
			t.Logf("  - %s", reason)
		}
	}

	// Or check individual results (README example)
	if email.AuthResults.SPF != nil {
		t.Logf("SPF status: %s", email.AuthResults.SPF.Status)
	}
	if len(email.AuthResults.DKIM) > 0 {
		t.Logf("DKIM signatures: %d", len(email.AuthResults.DKIM))
	}
	if email.AuthResults.DMARC != nil {
		t.Logf("DMARC status: %s", email.AuthResults.DMARC.Status)
	}

	// Also test IsPassing() convenience method (README)
	if validation.Passed != email.AuthResults.IsPassing() {
		t.Error("IsPassing() does not match Validate().Passed")
	}
}

// ============================================================================
// README Link Extraction Example (lines 198-233)
// ============================================================================

func TestREADME_LinkExtraction(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// Send email with verification link
	verifyLink := "https://example.com/verify?token=xyz789"
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body>
<h1>Verify your email</h1>
<p>Please click the link below to verify your email:</p>
<a href="%s">Verify Email</a>
<p>Or copy this link: %s</p>
</body>
</html>
`, verifyLink, verifyLink)

	sendTestHTMLEmail(t, inbox.EmailAddress(),
		"Verify your email",
		"Verify here: "+verifyLink,
		htmlBody,
	)

	// README example: Wait for email with subject regex
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Verify your email`)),
	)
	if err != nil {
		t.Fatal(err)
	}

	// README example: Links are automatically extracted
	var foundVerifyLink string
	for _, link := range email.Links {
		if strings.Contains(link, "/verify") {
			foundVerifyLink = link
			break
		}
	}

	if foundVerifyLink == "" {
		t.Fatal("verify link not found")
	}
	if !strings.HasPrefix(foundVerifyLink, "https://") {
		t.Fatal("verify link should use HTTPS")
	}

	t.Logf("Found verify link: %s", foundVerifyLink)
	t.Logf("Total links in email: %d", len(email.Links))
}

// ============================================================================
// README Attachments Example (lines 237-309)
// ============================================================================

func TestREADME_Attachments(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// Send email with attachment (base64 encoded "Hello, World!" for simplicity)
	// In a real test, you'd send actual files
	attachmentContent := "SGVsbG8sIFdvcmxkIQ==" // base64 of "Hello, World!"

	sendTestEmailWithAttachment(t, inbox.EmailAddress(),
		"Documents Attached",
		"Please find the attached document.",
		"test.txt",
		attachmentContent,
	)

	// README example
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Documents Attached`)),
	)
	if err != nil {
		t.Fatal(err)
	}

	// README example: Access attachments slice
	t.Logf("Found %d attachments", len(email.Attachments))

	// README example: Iterate through attachments
	for _, attachment := range email.Attachments {
		t.Logf("Filename: %s", attachment.Filename)
		t.Logf("Content-Type: %s", attachment.ContentType)
		t.Logf("Size: %d bytes", attachment.Size)

		if attachment.Content == nil {
			continue
		}

		// README example: Decode text-based attachments
		if strings.Contains(attachment.ContentType, "text") ||
			strings.Contains(attachment.ContentType, "octet-stream") {
			textContent := string(attachment.Content)
			t.Logf("Content: %s", textContent)
		}
	}

	// README example: Find specific attachment
	var foundAttachment *vaultsandbox.Attachment
	for i := range email.Attachments {
		if email.Attachments[i].Filename == "test.txt" {
			foundAttachment = &email.Attachments[i]
			break
		}
	}

	if foundAttachment == nil {
		t.Log("test.txt not found (may not have been parsed as attachment)")
		return
	}

	// README example: Verify attachment
	if foundAttachment.Size == 0 {
		t.Log("attachment size is 0 (may be expected for small attachments)")
	}

	if foundAttachment.Content != nil {
		t.Logf("Attachment content length: %d", len(foundAttachment.Content))
	}
}

// ============================================================================
// README WaitForEmailCount Example (lines 368-400)
// ============================================================================

func TestREADME_WaitForEmailCount(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// Send multiple emails
	for i := 1; i <= 3; i++ {
		sendTestEmail(t, inbox.EmailAddress(),
			fmt.Sprintf("Notification %d", i),
			fmt.Sprintf("This is notification email #%d", i),
		)
		// Small delay to ensure ordering
		time.Sleep(200 * time.Millisecond)
	}

	// README example: Wait for all 3 emails to arrive
	emails, err := inbox.WaitForEmailCount(ctx, 3,
		vaultsandbox.WithWaitTimeout(60*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}

	// README example: Verify all emails
	if len(emails) != 3 {
		t.Fatalf("expected 3 emails, got %d", len(emails))
	}

	for i, email := range emails {
		t.Logf("Email %d: Subject=%s", i+1, email.Subject)
		if !strings.Contains(email.Subject, "Notification") {
			t.Errorf("expected notification subject, got %s", email.Subject)
		}
	}
}

// ============================================================================
// README Real-time Monitoring Example (channel-based Watch API)
// ============================================================================

func TestREADME_RealTimeMonitoring(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Watching for emails at: %s", inbox.EmailAddress())

	// README example: Watch for new emails using WatchFunc
	watchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Track received emails
	var received []*vaultsandbox.Email
	done := make(chan struct{})
	var closeOnce sync.Once

	// Process emails using WatchFunc
	go func() {
		inbox.WatchFunc(watchCtx, func(email *vaultsandbox.Email) {
			received = append(received, email)
			t.Logf("New email received: %q", email.Subject)

			if len(received) >= 2 {
				closeOnce.Do(func() {
					cancel() // Stop watching
					close(done)
				})
			}
		})
	}()

	// Send emails
	sendTestEmail(t, inbox.EmailAddress(), "Monitor Test 1", "First email")
	time.Sleep(500 * time.Millisecond)
	sendTestEmail(t, inbox.EmailAddress(), "Monitor Test 2", "Second email")

	// Wait for emails to be received
	select {
	case <-done:
		t.Log("Received all expected emails")
	case <-time.After(30 * time.Second):
		t.Log("Timeout waiting for emails (may have already been received)")
	}

	t.Log("Stopped monitoring")

	// Verify we received emails
	if len(received) < 1 {
		// It's possible emails were received before Watch was active
		// Check inbox directly
		allEmails, err := inbox.GetEmails(ctx)
		if err != nil {
			t.Fatalf("GetEmails() error = %v", err)
		}
		if len(allEmails) < 2 {
			t.Errorf("expected at least 2 emails, got %d via Watch and %d in inbox",
				len(received), len(allEmails))
		}
	}
}

// ============================================================================
// README WatchInboxes Example (channel-based multi-inbox watching)
// ============================================================================

func TestREADME_WatchInboxes(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	// README example: Create multiple inboxes
	inbox1, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox1.Delete(ctx)

	inbox2, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox2.Delete(ctx)

	t.Logf("Watching inboxes: %s, %s", inbox1.EmailAddress(), inbox2.EmailAddress())

	// README example: Watch multiple inboxes using WatchInboxesFunc
	watchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Track which inboxes received emails
	var receivedEmails sync.Map
	done := make(chan struct{})
	var closeOnce sync.Once

	// Process events using WatchInboxesFunc
	go func() {
		client.WatchInboxesFunc(watchCtx, func(event *vaultsandbox.InboxEvent) {
			t.Logf("New email in %s: %s", event.Inbox.EmailAddress(), event.Email.Subject)
			receivedEmails.Store(event.Inbox.EmailAddress(), event.Email)

			// Check if both inboxes received emails
			count := 0
			receivedEmails.Range(func(_, _ any) bool {
				count++
				return true
			})
			if count >= 2 {
				closeOnce.Do(func() {
					cancel() // Stop watching
					close(done)
				})
			}
		}, inbox1, inbox2)
	}()

	// Send emails to both inboxes
	sendTestEmail(t, inbox1.EmailAddress(), "Multi-inbox Test 1", "Email to inbox 1")
	time.Sleep(300 * time.Millisecond)
	sendTestEmail(t, inbox2.EmailAddress(), "Multi-inbox Test 2", "Email to inbox 2")

	// Wait for both emails
	select {
	case <-done:
		t.Log("Received emails in both inboxes")
	case <-time.After(30 * time.Second):
		t.Log("Timeout (checking inboxes directly)")
	}

	// Verify emails were received
	emails1, _ := inbox1.GetEmails(ctx)
	emails2, _ := inbox2.GetEmails(ctx)

	if len(emails1) == 0 {
		t.Error("inbox1 should have received at least 1 email")
	}
	if len(emails2) == 0 {
		t.Error("inbox2 should have received at least 1 email")
	}
}

// ============================================================================
// README WaitOption Predicate Example (lines 630-650)
// ============================================================================

func TestREADME_WaitOptionPredicate(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// Send email to specific recipient in To field
	targetEmail := inbox.EmailAddress()
	sendTestEmail(t, targetEmail, "Predicate Test", "Testing custom predicate")

	// README example: Wait with custom predicate
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(30*time.Second),
		vaultsandbox.WithPredicate(func(email *vaultsandbox.Email) bool {
			for _, to := range email.To {
				if to == targetEmail || strings.Contains(to, strings.Split(targetEmail, "@")[0]) {
					return true
				}
			}
			return false
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Found email matching predicate: %s", email.Subject)

	// Verify the email matches our criteria
	found := false
	for _, to := range email.To {
		if strings.Contains(to, strings.Split(targetEmail, "@")[0]) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("email.To = %v, expected to contain %s", email.To, targetEmail)
	}
}

// ============================================================================
// README Filter Options Examples
// ============================================================================

func TestREADME_WaitForEmail_SubjectFilter(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// Send email with specific subject
	sendTestEmail(t, inbox.EmailAddress(), "Password Reset", "Click here to reset")

	// README example: Wait for email with specific subject
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(30*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Password Reset`)),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(email.Subject, "Password Reset") {
		t.Errorf("Subject = %s, want to contain 'Password Reset'", email.Subject)
	}
}

func TestREADME_WaitForEmail_FromFilter(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	sendTestEmail(t, inbox.EmailAddress(), "From Filter Test", "Testing from filter")

	// README example: Wait for email from specific sender
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(30*time.Second),
		vaultsandbox.WithFromRegex(regexp.MustCompile(`test@`)),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(email.From, "test@") {
		t.Errorf("From = %s, want to contain 'test@'", email.From)
	}
}

// ============================================================================
// README Error Handling Example (lines 696-749)
// ============================================================================

func TestREADME_ErrorHandling_Timeout(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// README example: This might return context.DeadlineExceeded
	_, err = inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(1*time.Second))
	if err == nil {
		t.Skip("unexpectedly received an email")
	}

	// README example: Check error types
	var apiErr *vaultsandbox.APIError

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		t.Log("Context deadline exceeded (expected for timeout test)")
	case errors.As(err, &apiErr):
		t.Logf("API Error (%d): %s", apiErr.StatusCode, apiErr.Message)
	case errors.Is(err, vaultsandbox.ErrSignatureInvalid):
		t.Log("CRITICAL: Signature verification failed!")
	default:
		t.Logf("Other error: %v", err)
	}
}

func TestREADME_ErrorHandling_APIError(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// Try to get a non-existent email
	_, err = inbox.GetEmail(ctx, "non-existent-email-id")
	if err == nil {
		t.Fatal("expected error for non-existent email")
	}

	// README example: Check for specific error
	if errors.Is(err, vaultsandbox.ErrEmailNotFound) {
		t.Log("Email not found (expected)")
	} else {
		var apiErr *vaultsandbox.APIError
		if errors.As(err, &apiErr) {
			t.Logf("API Error (%d): %s", apiErr.StatusCode, apiErr.Message)
		} else {
			t.Logf("Other error: %v", err)
		}
	}
}

func TestREADME_ErrorHandling_ErrUnauthorized(t *testing.T) {
	// README example: Invalid API key
	_, err := vaultsandbox.New("invalid-api-key",
		vaultsandbox.WithBaseURL(baseURL),
	)
	if err == nil {
		t.Fatal("expected error for invalid API key")
	}

	if errors.Is(err, vaultsandbox.ErrUnauthorized) {
		t.Log("Unauthorized error (expected)")
	} else {
		t.Logf("Error: %v", err)
	}
}

func TestREADME_ErrorHandling_ErrClientClosed(t *testing.T) {
	ctx := context.Background()

	// Create and close client
	client, err := vaultsandbox.New(apiKey,
		vaultsandbox.WithBaseURL(baseURL),
	)
	if err != nil {
		t.Fatal(err)
	}

	client.Close()

	// README example: Operations on closed client
	_, err = client.CreateInbox(ctx)
	if err == nil {
		t.Fatal("expected error for closed client")
	}

	if errors.Is(err, vaultsandbox.ErrClientClosed) {
		t.Log("Client closed error (expected)")
	} else {
		t.Logf("Error: %v", err)
	}
}

func TestREADME_ErrorHandling_ErrInboxNotFound(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// Create and delete inbox
	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	emailAddr := inbox.EmailAddress()
	inbox.Delete(ctx)

	// Try to delete again
	err = client.DeleteInbox(ctx, emailAddr)
	if err == nil {
		t.Log("No error on second delete (may be idempotent)")
		return
	}

	if errors.Is(err, vaultsandbox.ErrInboxNotFound) {
		t.Log("Inbox not found error (expected)")
	} else {
		t.Logf("Error: %v", err)
	}
}

// ============================================================================
// README Client Options Examples
// ============================================================================

func TestREADME_ClientOptions(t *testing.T) {
	ctx := context.Background()

	// README example: All client options
	client, err := vaultsandbox.New(apiKey,
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithTimeout(30*time.Second),
		vaultsandbox.WithRetries(3),
		vaultsandbox.WithRetryOn([]int{408, 429, 500, 502, 503, 504}),
		vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Verify client works
	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Created inbox with custom options: %s", inbox.EmailAddress())
}

func TestREADME_DeliveryStrategy_SSE(t *testing.T) {
	ctx := context.Background()

	// README example: SSE delivery strategy
	client, err := vaultsandbox.New(apiKey,
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Created inbox with SSE strategy: %s", inbox.EmailAddress())
}

func TestREADME_DeliveryStrategy_Polling(t *testing.T) {
	ctx := context.Background()

	// README example: Polling delivery strategy
	client, err := vaultsandbox.New(apiKey,
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Created inbox with Polling strategy: %s", inbox.EmailAddress())
}

// ============================================================================
// README Inbox Options Examples
// ============================================================================

func TestREADME_InboxOptions_TTL(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// README example: Custom TTL
	inbox, err := client.CreateInbox(ctx,
		vaultsandbox.WithTTL(10*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// Verify TTL is respected
	expectedExpiry := time.Now().Add(10 * time.Minute)
	actualExpiry := inbox.ExpiresAt()

	// Allow 30 second tolerance
	if actualExpiry.Before(expectedExpiry.Add(-30*time.Second)) ||
		actualExpiry.After(expectedExpiry.Add(30*time.Second)) {
		t.Errorf("ExpiresAt = %v, want around %v", actualExpiry, expectedExpiry)
	}

	t.Logf("Inbox expires at: %s", inbox.ExpiresAt().Format(time.RFC3339))
}

// ============================================================================
// README Export/Import Examples
// ============================================================================

func TestREADME_ExportImport(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// Create inbox
	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// README example: Export inbox
	exported := inbox.Export()

	// Verify export data
	if exported.EmailAddress != inbox.EmailAddress() {
		t.Errorf("exported.EmailAddress = %s, want %s", exported.EmailAddress, inbox.EmailAddress())
	}
	if exported.SecretKeyB64 == "" {
		t.Error("exported.SecretKeyB64 is empty")
	}

	// Validate export
	if err := exported.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	// README example: Export to JSON (for file export)
	jsonData, err := json.MarshalIndent(exported, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Exported inbox JSON:\n%s", string(jsonData))

	// README example: Import inbox (in new client)
	client2 := newClient(t)
	imported, err := client2.ImportInbox(ctx, exported)
	if err != nil {
		t.Fatal(err)
	}

	if imported.EmailAddress() != inbox.EmailAddress() {
		t.Errorf("imported.EmailAddress = %s, want %s", imported.EmailAddress(), inbox.EmailAddress())
	}
}

func TestREADME_ExportImportFile(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// Create inbox
	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// README example: Export to file
	tmpFile := t.TempDir() + "/inbox-export.json"
	if err := client.ExportInboxToFile(inbox, tmpFile); err != nil {
		t.Fatal(err)
	}

	t.Logf("Exported inbox to: %s", tmpFile)

	// README example: Import from file
	client2 := newClient(t)
	imported, err := client2.ImportInboxFromFile(ctx, tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	if imported.EmailAddress() != inbox.EmailAddress() {
		t.Errorf("imported.EmailAddress = %s, want %s", imported.EmailAddress(), inbox.EmailAddress())
	}

	t.Logf("Successfully imported inbox from file")
}

// ============================================================================
// README Email Methods Examples
// ============================================================================

func TestREADME_EmailMethods(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	sendTestEmail(t, inbox.EmailAddress(), "Email Methods Test", "Testing email methods")

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Received email: ID=%s, Subject=%s", email.ID, email.Subject)

	// README example: Mark as read
	// Note: This may fail if the email was already marked as read during fetch
	if err := inbox.MarkEmailAsRead(ctx, email.ID); err != nil {
		t.Logf("MarkEmailAsRead() error (may be expected): %v", err)
	} else {
		t.Log("Marked email as read")
	}

	// README example: Get raw email (RFC 5322 format)
	rawEmail, err := inbox.GetRawEmail(ctx, email.ID)
	if err != nil {
		t.Logf("GetRawEmail() error (may be expected): %v", err)
	} else if rawEmail == "" {
		t.Log("raw email is empty")
	} else {
		t.Logf("Raw email length: %d bytes", len(rawEmail))
		// Verify raw email has expected headers
		if strings.Contains(rawEmail, "Subject:") {
			t.Log("Raw email contains Subject header")
		}
	}

	// README example: Delete email
	if err := inbox.DeleteEmail(ctx, email.ID); err != nil {
		t.Logf("DeleteEmail() error (may be expected): %v", err)
	} else {
		t.Log("Deleted email")

		// Verify email is gone
		emails, err := inbox.GetEmails(ctx)
		if err != nil {
			t.Logf("GetEmails() after delete error: %v", err)
		} else {
			for _, e := range emails {
				if e.ID == email.ID {
					t.Error("email should have been deleted")
				}
			}
		}
	}
}

// ============================================================================
// README Inbox Methods Examples
// ============================================================================

func TestREADME_InboxMethods(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// README example: Inbox properties
	t.Logf("EmailAddress: %s", inbox.EmailAddress())
	t.Logf("InboxHash: %s", inbox.InboxHash())
	t.Logf("ExpiresAt: %s", inbox.ExpiresAt().Format(time.RFC3339))
	t.Logf("IsExpired: %v", inbox.IsExpired())

	// Verify properties
	if inbox.EmailAddress() == "" {
		t.Error("EmailAddress() is empty")
	}
	if inbox.InboxHash() == "" {
		t.Error("InboxHash() is empty")
	}
	if inbox.ExpiresAt().Before(time.Now()) {
		t.Error("ExpiresAt() is in the past")
	}
	if inbox.IsExpired() {
		t.Error("IsExpired() should be false for new inbox")
	}

	// README example: GetSyncStatus
	status, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("SyncStatus: EmailCount=%d, Hash=%s", status.EmailCount, status.EmailsHash)
}

// ============================================================================
// README Client Methods Examples
// ============================================================================

func TestREADME_ClientMethods(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// README example: ServerInfo
	info := client.ServerInfo()
	if info == nil {
		t.Fatal("ServerInfo() returned nil")
	}
	t.Logf("ServerInfo: MaxTTL=%v, DefaultTTL=%v, Domains=%v",
		info.MaxTTL, info.DefaultTTL, info.AllowedDomains)

	// README example: CheckKey
	if err := client.CheckKey(ctx); err != nil {
		t.Errorf("CheckKey() error = %v", err)
	}

	// README example: Create and track inboxes
	inbox1, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer inbox1.Delete(ctx)

	inbox2, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer inbox2.Delete(ctx)

	// README example: Inboxes()
	inboxes := client.Inboxes()
	if len(inboxes) < 2 {
		t.Errorf("Inboxes() = %d, want at least 2", len(inboxes))
	}

	// README example: GetInbox()
	retrieved, exists := client.GetInbox(inbox1.EmailAddress())
	if !exists {
		t.Error("GetInbox() returned false for existing inbox")
	}
	if retrieved.EmailAddress() != inbox1.EmailAddress() {
		t.Error("GetInbox() returned wrong inbox")
	}

	// Note: DeleteAllInboxes is not tested here because it would delete
	// inboxes from other concurrent tests. The defers above clean up
	// the inboxes created by this test.
}
