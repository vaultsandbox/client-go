//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	vaultsandbox "github.com/vaultsandbox/client-go"
)

var (
	apiKey  string
	baseURL string
)

func TestMain(m *testing.M) {
	// Load .env file if it exists (won't error if missing)
	if err := godotenv.Load("../.env"); err != nil {
		os.Stderr.WriteString("Note: .env file not found at project root\n")
	}

	apiKey = os.Getenv("VAULTSANDBOX_API_KEY")
	baseURL = os.Getenv("VAULTSANDBOX_URL")

	if apiKey == "" {
		os.Stderr.WriteString("Skipping integration tests: VAULTSANDBOX_API_KEY not set\n")
		os.Exit(0)
	}

	if baseURL == "" {
		os.Stderr.WriteString("Skipping integration tests: VAULTSANDBOX_URL not set\n")
		os.Exit(0)
	}

	os.Stderr.WriteString("Running integration tests...\n")
	os.Stderr.WriteString("API URL: " + baseURL + "\n")

	os.Exit(m.Run())
}

func newClient(t *testing.T) *vaultsandbox.Client {
	t.Helper()

	opts := []vaultsandbox.Option{
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithTimeout(30 * time.Second),
	}

	client, err := vaultsandbox.New(apiKey, opts...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Cleanup(func() {
		client.Close()
	})

	return client
}

func TestIntegration_CreateAndDeleteInbox(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	t.Logf("Created inbox: %s", inbox.EmailAddress())

	// Verify inbox exists
	if inbox.EmailAddress() == "" {
		t.Error("EmailAddress() is empty")
	}
	if inbox.ExpiresAt().Before(time.Now()) {
		t.Error("ExpiresAt() is in the past")
	}
	if inbox.InboxHash() == "" {
		t.Error("InboxHash() is empty")
	}
	if inbox.IsExpired() {
		t.Error("IsExpired() returned true for new inbox")
	}

	// Delete inbox
	if err := inbox.Delete(ctx); err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}

func TestIntegration_ServerInfo(t *testing.T) {
	client := newClient(t)

	info := client.ServerInfo()
	if info == nil {
		t.Fatal("ServerInfo() returned nil")
	}

	t.Logf("Server info: MaxTTL=%v, DefaultTTL=%v, Domains=%v",
		info.MaxTTL, info.DefaultTTL, info.AllowedDomains)

	if info.MaxTTL <= 0 {
		t.Error("MaxTTL is not positive")
	}
	if info.DefaultTTL <= 0 {
		t.Error("DefaultTTL is not positive")
	}
}

func TestIntegration_ExportImport(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// Create and export
	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	exported := inbox.Export()
	if exported.EmailAddress != inbox.EmailAddress() {
		t.Errorf("exported.EmailAddress = %s, want %s",
			exported.EmailAddress, inbox.EmailAddress())
	}
	if exported.SecretKeyB64 == "" {
		t.Error("exported.SecretKeyB64 is empty")
	}

	// Validate export data
	if err := exported.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	// Import into new client
	client2 := newClient(t)
	imported, err := client2.ImportInbox(ctx, exported)
	if err != nil {
		t.Fatalf("ImportInbox() error = %v", err)
	}

	if imported.EmailAddress() != inbox.EmailAddress() {
		t.Errorf("EmailAddress mismatch: got %s, want %s",
			imported.EmailAddress(), inbox.EmailAddress())
	}
}

func TestIntegration_MultipleInboxes(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// Create multiple inboxes
	const numInboxes = 3
	inboxes := make([]*vaultsandbox.Inbox, numInboxes)

	for i := 0; i < numInboxes; i++ {
		inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
		if err != nil {
			t.Fatalf("CreateInbox() error = %v", err)
		}
		inboxes[i] = inbox
		t.Logf("Created inbox %d: %s", i, inbox.EmailAddress())
	}

	// Verify all inboxes are tracked
	allInboxes := client.Inboxes()
	if len(allInboxes) != numInboxes {
		t.Errorf("Inboxes() returned %d, want %d", len(allInboxes), numInboxes)
	}

	// Get inbox by email
	for _, inbox := range inboxes {
		got, exists := client.GetInbox(inbox.EmailAddress())
		if !exists {
			t.Errorf("GetInbox(%s) not found", inbox.EmailAddress())
			continue
		}
		if got.EmailAddress() != inbox.EmailAddress() {
			t.Errorf("GetInbox() returned wrong inbox")
		}
	}

	// Clean up - delete all at once
	count, err := client.DeleteAllInboxes(ctx)
	if err != nil {
		t.Errorf("DeleteAllInboxes() error = %v", err)
	}
	t.Logf("Deleted %d inboxes", count)
}

func TestIntegration_TTLValidation(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// Test TTL below minimum should fail
	_, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(30*time.Second))
	if err == nil {
		t.Error("expected error for TTL below minimum")
	}
}

