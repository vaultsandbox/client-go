//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
	"github.com/vaultsandbox/client-go/authresults"
)

// testEmailRequest represents the request body for the test email API.
type testEmailRequest struct {
	To      string          `json:"to"`
	From    string          `json:"from,omitempty"`
	Subject string          `json:"subject,omitempty"`
	Text    string          `json:"text,omitempty"`
	HTML    string          `json:"html,omitempty"`
	Auth    *testEmailAuth  `json:"auth,omitempty"`
}

// testEmailAuth represents the auth configuration for test emails.
type testEmailAuth struct {
	SPF        string `json:"spf,omitempty"`
	DKIM       string `json:"dkim,omitempty"`
	DMARC      string `json:"dmarc,omitempty"`
	ReverseDNS *bool  `json:"reverseDns,omitempty"`
}

// testEmailResponse represents the response from the test email API.
type testEmailResponse struct {
	EmailID string `json:"emailId"`
}

// sendTestEmailWithAuth sends a test email via the test email API with auth configuration.
func sendTestEmailWithAuth(t *testing.T, req testEmailRequest) string {
	t.Helper()

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal test email request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", baseURL+"/api/test/emails", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create test email request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("send test email: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("test email API returned status %d (expected 201)", resp.StatusCode)
	}

	var result testEmailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode test email response: %v", err)
	}

	return result.EmailID
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool {
	return &b
}

// TestIntegration_AuthResults_AllPass tests parsing auth results when all checks pass.
func TestIntegration_AuthResults_AllPass(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send test email with all auth passing
	sendTestEmailWithAuth(t, testEmailRequest{
		To:      inbox.EmailAddress(),
		From:    "sender@example.com",
		Subject: "Auth All Pass Test",
		Text:    "Testing all authentication passing",
	})

	// Wait for email
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	// Verify auth results
	if email.AuthResults == nil {
		t.Fatal("AuthResults is nil")
	}

	// Check SPF
	if email.AuthResults.SPF == nil {
		t.Error("SPF result is nil")
	} else if email.AuthResults.SPF.Result != "pass" {
		t.Errorf("SPF.Result = %q, want %q", email.AuthResults.SPF.Result, "pass")
	}

	// Check DKIM
	if len(email.AuthResults.DKIM) == 0 {
		t.Error("DKIM results are empty")
	} else if email.AuthResults.DKIM[0].Result != "pass" {
		t.Errorf("DKIM[0].Result = %q, want %q", email.AuthResults.DKIM[0].Result, "pass")
	}

	// Check DMARC
	if email.AuthResults.DMARC == nil {
		t.Error("DMARC result is nil")
	} else if email.AuthResults.DMARC.Result != "pass" {
		t.Errorf("DMARC.Result = %q, want %q", email.AuthResults.DMARC.Result, "pass")
	}

	// Check ReverseDNS
	if email.AuthResults.ReverseDNS == nil {
		t.Error("ReverseDNS result is nil")
	} else if !email.AuthResults.ReverseDNS.Verified {
		t.Error("ReverseDNS.Verified = false, want true")
	}

	// Verify validation passes
	validation := email.AuthResults.Validate()
	if !validation.Passed {
		t.Errorf("Validate().Passed = false, want true; failures = %v", validation.Failures)
	}
	if !validation.SPFPassed {
		t.Error("SPFPassed = false, want true")
	}
	if !validation.DKIMPassed {
		t.Error("DKIMPassed = false, want true")
	}
	if !validation.DMARCPassed {
		t.Error("DMARCPassed = false, want true")
	}
	if !validation.ReverseDNSPassed {
		t.Error("ReverseDNSPassed = false, want true")
	}

	// Verify IsPassing matches
	if !email.AuthResults.IsPassing() {
		t.Error("IsPassing() = false, want true")
	}

	t.Logf("Auth results: SPF=%s, DKIM=%s, DMARC=%s, ReverseDNS=%v",
		email.AuthResults.SPF.Result,
		email.AuthResults.DKIM[0].Result,
		email.AuthResults.DMARC.Result,
		email.AuthResults.ReverseDNS.Verified)
}

