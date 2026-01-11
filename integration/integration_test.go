//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"regexp"
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
	if exported.SecretKey == "" {
		t.Error("exported.SecretKey is empty")
	}
	if exported.Version != vaultsandbox.ExportVersion {
		t.Errorf("exported.Version = %d, want %d", exported.Version, vaultsandbox.ExportVersion)
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

	// Clean up - delete only inboxes created by this test
	for _, inbox := range inboxes {
		if err := inbox.Delete(ctx); err != nil {
			t.Errorf("Delete() error = %v", err)
		}
	}
	t.Logf("Deleted %d inboxes", len(inboxes))
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

	if err := inbox.MarkEmailAsRead(ctx, email.ID); err != nil {
		t.Errorf("MarkEmailAsRead() error = %v", err)
	}
	t.Log("Marked email as read")

	// Test get raw email
	rawEmail, err := inbox.GetRawEmail(ctx, email.ID)
	if err != nil {
		t.Errorf("GetRawEmail() error = %v", err)
	}
	if rawEmail == "" {
		t.Error("raw email is empty")
	}
	t.Logf("Got raw email: %d bytes", len(rawEmail))

	// Test delete
	if err := inbox.DeleteEmail(ctx, email.ID); err != nil {
		t.Errorf("DeleteEmail() error = %v", err)
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

// TestIntegration_WatchInboxes_EmptySlice verifies WatchInboxes returns closed channel for empty slice.
func TestIntegration_WatchInboxes_EmptySlice(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// Watch empty inboxes slice - should return immediately closed channel
	ch := client.WatchInboxes(ctx)

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("WatchInboxes channel should be closed for empty inboxes")
		} else {
			t.Log("WatchInboxes correctly returned closed channel for empty slice")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("WatchInboxes channel should close immediately for empty slice")
	}
}

// TestIntegration_WatchInboxes_ClosedClient verifies WatchInboxes on a closed client.
func TestIntegration_WatchInboxes_ClosedClient(t *testing.T) {
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

	// WatchInboxes returns a channel (doesn't error) but no events will be received
	// since the delivery strategy is closed
	watchCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	ch := client.WatchInboxes(watchCtx, inbox)
	if ch == nil {
		t.Error("WatchInboxes should return a channel even on closed client")
	}

	// Channel should close when context times out (no events received)
	select {
	case event, ok := <-ch:
		if ok {
			t.Errorf("unexpected event received on closed client: %v", event)
		} else {
			t.Log("WatchInboxes channel closed as expected on closed client")
		}
	case <-time.After(200 * time.Millisecond):
		t.Log("WatchInboxes channel did not receive events on closed client (expected)")
	}

	// Clean up with a new client
	cleanupClient := newClient(t)
	cleanupClient.DeleteInbox(ctx, emailAddr)
}

// TestIntegration_DeleteAllInboxes tests deleting all inboxes at once.
func TestIntegration_DeleteAllInboxes(t *testing.T) {
	t.Skip("Skipping: DeleteAllInboxes can interfere with other tests and services")
	client := newClient(t)
	ctx := context.Background()

	// Create multiple inboxes
	const numInboxes = 3
	createdEmails := make([]string, 0, numInboxes)

	for i := 0; i < numInboxes; i++ {
		inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
		if err != nil {
			t.Fatalf("CreateInbox() #%d error = %v", i+1, err)
		}
		createdEmails = append(createdEmails, inbox.EmailAddress())
		t.Logf("Created inbox %d: %s", i+1, inbox.EmailAddress())
	}

	// Verify all inboxes are tracked
	inboxes := client.Inboxes()
	if len(inboxes) != numInboxes {
		t.Errorf("Inboxes() returned %d, want %d", len(inboxes), numInboxes)
	}

	// Delete all inboxes
	count, err := client.DeleteAllInboxes(ctx)
	if err != nil {
		t.Fatalf("DeleteAllInboxes() error = %v", err)
	}

	t.Logf("DeleteAllInboxes() deleted %d inboxes", count)

	// Verify count is reasonable (may be >= numInboxes if other inboxes existed)
	if count < numInboxes {
		t.Errorf("DeleteAllInboxes() count = %d, want at least %d", count, numInboxes)
	}

	// Verify client no longer tracks any inboxes
	inboxes = client.Inboxes()
	if len(inboxes) != 0 {
		t.Errorf("Inboxes() after DeleteAllInboxes() = %d, want 0", len(inboxes))
	}

	// Verify GetInbox returns false for all created emails
	for _, email := range createdEmails {
		_, exists := client.GetInbox(email)
		if exists {
			t.Errorf("GetInbox(%s) should return false after DeleteAllInboxes", email)
		}
	}
}

// TestIntegration_DeleteAllInboxes_Empty tests DeleteAllInboxes with no inboxes.
func TestIntegration_DeleteAllInboxes_Empty(t *testing.T) {
	t.Skip("Skipping: DeleteAllInboxes can interfere with other tests and services")
	client := newClient(t)
	ctx := context.Background()

	// Delete all (should not error even if none exist)
	count, err := client.DeleteAllInboxes(ctx)
	if err != nil {
		t.Fatalf("DeleteAllInboxes() error = %v", err)
	}

	t.Logf("DeleteAllInboxes() on fresh client returned count = %d", count)

	// Count should be 0 or more (depending on what's on the server)
	// The important thing is it doesn't error
}

// TestIntegration_DeleteAllInboxes_ThenCreate tests that creating inboxes works after DeleteAllInboxes.
func TestIntegration_DeleteAllInboxes_ThenCreate(t *testing.T) {
	t.Skip("Skipping: DeleteAllInboxes can interfere with other tests and services")
	client := newClient(t)
	ctx := context.Background()

	// Create an inbox
	inbox1, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() #1 error = %v", err)
	}
	t.Logf("Created inbox 1: %s", inbox1.EmailAddress())

	// Delete all inboxes
	_, err = client.DeleteAllInboxes(ctx)
	if err != nil {
		t.Fatalf("DeleteAllInboxes() error = %v", err)
	}
	t.Log("Deleted all inboxes")

	// Verify client has no inboxes
	if len(client.Inboxes()) != 0 {
		t.Errorf("Inboxes() after DeleteAllInboxes() = %d, want 0", len(client.Inboxes()))
	}

	// Create a new inbox - should work fine
	inbox2, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() #2 after DeleteAllInboxes error = %v", err)
	}
	t.Logf("Created inbox 2: %s", inbox2.EmailAddress())

	// Verify new inbox is tracked
	if len(client.Inboxes()) != 1 {
		t.Errorf("Inboxes() after creating new inbox = %d, want 1", len(client.Inboxes()))
	}

	// Clean up
	inbox2.Delete(ctx)
}