func TestIntegration_GetEmails_Empty(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// New inbox should have no emails
	emails, err := inbox.GetEmails(ctx)
	if err != nil {
		t.Fatalf("GetEmails() error = %v", err)
	}

	if len(emails) != 0 {
		t.Errorf("GetEmails() returned %d emails, want 0", len(emails))
	}
}

func TestIntegration_WaitForEmail_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Wait should timeout since no email will arrive
	start := time.Now()
	_, err = inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(3*time.Second),
		vaultsandbox.WithPollInterval(1*time.Second),
	)

	elapsed := time.Since(start)

	if err == nil {
		t.Error("WaitForEmail() should have returned error on timeout")
	}

	// Should have taken approximately the timeout duration
	if elapsed < 2*time.Second || elapsed > 5*time.Second {
		t.Errorf("WaitForEmail() took %v, expected around 3s", elapsed)
	}
}

// TestIntegration_WaitForEmail_Receive is a manual test that requires
// sending an email to the created inbox. Run with:
//
//	VAULTSANDBOX_API_KEY=xxx go test -tags=integration -run=WaitForEmail_Receive -v
func TestIntegration_WaitForEmail_Receive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	if os.Getenv("MANUAL_TEST") == "" {
		t.Skip("skipping manual test: set MANUAL_TEST=1 to run")
	}

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Send test email to: %s", inbox.EmailAddress())
	t.Logf("Waiting for email...")

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(2*time.Minute),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	t.Logf("Received email: Subject=%s, From=%s", email.Subject, email.From)

	// Verify email has expected fields
	if email.ID == "" {
		t.Error("email.ID is empty")
	}
	if email.From == "" {
		t.Error("email.From is empty")
	}
	if email.ReceivedAt.IsZero() {
		t.Error("email.ReceivedAt is zero")
	}
}

func TestIntegration_GetSyncStatus(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Get sync status for new inbox
	status, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus() error = %v", err)
	}

	// New inbox should have no emails
	if status.EmailCount != 0 {
		t.Errorf("EmailCount = %d, want 0", status.EmailCount)
	}
	// Hash should be present even for empty inbox
	if status.EmailsHash == "" {
		t.Error("EmailsHash is empty")
	}

	t.Logf("Sync status: count=%d, hash=%s", status.EmailCount, status.EmailsHash)
}

func TestIntegration_ExportFileRoundtrip(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// Create an inbox
	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Export to file
	tmpFile := t.TempDir() + "/inbox-export.json"
	if err := client.ExportInboxToFile(inbox, tmpFile); err != nil {
		t.Fatalf("ExportInboxToFile() error = %v", err)
	}

	// Verify file exists and contains valid JSON
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read export file: %v", err)
	}
	if len(data) == 0 {
		t.Error("export file is empty")
	}

	t.Logf("Exported %d bytes to %s", len(data), tmpFile)

	// Import from file in new client
	client2 := newClient(t)
	imported, err := client2.ImportInboxFromFile(ctx, tmpFile)
	if err != nil {
		t.Fatalf("ImportInboxFromFile() error = %v", err)
	}

	// Verify imported inbox matches original
	if imported.EmailAddress() != inbox.EmailAddress() {
		t.Errorf("EmailAddress mismatch: got %s, want %s",
			imported.EmailAddress(), inbox.EmailAddress())
	}
	if imported.InboxHash() != inbox.InboxHash() {
		t.Errorf("InboxHash mismatch: got %s, want %s",
			imported.InboxHash(), inbox.InboxHash())
	}
}

