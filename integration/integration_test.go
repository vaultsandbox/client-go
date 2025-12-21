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