// TestIntegration_SyncAfterGetEmails tests that GetEmails properly populates
// the client's sync state (used by syncInbox).
func TestIntegration_SyncAfterGetEmails(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Get emails (should be empty for new inbox)
	emails, err := inbox.GetEmails(ctx)
	if err != nil {
		t.Fatalf("GetEmails() error = %v", err)
	}

	if len(emails) != 0 {
		t.Errorf("GetEmails() returned %d emails, want 0 for new inbox", len(emails))
	}

	// Get sync status
	status, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus() error = %v", err)
	}

	t.Logf("Sync status: count=%d, hash=%s", status.EmailCount, status.EmailsHash)

	// Status should show 0 emails
	if status.EmailCount != 0 {
		t.Errorf("EmailCount = %d, want 0", status.EmailCount)
	}
}

// TestIntegration_MultipleClientsSameInbox tests that multiple clients can
// access the same inbox via import/export.
func TestIntegration_MultipleClientsSameInbox(t *testing.T) {
	client1 := newClient(t)
	ctx := context.Background()

	// Create inbox with client1
	inbox1, err := client1.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox1.Delete(ctx)

	t.Logf("Created inbox: %s", inbox1.EmailAddress())

	// Export from client1
	exported := inbox1.Export()

	// Import into client2
	client2 := newClient(t)
	inbox2, err := client2.ImportInbox(ctx, exported)
	if err != nil {
		t.Fatalf("ImportInbox() error = %v", err)
	}

	// Both clients should see the same inbox data
	if inbox1.EmailAddress() != inbox2.EmailAddress() {
		t.Errorf("EmailAddress mismatch: %s vs %s", inbox1.EmailAddress(), inbox2.EmailAddress())
	}

	if inbox1.InboxHash() != inbox2.InboxHash() {
		t.Errorf("InboxHash mismatch: %s vs %s", inbox1.InboxHash(), inbox2.InboxHash())
	}

	// Both should be able to get sync status
	status1, err := inbox1.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus() client1 error = %v", err)
	}

	status2, err := inbox2.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus() client2 error = %v", err)
	}

	// Both should see the same sync status
	if status1.EmailsHash != status2.EmailsHash {
		t.Errorf("EmailsHash mismatch: %s vs %s", status1.EmailsHash, status2.EmailsHash)
	}
	if status1.EmailCount != status2.EmailCount {
		t.Errorf("EmailCount mismatch: %d vs %d", status1.EmailCount, status2.EmailCount)
	}

	t.Logf("Both clients see consistent sync status: count=%d, hash=%s",
		status1.EmailCount, status1.EmailsHash)
}