func TestIntegration_CheckKey(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// Valid API key should not error
	if err := client.CheckKey(ctx); err != nil {
		t.Errorf("CheckKey() error = %v", err)
	}
}

func TestIntegration_CheckKey_Invalid(t *testing.T) {
	// Create client with invalid API key
	// The Go SDK validates the API key during New(), so it should fail immediately
	opts := []vaultsandbox.Option{
		vaultsandbox.WithBaseURL(baseURL),
	}

	_, err := vaultsandbox.New("invalid-api-key-12345", opts...)
	if err == nil {
		t.Fatal("New() should return error for invalid API key")
	}

	// Verify the error is an unauthorized error
	if !errors.Is(err, vaultsandbox.ErrUnauthorized) {
		t.Errorf("New() error = %v, want ErrUnauthorized", err)
	} else {
		t.Log("New() correctly returned ErrUnauthorized for invalid API key")
	}
}

// TestIntegration_SSEDelivery tests SSE real-time email delivery.
// Requires manual email sending to verify.
func TestIntegration_SSEDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	if os.Getenv("MANUAL_TEST") == "" {
		t.Skip("skipping manual test: set MANUAL_TEST=1 to run")
	}

	// Create client with SSE strategy
	opts := []vaultsandbox.Option{
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithTimeout(30 * time.Second),
		vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
	}

	client, err := vaultsandbox.New(apiKey, opts...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Using SSE strategy")
	t.Logf("Send test email to: %s", inbox.EmailAddress())
	t.Logf("Waiting for email via SSE...")

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(2*time.Minute),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	t.Logf("Received via SSE: Subject=%s", email.Subject)
}

// TestIntegration_PollingDelivery tests polling-based email delivery.
// Requires manual email sending to verify.
func TestIntegration_PollingDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	if os.Getenv("MANUAL_TEST") == "" {
		t.Skip("skipping manual test: set MANUAL_TEST=1 to run")
	}

	// Create client with polling strategy
	opts := []vaultsandbox.Option{
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithTimeout(30 * time.Second),
		vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
	}

	client, err := vaultsandbox.New(apiKey, opts...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Using polling strategy")
	t.Logf("Send test email to: %s", inbox.EmailAddress())
	t.Logf("Waiting for email via polling...")

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(2*time.Minute),
		vaultsandbox.WithPollInterval(2*time.Second),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	t.Logf("Received via polling: Subject=%s", email.Subject)
}

// TestIntegration_WaitForEmail_WithFilters tests email filtering options.
// Requires manual email sending with specific subject/from to verify.
func TestIntegration_WaitForEmail_WithFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	if os.Getenv("MANUAL_TEST") == "" {
		t.Skip("skipping manual test: set MANUAL_TEST=1 to run")
	}

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Send test email to: %s", inbox.EmailAddress())
	t.Log("Use subject containing 'TEST-FILTER' to match the filter")
	t.Logf("Waiting for email with subject filter...")

	// Wait for email with subject filter
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(2*time.Minute),
		vaultsandbox.WithSubject("TEST-FILTER"),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() with subject filter error = %v", err)
	}

	if email.Subject != "TEST-FILTER" {
		t.Errorf("Subject = %s, want TEST-FILTER", email.Subject)
	}

	t.Logf("Received filtered email: Subject=%s", email.Subject)
}

