//go:build integration

package integration

import (
	"context"
	"os"
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
	opts := []vaultsandbox.Option{
		vaultsandbox.WithBaseURL(baseURL),
	}

	client, err := vaultsandbox.New("invalid-api-key-12345", opts...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Invalid API key should error
	if err := client.CheckKey(ctx); err == nil {
		t.Error("CheckKey() should return error for invalid API key")
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