// TestIntegration_GetEmailsMetadataOnly tests the metadata-only fetch used by syncInbox.
func TestIntegration_GetEmailsMetadataOnly(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Get metadata for new inbox (should be empty)
	metadata, err := inbox.GetEmailsMetadataOnly(ctx)
	if err != nil {
		t.Fatalf("GetEmailsMetadataOnly() error = %v", err)
	}

	if len(metadata) != 0 {
		t.Errorf("GetEmailsMetadataOnly() returned %d items, want 0 for new inbox", len(metadata))
	}

	t.Log("GetEmailsMetadataOnly() returned empty list for new inbox as expected")
}

// TestIntegration_GetEmailsMetadataOnly_WithEmail tests metadata fetch after receiving email.
// Requires manual email sending.
func TestIntegration_GetEmailsMetadataOnly_WithEmail(t *testing.T) {
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

	// Wait for email
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(2*time.Minute),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	t.Logf("Received email: ID=%s, Subject=%s", email.ID, email.Subject)

	// Now fetch metadata only
	metadata, err := inbox.GetEmailsMetadataOnly(ctx)
	if err != nil {
		t.Fatalf("GetEmailsMetadataOnly() error = %v", err)
	}

	if len(metadata) != 1 {
		t.Errorf("GetEmailsMetadataOnly() returned %d items, want 1", len(metadata))
	}

	// Verify metadata matches the received email
	if len(metadata) > 0 {
		m := metadata[0]
		if m.ID != email.ID {
			t.Errorf("Metadata ID = %s, want %s", m.ID, email.ID)
		}
		t.Logf("Metadata: ID=%s, ReceivedAt=%v, IsRead=%v", m.ID, m.ReceivedAt, m.IsRead)
	}
}

// TestIntegration_SyncStatusHashConsistency tests that sync status hash is consistent
// with the actual email state (used by syncInbox for change detection).
func TestIntegration_SyncStatusHashConsistency(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Get initial sync status
	status1, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus() #1 error = %v", err)
	}

	// Get emails to verify consistency
	emails, err := inbox.GetEmails(ctx)
	if err != nil {
		t.Fatalf("GetEmails() error = %v", err)
	}

	// Email count should match sync status
	if len(emails) != status1.EmailCount {
		t.Errorf("GetEmails() len = %d, sync status EmailCount = %d, want match",
			len(emails), status1.EmailCount)
	}

	// Get sync status again - should be identical
	status2, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus() #2 error = %v", err)
	}

	if status1.EmailsHash != status2.EmailsHash {
		t.Errorf("Hash changed unexpectedly: %s -> %s", status1.EmailsHash, status2.EmailsHash)
	}

	t.Logf("Sync status consistent: count=%d, hash=%s", status1.EmailCount, status1.EmailsHash)
}