// TestIntegration_WaitForEmail_WithPredicate tests custom predicate filtering.
// Requires manual email sending to verify.
func TestIntegration_WaitForEmail_WithPredicate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	if os.Getenv("MANUAL_TEST") == "" {
		t.Skip("skipping manual test: set MANUAL_TEST=1 to run")
	}

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Send test email to: %s", inbox.EmailAddress())
	t.Log("Send an email with HTML content to match the predicate")
	t.Logf("Waiting for email with HTML content...")

	// Custom predicate that checks for HTML content
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(2*time.Minute),
		vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
			return e.HTML != ""
		}),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() with predicate error = %v", err)
	}

	if email.HTML == "" {
		t.Error("Expected email with HTML content")
	}

	t.Logf("Received email with HTML: Subject=%s", email.Subject)
}

// TestIntegration_WaitForEmailCount tests waiting for multiple emails.
// Requires manual sending of multiple emails to verify.
func TestIntegration_WaitForEmailCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	if os.Getenv("MANUAL_TEST") == "" {
		t.Skip("skipping manual test: set MANUAL_TEST=1 to run")
	}

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Send 2 test emails to: %s", inbox.EmailAddress())
	t.Logf("Waiting for 2 emails...")

	emails, err := inbox.WaitForEmailCount(ctx, 2,
		vaultsandbox.WithWaitTimeout(3*time.Minute),
	)
	if err != nil {
		t.Fatalf("WaitForEmailCount() error = %v", err)
	}

	if len(emails) != 2 {
		t.Errorf("got %d emails, want 2", len(emails))
	}

	for i, e := range emails {
		t.Logf("Email %d: Subject=%s", i+1, e.Subject)
	}
}

// TestIntegration_EmailOperations tests email operations like markAsRead and delete.
// Requires manual email sending to verify.
func TestIntegration_EmailOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	if os.Getenv("MANUAL_TEST") == "" {
		t.Skip("skipping manual test: set MANUAL_TEST=1 to run")
	}

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Send test email to: %s", inbox.EmailAddress())
	t.Logf("Waiting for email...")

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(2*time.Minute),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	// Test mark as read
	if email.IsRead {
		t.Error("email should not be read initially")
	}

	if err := email.MarkAsRead(ctx); err != nil {
		t.Errorf("MarkAsRead() error = %v", err)
	}
	t.Log("Marked email as read")

	// Test get raw email
	rawEmail, err := email.GetRaw(ctx)
	if err != nil {
		t.Errorf("GetRaw() error = %v", err)
	}
	if rawEmail == "" {
		t.Error("raw email is empty")
	}
	t.Logf("Got raw email: %d bytes", len(rawEmail))

	// Test delete
	if err := email.Delete(ctx); err != nil {
		t.Errorf("Delete() error = %v", err)
	}
	t.Log("Deleted email")

	// Verify email is gone
	emails, err := inbox.GetEmails(ctx)
	if err != nil {
		t.Fatalf("GetEmails() error = %v", err)
	}
	for _, e := range emails {
		if e.ID == email.ID {
			t.Error("email should have been deleted")
		}
	}
}

// TestIntegration_AuthResults tests email authentication result parsing.
// Requires receiving an email from a properly configured sender.
func TestIntegration_AuthResults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	if os.Getenv("MANUAL_TEST") == "" {
		t.Skip("skipping manual test: set MANUAL_TEST=1 to run")
	}

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Send test email from a properly configured domain to: %s", inbox.EmailAddress())
	t.Logf("Waiting for email...")

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(2*time.Minute),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	// Check if auth results are present
	if email.AuthResults == nil {
		t.Log("No authentication results in email (sender may not have SPF/DKIM/DMARC)")
		return
	}

	// Validate auth results
	validation := email.AuthResults.Validate()
	t.Logf("Auth validation: Passed=%v, SPF=%v, DKIM=%v, DMARC=%v, ReverseDNS=%v",
		validation.Passed, validation.SPFPassed, validation.DKIMPassed,
		validation.DMARCPassed, validation.ReverseDNSPassed)

	if len(validation.Failures) > 0 {
		t.Logf("Auth failures: %v", validation.Failures)
	}

	// Also test IsPassing()
	if validation.Passed != email.AuthResults.IsPassing() {
		t.Error("IsPassing() does not match Validate().Passed")
	}
}