// TestIntegration_AuthResults_SPFFail tests parsing auth results when SPF fails.
func TestIntegration_AuthResults_SPFFail(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send test email with SPF fail
	sendTestEmailWithAuth(t, testEmailRequest{
		To:      inbox.EmailAddress(),
		From:    "sender@example.com",
		Subject: "SPF Fail Test",
		Auth: &testEmailAuth{
			SPF: "fail",
		},
	})

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.AuthResults == nil {
		t.Fatal("AuthResults is nil")
	}

	// SPF should fail
	if email.AuthResults.SPF == nil {
		t.Error("SPF result is nil")
	} else if email.AuthResults.SPF.Result != "fail" {
		t.Errorf("SPF.Result = %q, want %q", email.AuthResults.SPF.Result, "fail")
	}

	// Validation should fail
	validation := email.AuthResults.Validate()
	if validation.Passed {
		t.Error("Validate().Passed = true, want false (SPF failed)")
	}
	if validation.SPFPassed {
		t.Error("SPFPassed = true, want false")
	}

	// Check that failure message mentions SPF
	if len(validation.Failures) == 0 {
		t.Error("Failures is empty, expected SPF failure message")
	} else {
		found := false
		for _, f := range validation.Failures {
			if contains(f, "SPF") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Failures %v does not contain SPF failure", validation.Failures)
		}
	}

	t.Logf("SPF failure test passed: Result=%s, Failures=%v",
		email.AuthResults.SPF.Result, validation.Failures)
}

// TestIntegration_AuthResults_DKIMFail tests parsing auth results when DKIM fails.
func TestIntegration_AuthResults_DKIMFail(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send test email with DKIM fail
	sendTestEmailWithAuth(t, testEmailRequest{
		To:      inbox.EmailAddress(),
		From:    "sender@example.com",
		Subject: "DKIM Fail Test",
		Auth: &testEmailAuth{
			DKIM: "fail",
		},
	})

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.AuthResults == nil {
		t.Fatal("AuthResults is nil")
	}

	// DKIM should fail
	if len(email.AuthResults.DKIM) == 0 {
		t.Error("DKIM results are empty")
	} else if email.AuthResults.DKIM[0].Result != "fail" {
		t.Errorf("DKIM[0].Result = %q, want %q", email.AuthResults.DKIM[0].Result, "fail")
	}

	// Validation should fail
	validation := email.AuthResults.Validate()
	if validation.Passed {
		t.Error("Validate().Passed = true, want false (DKIM failed)")
	}
	if validation.DKIMPassed {
		t.Error("DKIMPassed = true, want false")
	}

	t.Logf("DKIM failure test passed: Result=%s", email.AuthResults.DKIM[0].Result)
}

// TestIntegration_AuthResults_DMARCFail tests parsing auth results when DMARC fails.
func TestIntegration_AuthResults_DMARCFail(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send test email with DMARC fail
	sendTestEmailWithAuth(t, testEmailRequest{
		To:      inbox.EmailAddress(),
		From:    "sender@example.com",
		Subject: "DMARC Fail Test",
		Auth: &testEmailAuth{
			DMARC: "fail",
		},
	})

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.AuthResults == nil {
		t.Fatal("AuthResults is nil")
	}

	// DMARC should fail
	if email.AuthResults.DMARC == nil {
		t.Error("DMARC result is nil")
	} else if email.AuthResults.DMARC.Result != "fail" {
		t.Errorf("DMARC.Result = %q, want %q", email.AuthResults.DMARC.Result, "fail")
	}

	// Validation should fail
	validation := email.AuthResults.Validate()
	if validation.Passed {
		t.Error("Validate().Passed = true, want false (DMARC failed)")
	}
	if validation.DMARCPassed {
		t.Error("DMARCPassed = true, want false")
	}

	t.Logf("DMARC failure test passed: Result=%s", email.AuthResults.DMARC.Result)
}

// TestIntegration_AuthResults_ReverseDNSFail tests parsing auth results when ReverseDNS fails.
func TestIntegration_AuthResults_ReverseDNSFail(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send test email with ReverseDNS fail
	sendTestEmailWithAuth(t, testEmailRequest{
		To:      inbox.EmailAddress(),
		From:    "sender@example.com",
		Subject: "ReverseDNS Fail Test",
		Auth: &testEmailAuth{
			ReverseDNS: boolPtr(false),
		},
	})

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.AuthResults == nil {
		t.Fatal("AuthResults is nil")
	}

	// ReverseDNS should fail
	if email.AuthResults.ReverseDNS == nil {
		t.Error("ReverseDNS result is nil")
	} else if email.AuthResults.ReverseDNS.Verified {
		t.Error("ReverseDNS.Verified = true, want false")
	}

	// Overall validation should still pass (ReverseDNS not included in Passed)
	validation := email.AuthResults.Validate()
	if !validation.Passed {
		t.Errorf("Validate().Passed = false, want true (ReverseDNS doesn't affect Passed)")
	}
	if validation.ReverseDNSPassed {
		t.Error("ReverseDNSPassed = true, want false")
	}

	// Should have a failure message for ReverseDNS
	if len(validation.Failures) == 0 {
		t.Error("Failures is empty, expected ReverseDNS failure message")
	}

	t.Logf("ReverseDNS failure test passed: Verified=%v, Failures=%v",
		email.AuthResults.ReverseDNS.Verified, validation.Failures)
}