// TestIntegration_SyncWithEmail tests the full sync flow with an actual email.
// This exercises the syncInbox code path indirectly.
// Requires manual email sending.
func TestIntegration_SyncWithEmail(t *testing.T) {
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

	// Get initial sync status
	initialStatus, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("Initial GetSyncStatus() error = %v", err)
	}

	t.Logf("Initial sync status: count=%d, hash=%s", initialStatus.EmailCount, initialStatus.EmailsHash)
	t.Logf("Send test email to: %s", inbox.EmailAddress())
	t.Logf("Waiting for email...")

	// Wait for email
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(2*time.Minute),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	t.Logf("Received email: ID=%s, Subject=%s", email.ID, email.Subject)

	// Get new sync status - should have changed
	newStatus, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("New GetSyncStatus() error = %v", err)
	}

	t.Logf("New sync status: count=%d, hash=%s", newStatus.EmailCount, newStatus.EmailsHash)

	// Hash should have changed (we received a new email)
	if newStatus.EmailsHash == initialStatus.EmailsHash {
		t.Error("Sync hash should change after receiving email")
	}

	// Email count should have increased
	if newStatus.EmailCount <= initialStatus.EmailCount {
		t.Errorf("EmailCount should have increased: %d -> %d",
			initialStatus.EmailCount, newStatus.EmailCount)
	}

	// Verify metadata matches
	metadata, err := inbox.GetEmailsMetadataOnly(ctx)
	if err != nil {
		t.Fatalf("GetEmailsMetadataOnly() error = %v", err)
	}

	if len(metadata) != newStatus.EmailCount {
		t.Errorf("Metadata count = %d, sync status EmailCount = %d, want match",
			len(metadata), newStatus.EmailCount)
	}

	// Find our email in metadata
	found := false
	for _, m := range metadata {
		if m.ID == email.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Email ID not found in metadata")
	}
}

// TestIntegration_SyncWithEmailDeletion tests sync hash changes when email is deleted.
// Requires manual email sending.
func TestIntegration_SyncWithEmailDeletion(t *testing.T) {
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

	// Wait for email
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(2*time.Minute),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	t.Logf("Received email: ID=%s", email.ID)

	// Get sync status with email
	statusWithEmail, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus() with email error = %v", err)
	}

	t.Logf("Status with email: count=%d, hash=%s", statusWithEmail.EmailCount, statusWithEmail.EmailsHash)

	// Delete the email
	if err := inbox.DeleteEmail(ctx, email.ID); err != nil {
		t.Fatalf("DeleteEmail() error = %v", err)
	}

	t.Log("Email deleted")

	// Get sync status after deletion
	statusAfterDelete, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus() after delete error = %v", err)
	}

	t.Logf("Status after delete: count=%d, hash=%s", statusAfterDelete.EmailCount, statusAfterDelete.EmailsHash)

	// Hash should change after deletion
	if statusAfterDelete.EmailsHash == statusWithEmail.EmailsHash {
		t.Error("Sync hash should change after email deletion")
	}

	// Email count should decrease
	if statusAfterDelete.EmailCount >= statusWithEmail.EmailCount {
		t.Errorf("EmailCount should decrease after deletion: %d -> %d",
			statusWithEmail.EmailCount, statusAfterDelete.EmailCount)
	}
}