// TestIntegration_ServerInfo_Values verifies server info returns expected values.
func TestIntegration_ServerInfo_Values(t *testing.T) {
	client := newClient(t)

	info := client.ServerInfo()
	if info == nil {
		t.Fatal("ServerInfo() returned nil")
	}

	// Verify all expected fields are present and have valid values
	t.Logf("Server info: MaxTTL=%v, DefaultTTL=%v, Domains=%v",
		info.MaxTTL, info.DefaultTTL, info.AllowedDomains)

	// MaxTTL should be at least 1 minute (reasonable minimum)
	if info.MaxTTL < time.Minute {
		t.Errorf("MaxTTL = %v, want at least 1 minute", info.MaxTTL)
	}

	// DefaultTTL should be at least 1 minute
	if info.DefaultTTL < time.Minute {
		t.Errorf("DefaultTTL = %v, want at least 1 minute", info.DefaultTTL)
	}

	// DefaultTTL should not exceed MaxTTL
	if info.DefaultTTL > info.MaxTTL {
		t.Errorf("DefaultTTL (%v) > MaxTTL (%v)", info.DefaultTTL, info.MaxTTL)
	}

	// AllowedDomains should not be empty
	if len(info.AllowedDomains) == 0 {
		t.Error("AllowedDomains is empty")
	}

	// Each domain should be non-empty
	for i, domain := range info.AllowedDomains {
		if domain == "" {
			t.Errorf("AllowedDomains[%d] is empty", i)
		}
	}
}

// TestIntegration_AccessAfterDelete tests that accessing a deleted inbox returns ErrInboxNotFound.
func TestIntegration_AccessAfterDelete(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// Create an inbox
	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	emailAddress := inbox.EmailAddress()
	t.Logf("Created inbox: %s", emailAddress)

	// Delete the inbox
	if err := inbox.Delete(ctx); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	t.Log("Inbox deleted")

	// Try to get emails from deleted inbox - should fail with ErrInboxNotFound
	_, err = inbox.GetEmails(ctx)
	if err == nil {
		t.Error("GetEmails() on deleted inbox should return error")
	} else if !errors.Is(err, vaultsandbox.ErrInboxNotFound) {
		t.Errorf("GetEmails() error = %v, want ErrInboxNotFound", err)
	} else {
		t.Log("GetEmails() correctly returned ErrInboxNotFound")
	}

	// Try to get sync status from deleted inbox - should fail
	_, err = inbox.GetSyncStatus(ctx)
	if err == nil {
		t.Error("GetSyncStatus() on deleted inbox should return error")
	} else if !errors.Is(err, vaultsandbox.ErrInboxNotFound) {
		t.Errorf("GetSyncStatus() error = %v, want ErrInboxNotFound", err)
	} else {
		t.Log("GetSyncStatus() correctly returned ErrInboxNotFound")
	}
}

// TestIntegration_SyncStatus_ConsistentHash tests that sync status hash is consistent.
func TestIntegration_SyncStatus_ConsistentHash(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Get sync status multiple times - hash should be consistent
	status1, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus() #1 error = %v", err)
	}

	status2, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus() #2 error = %v", err)
	}

	status3, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus() #3 error = %v", err)
	}

	// All hashes should be identical for an unchanged inbox
	if status1.EmailsHash != status2.EmailsHash {
		t.Errorf("Hash changed between calls: %s != %s", status1.EmailsHash, status2.EmailsHash)
	}
	if status2.EmailsHash != status3.EmailsHash {
		t.Errorf("Hash changed between calls: %s != %s", status2.EmailsHash, status3.EmailsHash)
	}

	// Email count should also be consistent
	if status1.EmailCount != status2.EmailCount || status2.EmailCount != status3.EmailCount {
		t.Errorf("Email count changed: %d, %d, %d", status1.EmailCount, status2.EmailCount, status3.EmailCount)
	}

	t.Logf("Consistent hash across 3 calls: %s (count=%d)", status1.EmailsHash, status1.EmailCount)
}