// TestIntegration_AuthResults_AllFail tests parsing auth results when all checks fail.
func TestIntegration_AuthResults_AllFail(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send test email with all auth failing
	sendTestEmailWithAuth(t, testEmailRequest{
		To:      inbox.EmailAddress(),
		From:    "sender@example.com",
		Subject: "All Auth Fail Test",
		Auth: &testEmailAuth{
			SPF:        "fail",
			DKIM:       "fail",
			DMARC:      "fail",
			ReverseDNS: boolPtr(false),
		},
	})

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.AuthResults == nil {
		t.Fatal("AuthResults is nil")
	}

	// All checks should fail
	if email.AuthResults.SPF == nil || email.AuthResults.SPF.Result != "fail" {
		t.Errorf("SPF.Result = %v, want fail", email.AuthResults.SPF)
	}
	if len(email.AuthResults.DKIM) == 0 || email.AuthResults.DKIM[0].Result != "fail" {
		t.Errorf("DKIM[0].Result = %v, want fail", email.AuthResults.DKIM)
	}
	if email.AuthResults.DMARC == nil || email.AuthResults.DMARC.Result != "fail" {
		t.Errorf("DMARC.Result = %v, want fail", email.AuthResults.DMARC)
	}
	if email.AuthResults.ReverseDNS == nil || email.AuthResults.ReverseDNS.Verified {
		t.Errorf("ReverseDNS.Verified = %v, want false", email.AuthResults.ReverseDNS)
	}

	// Validation should fail
	validation := email.AuthResults.Validate()
	if validation.Passed {
		t.Error("Validate().Passed = true, want false")
	}
	if validation.SPFPassed {
		t.Error("SPFPassed = true, want false")
	}
	if validation.DKIMPassed {
		t.Error("DKIMPassed = true, want false")
	}
	if validation.DMARCPassed {
		t.Error("DMARCPassed = true, want false")
	}
	if validation.ReverseDNSPassed {
		t.Error("ReverseDNSPassed = true, want false")
	}

	// Should have multiple failure messages
	if len(validation.Failures) < 3 {
		t.Errorf("Expected at least 3 failures, got %d: %v", len(validation.Failures), validation.Failures)
	}

	// IsPassing should be false
	if email.AuthResults.IsPassing() {
		t.Error("IsPassing() = true, want false")
	}

	t.Logf("All auth failing test passed: Failures=%v", validation.Failures)
}

// TestIntegration_AuthResults_SPFSoftfail tests parsing SPF softfail result.
func TestIntegration_AuthResults_SPFSoftfail(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send test email with SPF softfail
	sendTestEmailWithAuth(t, testEmailRequest{
		To:      inbox.EmailAddress(),
		From:    "sender@example.com",
		Subject: "SPF Softfail Test",
		Auth: &testEmailAuth{
			SPF: "softfail",
		},
	})

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.AuthResults == nil {
		t.Fatal("AuthResults is nil")
	}

	// SPF should be softfail
	if email.AuthResults.SPF == nil {
		t.Error("SPF result is nil")
	} else if email.AuthResults.SPF.Result != "softfail" {
		t.Errorf("SPF.Result = %q, want %q", email.AuthResults.SPF.Result, "softfail")
	}

	// softfail is not pass, so validation should fail
	validation := email.AuthResults.Validate()
	if validation.SPFPassed {
		t.Error("SPFPassed = true, want false (softfail is not pass)")
	}

	t.Logf("SPF softfail test passed: Result=%s", email.AuthResults.SPF.Result)
}