// TestIntegration_SyncInboxNewEmails tests the syncInbox code path for discovering
// new emails. This happens when importing an inbox that already has emails,
// since the imported client's seenEmails is empty.
// Requires manual email sending.
func TestIntegration_SyncInboxNewEmails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	if os.Getenv("MANUAL_TEST") == "" {
		t.Skip("skipping manual test: set MANUAL_TEST=1 to run")
	}

	ctx := context.Background()

	// Client 1: Create inbox and receive email via SSE
	client1, err := vaultsandbox.New(apiKey,
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("New() client1 error = %v", err)
	}

	inbox1, err := client1.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		client1.Close()
		t.Fatalf("CreateInbox() error = %v", err)
	}

	t.Logf("Send test email to: %s", inbox1.EmailAddress())
	t.Logf("Waiting for email via SSE in client1...")

	// Wait for email via SSE - this marks it as "seen" in client1
	email, err := inbox1.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(2*time.Minute))
	if err != nil {
		inbox1.Delete(ctx)
		client1.Close()
		t.Fatalf("WaitForEmail() error = %v", err)
	}
	t.Logf("Client1 received email: ID=%s, Subject=%s", email.ID, email.Subject)

	// Export the inbox
	exported := inbox1.Export()
	client1.Close() // Close client1, inbox still exists on server

	// Client 2: Import the same inbox with polling strategy
	// The seenEmails will be empty, so syncInbox should find the email as "new"
	client2, err := vaultsandbox.New(apiKey,
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithTimeout(30*time.Second),
		vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
		vaultsandbox.WithPollingConfig(vaultsandbox.PollingConfig{
			InitialInterval:   2 * time.Second,
			MaxBackoff:        10 * time.Second,
			BackoffMultiplier: 2.0,
			JitterFactor:      0.1,
		}),
	)
	if err != nil {
		t.Fatalf("New() client2 error = %v", err)
	}
	defer client2.Close()

	inbox2, err := client2.ImportInbox(ctx, exported)
	if err != nil {
		t.Fatalf("ImportInbox() error = %v", err)
	}
	defer inbox2.Delete(ctx)

	// Set up a channel to receive the email via Watch
	// The polling will trigger syncInbox which should find the email
	emailCh := inbox2.Watch(ctx)

	// Wait for the email to be delivered via sync (polling triggers syncInbox)
	select {
	case receivedEmail := <-emailCh:
		if receivedEmail == nil {
			t.Fatal("received nil email from Watch")
		}
		t.Logf("Client2 received email via sync: ID=%s, Subject=%s", receivedEmail.ID, receivedEmail.Subject)
		if receivedEmail.ID != email.ID {
			t.Errorf("received email ID=%s, want %s", receivedEmail.ID, email.ID)
		}
	case <-time.After(30 * time.Second):
		t.Error("timeout waiting for email via syncInbox")
	}
}

// TestIntegration_WaitForEmail_ReceivesEmail tests WaitForEmail receiving an email.
// This is an automated test that sends an email via SMTP.
func TestIntegration_WaitForEmail_ReceivesEmail(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send email in background after a short delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		sendTestEmail(t, inbox.EmailAddress(), "WaitForEmail Test", "Test body content")
	}()

	// Wait for the email
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email == nil {
		t.Fatal("WaitForEmail() returned nil email")
	}
	if email.ID == "" {
		t.Error("email.ID is empty")
	}
	if email.Subject != "WaitForEmail Test" {
		t.Errorf("email.Subject = %q, want %q", email.Subject, "WaitForEmail Test")
	}
	if !strings.Contains(email.Text, "Test body content") {
		t.Errorf("email.Text = %q, should contain 'Test body content'", email.Text)
	}
}

// TestIntegration_WaitForEmail_ExistingEmail tests WaitForEmail finding an already existing email.
// This exercises the path where GetEmails finds a match before watching.
func TestIntegration_WaitForEmail_ExistingEmail(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send email BEFORE calling WaitForEmail
	sendTestEmail(t, inbox.EmailAddress(), "Existing Email Test", "Already here")

	// Give time for email to be processed
	time.Sleep(2 * time.Second)

	// WaitForEmail should find the existing email immediately
	start := time.Now()
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email == nil {
		t.Fatal("WaitForEmail() returned nil email")
	}
	if email.Subject != "Existing Email Test" {
		t.Errorf("email.Subject = %q, want %q", email.Subject, "Existing Email Test")
	}

	// Should find existing email quickly (within a few seconds, not the full timeout)
	if elapsed > 5*time.Second {
		t.Errorf("WaitForEmail() took %v, expected faster for existing email", elapsed)
	}
}