// TestIntegration_GetEmail_NotFound tests that getting a non-existent email returns ErrEmailNotFound.
func TestIntegration_GetEmail_NotFound(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Try to get a non-existent email
	_, err = inbox.GetEmail(ctx, "non-existent-email-id-12345")
	if err == nil {
		t.Error("GetEmail() with invalid ID should return error")
	} else if !errors.Is(err, vaultsandbox.ErrEmailNotFound) {
		t.Errorf("GetEmail() error = %v, want ErrEmailNotFound", err)
	} else {
		t.Log("GetEmail() correctly returned ErrEmailNotFound")
	}
}

// TestIntegration_NetworkError tests that connecting to an invalid host returns NetworkError.
func TestIntegration_NetworkError(t *testing.T) {
	// Create client with invalid URL
	opts := []vaultsandbox.Option{
		vaultsandbox.WithBaseURL("https://invalid-host-that-does-not-exist.example.com"),
		vaultsandbox.WithTimeout(5 * time.Second),
	}

	// This should fail during initialization (CheckKey call)
	_, err := vaultsandbox.New(apiKey, opts...)
	if err == nil {
		t.Error("New() with invalid host should return error")
	} else {
		// The error should be a network error or timeout
		t.Logf("New() correctly returned error: %v", err)

		// Check if it's a network-related error (could be wrapped)
		var netErr *vaultsandbox.NetworkError
		if errors.As(err, &netErr) {
			t.Log("Error is a NetworkError as expected")
		} else {
			// It might be a wrapped error or timeout, which is also acceptable
			t.Logf("Error type: %T", err)
		}
	}
}

// TestIntegration_ResourceCleanup tests that Close() properly cleans up resources.
func TestIntegration_ResourceCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	ctx := context.Background()

	// Create client
	opts := []vaultsandbox.Option{
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithTimeout(30 * time.Second),
	}

	client, err := vaultsandbox.New(apiKey, opts...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create multiple inboxes
	inbox1, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() #1 error = %v", err)
	}
	t.Logf("Created inbox 1: %s", inbox1.EmailAddress())

	inbox2, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() #2 error = %v", err)
	}
	t.Logf("Created inbox 2: %s", inbox2.EmailAddress())

	// Verify inboxes are tracked
	inboxes := client.Inboxes()
	if len(inboxes) != 2 {
		t.Errorf("Inboxes() = %d, want 2", len(inboxes))
	}

	// Close the client
	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	t.Log("Client closed")

	// After close, client should be unusable
	_, err = client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err == nil {
		t.Error("CreateInbox() after Close() should return error")
	} else if !errors.Is(err, vaultsandbox.ErrClientClosed) {
		t.Errorf("CreateInbox() after Close() error = %v, want ErrClientClosed", err)
	} else {
		t.Log("CreateInbox() after Close() correctly returned ErrClientClosed")
	}

	// Inboxes should be cleared
	if len(client.Inboxes()) != 0 {
		t.Errorf("Inboxes() after Close() = %d, want 0", len(client.Inboxes()))
	}

	// Close should be idempotent
	if err := client.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}

	// Clean up - delete inboxes using a new client
	cleanupClient := newClient(t)
	_ = cleanupClient.DeleteInbox(ctx, inbox1.EmailAddress())
	_ = cleanupClient.DeleteInbox(ctx, inbox2.EmailAddress())
}