// TestIntegration_AuthResults_MixedResults tests mixed authentication results.
func TestIntegration_AuthResults_MixedResults(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send test email with mixed results
	sendTestEmailWithAuth(t, testEmailRequest{
		To:      inbox.EmailAddress(),
		From:    "sender@example.com",
		Subject: "Mixed Auth Results Test",
		Auth: &testEmailAuth{
			SPF:        "softfail",
			DKIM:       "pass",
			DMARC:      "fail",
			ReverseDNS: boolPtr(true),
		},
	})

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.AuthResults == nil {
		t.Fatal("AuthResults is nil")
	}

	// Verify individual results
	if email.AuthResults.SPF.Result != "softfail" {
		t.Errorf("SPF.Result = %q, want softfail", email.AuthResults.SPF.Result)
	}
	if email.AuthResults.DKIM[0].Result != "pass" {
		t.Errorf("DKIM[0].Result = %q, want pass", email.AuthResults.DKIM[0].Result)
	}
	if email.AuthResults.DMARC.Result != "fail" {
		t.Errorf("DMARC.Result = %q, want fail", email.AuthResults.DMARC.Result)
	}
	if !email.AuthResults.ReverseDNS.Verified {
		t.Error("ReverseDNS.Verified = false, want true")
	}

	// Verify validation
	validation := email.AuthResults.Validate()
	if validation.Passed {
		t.Error("Validate().Passed = true, want false (SPF softfail and DMARC fail)")
	}
	if validation.SPFPassed {
		t.Error("SPFPassed = true, want false")
	}
	if !validation.DKIMPassed {
		t.Error("DKIMPassed = false, want true")
	}
	if validation.DMARCPassed {
		t.Error("DMARCPassed = true, want false")
	}
	if !validation.ReverseDNSPassed {
		t.Error("ReverseDNSPassed = false, want true")
	}

	t.Logf("Mixed results test passed: SPF=%s, DKIM=%s, DMARC=%s, ReverseDNS=%v",
		email.AuthResults.SPF.Result,
		email.AuthResults.DKIM[0].Result,
		email.AuthResults.DMARC.Result,
		email.AuthResults.ReverseDNS.Verified)
}

// TestIntegration_AuthResults_ValidateFunction tests the standalone Validate function.
func TestIntegration_AuthResults_ValidateFunction(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send test email with all passing
	sendTestEmailWithAuth(t, testEmailRequest{
		To:      inbox.EmailAddress(),
		From:    "sender@example.com",
		Subject: "Validate Function Test",
	})

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.AuthResults == nil {
		t.Fatal("AuthResults is nil")
	}

	// Test the standalone Validate function from the authresults package
	err = authresults.Validate(email.AuthResults)
	if err != nil {
		t.Errorf("authresults.Validate() error = %v, want nil", err)
	}

	// Test individual validators
	if err := authresults.ValidateSPF(email.AuthResults); err != nil {
		t.Errorf("ValidateSPF() error = %v", err)
	}
	if err := authresults.ValidateDKIM(email.AuthResults); err != nil {
		t.Errorf("ValidateDKIM() error = %v", err)
	}
	if err := authresults.ValidateDMARC(email.AuthResults); err != nil {
		t.Errorf("ValidateDMARC() error = %v", err)
	}
	if err := authresults.ValidateReverseDNS(email.AuthResults); err != nil {
		t.Errorf("ValidateReverseDNS() error = %v", err)
	}

	t.Log("Validate function test passed")
}

// TestIntegration_AuthResults_ValidateFunctionFails tests standalone Validate with failures.
func TestIntegration_AuthResults_ValidateFunctionFails(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send test email with SPF fail
	sendTestEmailWithAuth(t, testEmailRequest{
		To:      inbox.EmailAddress(),
		From:    "sender@example.com",
		Subject: "Validate Function Fail Test",
		Auth: &testEmailAuth{
			SPF: "fail",
		},
	})

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.AuthResults == nil {
		t.Fatal("AuthResults is nil")
	}

	// Validate should return error
	err = authresults.Validate(email.AuthResults)
	if err == nil {
		t.Error("authresults.Validate() should return error for failed SPF")
	}

	// ValidateSPF should return ErrSPFFailed
	err = authresults.ValidateSPF(email.AuthResults)
	if err == nil {
		t.Error("ValidateSPF() should return error")
	}

	t.Logf("Validate function fail test passed: error = %v", err)
}