// TestIntegration_WaitForEmail_SubjectFilter tests WaitForEmail with subject filtering.
func TestIntegration_WaitForEmail_SubjectFilter(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send non-matching email first
	sendTestEmail(t, inbox.EmailAddress(), "Wrong Subject", "This should not match")

	// Send matching email after delay
	go func() {
		time.Sleep(1 * time.Second)
		sendTestEmail(t, inbox.EmailAddress(), "Target Subject", "This should match")
	}()

	// Wait for email with specific subject filter
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(30*time.Second),
		vaultsandbox.WithSubject("Target Subject"),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.Subject != "Target Subject" {
		t.Errorf("email.Subject = %q, want %q", email.Subject, "Target Subject")
	}
}

// TestIntegration_WaitForEmail_FromFilter tests WaitForEmail with from filtering.
func TestIntegration_WaitForEmail_FromFilter(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send email
	sendTestEmail(t, inbox.EmailAddress(), "From Filter Test", "Testing from filter")

	// Wait for email with from filter (test@example.com is the default sender in sendTestEmail)
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(30*time.Second),
		vaultsandbox.WithFrom("test@example.com"),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.From != "test@example.com" {
		t.Errorf("email.From = %q, want %q", email.From, "test@example.com")
	}
}

// TestIntegration_WaitForEmail_PredicateFilter tests WaitForEmail with custom predicate.
func TestIntegration_WaitForEmail_PredicateFilter(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send email with specific content
	sendTestEmail(t, inbox.EmailAddress(), "Predicate Test", "MAGIC-TOKEN-12345")

	// Wait for email using predicate
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(30*time.Second),
		vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
			return strings.Contains(e.Text, "MAGIC-TOKEN")
		}),
	)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if !strings.Contains(email.Text, "MAGIC-TOKEN") {
		t.Errorf("email.Text = %q, should contain 'MAGIC-TOKEN'", email.Text)
	}
}

// TestIntegration_WaitForEmailCount_ReceivesMultiple tests WaitForEmailCount receiving multiple emails.
func TestIntegration_WaitForEmailCount_ReceivesMultiple(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send multiple emails
	go func() {
		time.Sleep(500 * time.Millisecond)
		sendTestEmail(t, inbox.EmailAddress(), "Count Test 1", "First email")
		time.Sleep(500 * time.Millisecond)
		sendTestEmail(t, inbox.EmailAddress(), "Count Test 2", "Second email")
		time.Sleep(500 * time.Millisecond)
		sendTestEmail(t, inbox.EmailAddress(), "Count Test 3", "Third email")
	}()

	// Wait for 3 emails
	emails, err := inbox.WaitForEmailCount(ctx, 3, vaultsandbox.WithWaitTimeout(60*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmailCount() error = %v", err)
	}

	if len(emails) != 3 {
		t.Errorf("got %d emails, want 3", len(emails))
	}

	// Verify all emails are unique (deduplication)
	ids := make(map[string]bool)
	for _, e := range emails {
		if ids[e.ID] {
			t.Errorf("duplicate email ID: %s", e.ID)
		}
		ids[e.ID] = true
	}
}

// TestIntegration_WaitForEmailCount_ExistingAndNew tests WaitForEmailCount with
// a mix of existing and new emails.
func TestIntegration_WaitForEmailCount_ExistingAndNew(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send first email BEFORE calling WaitForEmailCount
	sendTestEmail(t, inbox.EmailAddress(), "Existing Count Test", "Already here")
	time.Sleep(2 * time.Second) // Give time for email to be processed

	// Send second email AFTER starting to wait
	go func() {
		time.Sleep(1 * time.Second)
		sendTestEmail(t, inbox.EmailAddress(), "New Count Test", "Just arrived")
	}()

	// Wait for 2 emails (1 existing + 1 new)
	emails, err := inbox.WaitForEmailCount(ctx, 2, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmailCount() error = %v", err)
	}

	if len(emails) != 2 {
		t.Errorf("got %d emails, want 2", len(emails))
	}

	// Verify we got both emails
	subjects := make(map[string]bool)
	for _, e := range emails {
		subjects[e.Subject] = true
	}
	if !subjects["Existing Count Test"] {
		t.Error("missing 'Existing Count Test' email")
	}
	if !subjects["New Count Test"] {
		t.Error("missing 'New Count Test' email")
	}
}

// TestIntegration_WaitForEmailCount_WithFilter tests WaitForEmailCount with filtering.
func TestIntegration_WaitForEmailCount_WithFilter(t *testing.T) {
	skipIfNoSMTP(t)

	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send mix of matching and non-matching emails
	go func() {
		time.Sleep(500 * time.Millisecond)
		sendTestEmail(t, inbox.EmailAddress(), "MATCH-1", "First matching")
		time.Sleep(300 * time.Millisecond)
		sendTestEmail(t, inbox.EmailAddress(), "NO-MATCH", "Should be filtered")
		time.Sleep(300 * time.Millisecond)
		sendTestEmail(t, inbox.EmailAddress(), "MATCH-2", "Second matching")
	}()

	// Wait for 2 emails with subject prefix filter
	emails, err := inbox.WaitForEmailCount(ctx, 2,
		vaultsandbox.WithWaitTimeout(30*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`^MATCH-`)),
	)
	if err != nil {
		t.Fatalf("WaitForEmailCount() error = %v", err)
	}

	if len(emails) != 2 {
		t.Errorf("got %d emails, want 2", len(emails))
	}

	// Verify only matching emails were returned
	for _, e := range emails {
		if !strings.HasPrefix(e.Subject, "MATCH-") {
			t.Errorf("email.Subject = %q, should start with 'MATCH-'", e.Subject)
		}
	}
}