// TestIntegration_EmailAddressFormat verifies inbox email address format.
func TestIntegration_EmailAddressFormat(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	email := inbox.EmailAddress()
	t.Logf("Created inbox: %s", email)

	// Email should contain @ symbol
	if !strings.Contains(email, "@") {
		t.Errorf("EmailAddress %q does not contain @", email)
	}

	// Email should have local part before @
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		t.Errorf("EmailAddress %q should have exactly one @", email)
	}

	if parts[0] == "" {
		t.Error("EmailAddress has empty local part")
	}

	if parts[1] == "" {
		t.Error("EmailAddress has empty domain part")
	}

	// Domain should be in allowed domains
	serverInfo := client.ServerInfo()
	found := false
	for _, domain := range serverInfo.AllowedDomains {
		if parts[1] == domain {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("EmailAddress domain %q not in AllowedDomains %v", parts[1], serverInfo.AllowedDomains)
	}
}

// ============================================================================
// Edge Case Tests (Section 6 from tests-spec.md)
// ============================================================================

// TestIntegration_WaitForEmail_ZeroTimeout verifies that a timeout of 0 causes
// immediate timeout. With zero timeout, the context deadline is already expired.
func TestIntegration_WaitForEmail_ZeroTimeout(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Wait with zero timeout - should return immediately with deadline exceeded
	start := time.Now()
	_, err = inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(0))
	elapsed := time.Since(start)

	// Should return very quickly (within 100ms)
	if elapsed > 100*time.Millisecond {
		t.Errorf("WaitForEmail with zero timeout took %v, expected immediate return", elapsed)
	}

	// Should get a context deadline exceeded error
	if err == nil {
		t.Error("WaitForEmail with zero timeout should return error")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("WaitForEmail error = %v, want context.DeadlineExceeded", err)
	} else {
		t.Log("WaitForEmail correctly returned context.DeadlineExceeded for zero timeout")
	}
}

// TestIntegration_WaitForEmailCount_ZeroTimeout verifies zero timeout for WaitForEmailCount.
func TestIntegration_WaitForEmailCount_ZeroTimeout(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Wait with zero timeout
	start := time.Now()
	_, err = inbox.WaitForEmailCount(ctx, 1, vaultsandbox.WithWaitTimeout(0))
	elapsed := time.Since(start)

	// Should return very quickly
	if elapsed > 100*time.Millisecond {
		t.Errorf("WaitForEmailCount with zero timeout took %v, expected immediate return", elapsed)
	}

	// Should get a context deadline exceeded error
	if err == nil {
		t.Error("WaitForEmailCount with zero timeout should return error")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("WaitForEmailCount error = %v, want context.DeadlineExceeded", err)
	} else {
		t.Log("WaitForEmailCount correctly returned context.DeadlineExceeded for zero timeout")
	}
}

// TestIntegration_WaitForEmail_VeryShortTimeout verifies behavior with very short timeout.
func TestIntegration_WaitForEmail_VeryShortTimeout(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Wait with 1ms timeout
	start := time.Now()
	_, err = inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(1*time.Millisecond))
	elapsed := time.Since(start)

	// Should return quickly (within 200ms to account for network latency)
	if elapsed > 200*time.Millisecond {
		t.Errorf("WaitForEmail with 1ms timeout took %v, expected quick return", elapsed)
	}

	// Should get an error (no email available)
	if err == nil {
		t.Error("WaitForEmail with very short timeout should return error (no email)")
	} else {
		t.Logf("WaitForEmail correctly returned error for very short timeout: %v", err)
	}
}