// TestIntegration_AuthResults_DomainExtraction tests that domain is extracted from From address.
func TestIntegration_AuthResults_DomainExtraction(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Send test email with specific from address
	fromAddr := "test@testdomain.example.com"
	sendTestEmailWithAuth(t, testEmailRequest{
		To:      inbox.EmailAddress(),
		From:    fromAddr,
		Subject: "Domain Extraction Test",
	})

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if email.AuthResults == nil {
		t.Fatal("AuthResults is nil")
	}

	// Domain should be extracted from the from address
	expectedDomain := "testdomain.example.com"

	if email.AuthResults.SPF != nil && email.AuthResults.SPF.Domain != "" {
		if email.AuthResults.SPF.Domain != expectedDomain {
			t.Errorf("SPF.Domain = %q, want %q", email.AuthResults.SPF.Domain, expectedDomain)
		}
	}

	if len(email.AuthResults.DKIM) > 0 && email.AuthResults.DKIM[0].Domain != "" {
		if email.AuthResults.DKIM[0].Domain != expectedDomain {
			t.Errorf("DKIM[0].Domain = %q, want %q", email.AuthResults.DKIM[0].Domain, expectedDomain)
		}
	}

	if email.AuthResults.DMARC != nil && email.AuthResults.DMARC.Domain != "" {
		if email.AuthResults.DMARC.Domain != expectedDomain {
			t.Errorf("DMARC.Domain = %q, want %q", email.AuthResults.DMARC.Domain, expectedDomain)
		}
	}

	t.Logf("Domain extraction test passed: SPF.Domain=%s, DKIM.Domain=%s, DMARC.Domain=%s",
		safeGetDomain(email.AuthResults.SPF),
		safeGetDKIMDomain(email.AuthResults.DKIM),
		safeGetDMARCDomain(email.AuthResults.DMARC))
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func safeGetDomain(spf *authresults.SPFResult) string {
	if spf == nil {
		return "<nil>"
	}
	return spf.Domain
}

func safeGetDKIMDomain(dkim []authresults.DKIMResult) string {
	if len(dkim) == 0 {
		return "<empty>"
	}
	return dkim[0].Domain
}

func safeGetDMARCDomain(dmarc *authresults.DMARCResult) string {
	if dmarc == nil {
		return "<nil>"
	}
	return dmarc.Domain
}

// TestIntegration_AuthResults_JSONUnmarshal tests that auth results unmarshal correctly.
func TestIntegration_AuthResults_JSONUnmarshal(t *testing.T) {
	// Test that the Result field correctly maps from the wire "result" field
	jsonData := `{
		"spf": {"result": "pass", "domain": "example.com"},
		"dkim": [{"result": "pass", "domain": "example.com", "selector": "test"}],
		"dmarc": {"result": "pass", "domain": "example.com", "policy": "none", "aligned": true},
		"reverseDns": {"verified": true, "ip": "127.0.0.1", "hostname": "mail.example.com"}
	}`

	var ar authresults.AuthResults
	if err := json.Unmarshal([]byte(jsonData), &ar); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify SPF
	if ar.SPF == nil {
		t.Error("SPF is nil")
	} else {
		if ar.SPF.Result != "pass" {
			t.Errorf("SPF.Result = %q, want %q", ar.SPF.Result, "pass")
		}
		if ar.SPF.Domain != "example.com" {
			t.Errorf("SPF.Domain = %q, want %q", ar.SPF.Domain, "example.com")
		}
	}

	// Verify DKIM
	if len(ar.DKIM) == 0 {
		t.Error("DKIM is empty")
	} else {
		if ar.DKIM[0].Result != "pass" {
			t.Errorf("DKIM[0].Result = %q, want %q", ar.DKIM[0].Result, "pass")
		}
		if ar.DKIM[0].Selector != "test" {
			t.Errorf("DKIM[0].Selector = %q, want %q", ar.DKIM[0].Selector, "test")
		}
	}

	// Verify DMARC
	if ar.DMARC == nil {
		t.Error("DMARC is nil")
	} else {
		if ar.DMARC.Result != "pass" {
			t.Errorf("DMARC.Result = %q, want %q", ar.DMARC.Result, "pass")
		}
		if ar.DMARC.Policy != "none" {
			t.Errorf("DMARC.Policy = %q, want %q", ar.DMARC.Policy, "none")
		}
		if !ar.DMARC.Aligned {
			t.Error("DMARC.Aligned = false, want true")
		}
	}

	// Verify ReverseDNS
	if ar.ReverseDNS == nil {
		t.Error("ReverseDNS is nil")
	} else {
		if !ar.ReverseDNS.Verified {
			t.Error("ReverseDNS.Verified = false, want true")
		}
		if ar.ReverseDNS.IP != "127.0.0.1" {
			t.Errorf("ReverseDNS.IP = %q, want %q", ar.ReverseDNS.IP, "127.0.0.1")
		}
		if ar.ReverseDNS.Hostname != "mail.example.com" {
			t.Errorf("ReverseDNS.Hostname = %q, want %q", ar.ReverseDNS.Hostname, "mail.example.com")
		}
	}

	// Verify validation passes
	validation := ar.Validate()
	if !validation.Passed {
		t.Errorf("Validate().Passed = false; failures = %v", validation.Failures)
	}

	t.Log("JSON unmarshal test passed")
}

// TestIntegration_AuthResults_JSONMarshal tests that auth results marshal correctly.
func TestIntegration_AuthResults_JSONMarshal(t *testing.T) {
	ar := &authresults.AuthResults{
		SPF: &authresults.SPFResult{
			Result:  "pass",
			Domain:  "example.com",
			IP:      "1.2.3.4",
			Details: "spf=pass",
		},
		DKIM: []authresults.DKIMResult{
			{
				Result:    "pass",
				Domain:    "example.com",
				Selector:  "selector1",
				Signature: "dkim=pass",
			},
		},
		DMARC: &authresults.DMARCResult{
			Result:  "pass",
			Policy:  "reject",
			Aligned: true,
			Domain:  "example.com",
		},
		ReverseDNS: &authresults.ReverseDNSResult{
			Verified: true,
			IP:       "1.2.3.4",
			Hostname: "mail.example.com",
		},
	}

	data, err := json.Marshal(ar)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Verify the JSON contains "result" not "Result" (wire format)
	jsonStr := string(data)
	if !containsHelper(jsonStr, `"result":"pass"`) {
		t.Errorf("JSON should contain \"result\":\"pass\", got: %s", jsonStr)
	}

	// Verify it can be unmarshaled back
	var ar2 authresults.AuthResults
	if err := json.Unmarshal(data, &ar2); err != nil {
		t.Fatalf("json.Unmarshal() roundtrip error = %v", err)
	}

	if ar2.SPF.Result != ar.SPF.Result {
		t.Errorf("SPF.Result roundtrip: got %q, want %q", ar2.SPF.Result, ar.SPF.Result)
	}

	t.Logf("JSON marshal test passed: %s", string(data))
}

// TestIntegration_AuthResults_Wire_Format verifies wire format compatibility.
func TestIntegration_AuthResults_Wire_Format(t *testing.T) {
	// This is the exact wire format from the API as documented in test-email-api.md
	wireFormat := `{
		"spf": {
			"result": "pass",
			"domain": "example.com",
			"details": "spf=pass (test email)"
		},
		"dkim": [{
			"domain": "example.com",
			"result": "pass",
			"selector": "test",
			"signature": "dkim=pass (test email)"
		}],
		"dmarc": {
			"result": "pass",
			"policy": "none",
			"domain": "example.com",
			"aligned": true
		},
		"reverseDns": {
			"hostname": "test.vaultsandbox.local",
			"verified": true,
			"ip": "127.0.0.1"
		}
	}`

	var ar authresults.AuthResults
	if err := json.Unmarshal([]byte(wireFormat), &ar); err != nil {
		t.Fatalf("Failed to unmarshal wire format: %v", err)
	}

	// Verify all fields are correctly parsed
	if ar.SPF == nil || ar.SPF.Result != "pass" {
		t.Errorf("SPF not parsed correctly: %+v", ar.SPF)
	}
	if len(ar.DKIM) == 0 || ar.DKIM[0].Result != "pass" {
		t.Errorf("DKIM not parsed correctly: %+v", ar.DKIM)
	}
	if ar.DMARC == nil || ar.DMARC.Result != "pass" {
		t.Errorf("DMARC not parsed correctly: %+v", ar.DMARC)
	}
	if ar.ReverseDNS == nil || !ar.ReverseDNS.Verified {
		t.Errorf("ReverseDNS not parsed correctly: %+v", ar.ReverseDNS)
	}

	// Verify validation
	if !ar.IsPassing() {
		validation := ar.Validate()
		t.Errorf("Wire format should validate as passing: %v", validation.Failures)
	}

	t.Log("Wire format compatibility test passed")
}

// Formatting helper for test output
func init() {
	// Suppress unused import warning for fmt
	_ = fmt.Sprintf
}