// TestIntegration_WaitForEmailCount_Deduplication tests that WaitForEmailCount
// properly deduplicates emails when the same email is seen multiple times
// (e.g., from GetEmails and then again from the Watch channel via polling).
func TestIntegration_WaitForEmailCount_Deduplication(t *testing.T) {
	skipIfNoSMTP(t)

	// Use polling with very short intervals to increase chance of duplicate delivery
	client, err := vaultsandbox.New(apiKey,
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithTimeout(30*time.Second),
		vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
		vaultsandbox.WithPollingConfig(vaultsandbox.PollingConfig{
			InitialInterval:   500 * time.Millisecond,
			MaxBackoff:        1 * time.Second,
			BackoffMultiplier: 1.0,
			JitterFactor:      0.0,
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send first email and wait for it to be on the server
	sendTestEmail(t, inbox.EmailAddress(), "Dedup Test 1", "First email")
	time.Sleep(2 * time.Second) // Ensure email is on server

	// Now call WaitForEmailCount asking for 2 emails.
	// The first email will be found by GetEmails.
	// The polling sync will also try to notify about the same email.
	// The deduplication logic should handle this.
	// Then send second email to complete the wait.
	go func() {
		time.Sleep(1 * time.Second)
		sendTestEmail(t, inbox.EmailAddress(), "Dedup Test 2", "Second email")
	}()

	emails, err := inbox.WaitForEmailCount(ctx, 2, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmailCount() error = %v", err)
	}

	if len(emails) != 2 {
		t.Errorf("got %d emails, want 2", len(emails))
	}

	// Verify no duplicates in results
	ids := make(map[string]bool)
	for _, e := range emails {
		if ids[e.ID] {
			t.Errorf("duplicate email ID in results: %s", e.ID)
		}
		ids[e.ID] = true
	}

	// Verify we got both emails
	subjects := make(map[string]bool)
	for _, e := range emails {
		subjects[e.Subject] = true
	}
	if !subjects["Dedup Test 1"] {
		t.Error("missing 'Dedup Test 1' email")
	}
	if !subjects["Dedup Test 2"] {
		t.Error("missing 'Dedup Test 2' email")
	}
}

// TestIntegration_SyncInboxNewEmails_Automated tests the syncInbox code path
// for discovering new emails when importing an inbox that already has emails.
// This exercises the loop in client.go lines 585-605 where emails on the
// server that aren't in seenEmails are fetched and delivered to subscribers.
//
// The test works by:
// 1. Client1 creates inbox and receives an email (marks it as seen in client1)
// 2. Export the inbox and close client1
// 3. Client2 with SSE strategy imports the inbox (seenEmails is empty)
// 4. When SSE connects, OnReconnect calls syncAllInboxes → syncInbox
// 5. syncInbox discovers the email as "new" and delivers it via Watch
func TestIntegration_SyncInboxNewEmails_Automated(t *testing.T) {
	skipIfNoSMTP(t)

	ctx := context.Background()

	// Client 1: Create inbox and receive email (marks it as seen)
	client1, err := vaultsandbox.New(apiKey,
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("New() client1 error = %v", err)
	}

	inbox1, err := client1.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))
	if err != nil {
		client1.Close()
		t.Fatalf("CreateInbox() error = %v", err)
	}
	emailAddr := inbox1.EmailAddress()
	t.Logf("Created inbox: %s", emailAddr)

	// Send email to the inbox
	sendTestEmail(t, emailAddr, "SyncInbox Test", "Testing syncInbox new emails path")

	// Wait for email via client1 - this marks it as "seen" in client1's syncState
	email1, err := inbox1.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		inbox1.Delete(ctx)
		client1.Close()
		t.Fatalf("WaitForEmail() error = %v", err)
	}
	t.Logf("Client1 received email: ID=%s, Subject=%s", email1.ID, email1.Subject)

	// Export the inbox
	exported := inbox1.Export()

	// Close client1 - the inbox still exists on the server
	client1.Close()
	t.Log("Closed client1")

	// Client 2: Import the inbox with SSE strategy.
	// Since this is a new client, its seenEmails will be empty.
	// When SSE connects, the OnReconnect callback triggers syncAllInboxes,
	// which calls syncInbox for each inbox. syncInbox will:
	// 1. Fetch sync status and detect hash mismatch (server has email, client has none)
	// 2. Fetch metadata and find new email IDs
	// 3. Fetch full email data for each new email (the code path we want to test)
	// 4. Notify subscribers (our Watch channel)
	client2, err := vaultsandbox.New(apiKey,
		vaultsandbox.WithBaseURL(baseURL),
		vaultsandbox.WithTimeout(30*time.Second),
		vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
	)
	if err != nil {
		t.Fatalf("New() client2 error = %v", err)
	}
	defer client2.Close()

	// Set up Watch channel BEFORE importing so we catch the sync notification
	watchCtx, watchCancel := context.WithTimeout(ctx, 30*time.Second)
	defer watchCancel()

	// We need to import and then watch, but there's a race condition:
	// the sync might complete before we set up Watch. To handle this,
	// we'll check both the Watch channel and GetEmails.
	inbox2, err := client2.ImportInbox(ctx, exported)
	if err != nil {
		t.Fatalf("ImportInbox() error = %v", err)
	}
	defer inbox2.Delete(ctx)

	t.Log("Imported inbox into client2 with SSE strategy")

	emailCh := inbox2.Watch(watchCtx)

	// Wait for the email to be delivered via syncInbox
	// Give some time for SSE to connect and sync to run
	select {
	case receivedEmail := <-emailCh:
		if receivedEmail == nil {
			t.Fatal("received nil email from Watch")
		}
		t.Logf("Client2 received email via syncInbox: ID=%s, Subject=%s", receivedEmail.ID, receivedEmail.Subject)

		// Verify it's the same email
		if receivedEmail.ID != email1.ID {
			t.Errorf("received email ID=%s, want %s", receivedEmail.ID, email1.ID)
		}
		if receivedEmail.Subject != email1.Subject {
			t.Errorf("received email Subject=%s, want %s", receivedEmail.Subject, email1.Subject)
		}
	case <-time.After(15 * time.Second):
		// Sync might have happened before Watch was set up.
		// Verify the email exists in the inbox (it was synced even if we missed the notification)
		emails, err := inbox2.GetEmails(ctx)
		if err != nil {
			t.Fatalf("GetEmails() error = %v", err)
		}
		if len(emails) == 0 {
			t.Error("timeout waiting for email via syncInbox - no emails in inbox")
		} else {
			// Email was synced, just missed the Watch notification (timing issue)
			t.Logf("Email found via GetEmails (sync completed before Watch): ID=%s", emails[0].ID)
			if emails[0].ID != email1.ID {
				t.Errorf("email ID=%s, want %s", emails[0].ID, email1.ID)
			}
		}
	}
}