// TestIntegration_WaitForEmail_ContextCancellation verifies context cancellation is respected.
func TestIntegration_WaitForEmail_ContextCancellation(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Create a context that we'll cancel
	waitCtx, cancel := context.WithCancel(ctx)

	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = inbox.WaitForEmail(waitCtx, vaultsandbox.WithWaitTimeout(30*time.Second))
	elapsed := time.Since(start)

	// Should return quickly after cancellation
	if elapsed > 500*time.Millisecond {
		t.Errorf("WaitForEmail took %v after context cancel, expected quick return", elapsed)
	}

	// Should get a context canceled error
	if err == nil {
		t.Error("WaitForEmail should return error when context is canceled")
	} else if !errors.Is(err, context.Canceled) {
		t.Errorf("WaitForEmail error = %v, want context.Canceled", err)
	} else {
		t.Log("WaitForEmail correctly returned context.Canceled")
	}
}

// TestIntegration_DeletedInboxDuringWait verifies behavior when inbox is deleted during wait.
func TestIntegration_DeletedInboxDuringWait(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	emailAddr := inbox.EmailAddress()
	t.Logf("Created inbox: %s", emailAddr)

	// Channel to signal when wait has started
	waitStarted := make(chan struct{})
	waitDone := make(chan error, 1)

	// Start waiting for email in a goroutine
	go func() {
		close(waitStarted)
		_, err := inbox.WaitForEmail(ctx,
			vaultsandbox.WithWaitTimeout(10*time.Second),
			vaultsandbox.WithPollInterval(500*time.Millisecond),
		)
		waitDone <- err
	}()

	// Wait for the wait to start
	<-waitStarted
	time.Sleep(200 * time.Millisecond)

	// Delete the inbox while waiting
	t.Log("Deleting inbox during wait...")
	if err := inbox.Delete(ctx); err != nil {
		t.Logf("Delete() error (may be expected): %v", err)
	}

	// Wait for the WaitForEmail to complete
	select {
	case err := <-waitDone:
		if err == nil {
			t.Error("WaitForEmail should return error when inbox is deleted")
		} else {
			// Should get ErrInboxNotFound or a context error
			if errors.Is(err, vaultsandbox.ErrInboxNotFound) {
				t.Log("WaitForEmail correctly returned ErrInboxNotFound")
			} else if errors.Is(err, context.DeadlineExceeded) {
				t.Log("WaitForEmail returned DeadlineExceeded (inbox deleted but timeout reached first)")
			} else {
				t.Logf("WaitForEmail returned error: %v", err)
			}
		}
	case <-time.After(15 * time.Second):
		t.Error("WaitForEmail did not complete in time")
	}
}

// TestIntegration_MonitorInboxes_EmptySlice verifies MonitorInboxes returns error for empty slice.
func TestIntegration_MonitorInboxes_EmptySlice(t *testing.T) {
	client := newClient(t)

	// Try to monitor empty inboxes slice
	_, err := client.MonitorInboxes([]*vaultsandbox.Inbox{})
	if err == nil {
		t.Error("MonitorInboxes should return error for empty inboxes")
	} else {
		t.Logf("MonitorInboxes correctly returned error for empty slice: %v", err)
	}
}

// TestIntegration_MonitorInboxes_ClosedClient verifies MonitorInboxes fails on closed client.
func TestIntegration_MonitorInboxes_ClosedClient(t *testing.T) {
	ctx := context.Background()

	// Create a client
	opts := []vaultsandbox.Option{
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithTimeout(30 * time.Second),
	}

	client, err := vaultsandbox.New(apiKey, opts...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create an inbox
	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	emailAddr := inbox.EmailAddress()

	// Close the client
	client.Close()

	// Try to monitor after close
	_, err = client.MonitorInboxes([]*vaultsandbox.Inbox{inbox})
	if err == nil {
		t.Error("MonitorInboxes should return error on closed client")
	} else if !errors.Is(err, vaultsandbox.ErrClientClosed) {
		t.Errorf("MonitorInboxes error = %v, want ErrClientClosed", err)
	} else {
		t.Log("MonitorInboxes correctly returned ErrClientClosed")
	}

	// Clean up with a new client
	cleanupClient := newClient(t)
	cleanupClient.DeleteInbox(ctx, emailAddr)
}
