---
title: Testing Password Reset Flows
description: Learn how to test password reset email flows with VaultSandbox
---

Password reset flows are one of the most common email testing scenarios. This guide demonstrates how to use VaultSandbox to test password reset emails end-to-end, including link extraction and email validation.

## Basic Password Reset Test

Here's a complete example of testing a password reset flow:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
	"github.com/vaultsandbox/client-go/authresults"
)

func main() {
	ctx := context.Background()

	client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// Trigger password reset in your application
	yourApp.RequestPasswordReset(inbox.EmailAddress())

	// Wait for and validate the reset email
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset your password`)),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Extract reset link
	var resetLink string
	for _, link := range email.Links {
		if strings.Contains(link, "/reset-password") {
			resetLink = link
			break
		}
	}
	fmt.Println("Reset link:", resetLink)

	// Validate email authentication
	validation := email.AuthResults.Validate()
	fmt.Println("Auth passed:", validation.Passed)
	fmt.Println("Failures:", validation.Failures)
}
```

## Go Testing Integration

Integrate password reset testing into your Go test suite:

```go
package yourapp_test

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var client *vaultsandbox.Client

func TestMain(m *testing.M) {
	var err error
	client, err = vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	os.Exit(m.Run())
}

func TestPasswordResetEmail(t *testing.T) {
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	require.NoError(t, err)
	defer inbox.Delete(ctx)

	// Trigger password reset
	requestPasswordReset(inbox.EmailAddress())

	// Wait for email
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset your password`)),
	)
	require.NoError(t, err)

	// Validate sender
	assert.Equal(t, "noreply@example.com", email.From)

	// Validate content
	assert.Contains(t, email.Text, "requested a password reset")
	assert.NotEmpty(t, email.HTML)

	// Extract and validate reset link
	var resetLink string
	for _, link := range email.Links {
		if strings.Contains(link, "/reset-password") {
			resetLink = link
			break
		}
	}
	require.NotEmpty(t, resetLink)
	assert.True(t, strings.HasPrefix(resetLink, "https://"))
	assert.Contains(t, resetLink, "token=")
}

func TestPasswordResetEmailContainsUserInfo(t *testing.T) {
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	require.NoError(t, err)
	defer inbox.Delete(ctx)

	userEmail := inbox.EmailAddress()
	userName := "John Doe"

	requestPasswordResetWithName(userEmail, userName)

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset your password`)),
	)
	require.NoError(t, err)

	// Verify personalization
	assert.Contains(t, email.Text, userName)
	assert.Contains(t, email.To, userEmail)
}

func TestResetLinkIsFunctional(t *testing.T) {
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	require.NoError(t, err)
	defer inbox.Delete(ctx)

	requestPasswordReset(inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset your password`)),
	)
	require.NoError(t, err)

	var resetLink string
	for _, link := range email.Links {
		if strings.Contains(link, "/reset-password") {
			resetLink = link
			break
		}
	}
	require.NotEmpty(t, resetLink)

	// Test that the link is accessible
	resp, err := http.Get(resetLink)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

## Link Extraction Patterns

VaultSandbox automatically extracts all links from emails. Here are common patterns for finding password reset links:

```go
// Find by path
var resetLink string
for _, link := range email.Links {
	if strings.Contains(link, "/reset-password") {
		resetLink = link
		break
	}
}

// Find by domain
for _, link := range email.Links {
	if strings.Contains(link, "yourdomain.com/reset") {
		resetLink = link
		break
	}
}

// Find by query parameter
for _, link := range email.Links {
	if strings.Contains(link, "token=") {
		resetLink = link
		break
	}
}

// Find using regex
resetPattern := regexp.MustCompile(`/reset.*token=`)
for _, link := range email.Links {
	if resetPattern.MatchString(link) {
		resetLink = link
		break
	}
}

// Extract token from link
parsedURL, err := url.Parse(resetLink)
if err != nil {
	log.Fatal(err)
}
token := parsedURL.Query().Get("token")
assert.NotEmpty(t, token)
assert.Greater(t, len(token), 20)
```

## Validating Email Content

Test the content and formatting of your password reset emails:

```go
func TestPasswordResetEmailFormatting(t *testing.T) {
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	require.NoError(t, err)
	defer inbox.Delete(ctx)

	requestPasswordReset(inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset your password`)),
	)
	require.NoError(t, err)

	// Validate plain text version
	assert.NotEmpty(t, email.Text)
	assert.Contains(t, email.Text, "reset your password")
	assert.NotContains(t, email.Text, "undefined")
	assert.NotContains(t, email.Text, "[object Object]")

	// Validate HTML version
	assert.NotEmpty(t, email.HTML)
	assert.Contains(t, email.HTML, "<a href=")
	assert.Contains(t, email.HTML, "reset")

	// Validate email has exactly one reset link
	resetCount := 0
	for _, link := range email.Links {
		if strings.Contains(link, "/reset-password") {
			resetCount++
		}
	}
	assert.Equal(t, 1, resetCount)

	// Validate headers
	_, hasContentType := email.Headers["content-type"]
	assert.True(t, hasContentType)
}
```

## Testing Security Features

Validate email authentication to ensure your emails won't be marked as spam:

```go
func TestEmailAuthenticationChecks(t *testing.T) {
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	require.NoError(t, err)
	defer inbox.Delete(ctx)

	requestPasswordReset(inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset your password`)),
	)
	require.NoError(t, err)

	// Validate authentication results
	require.NotNil(t, email.AuthResults)

	validation := email.AuthResults.Validate()

	// Check that validation was performed
	assert.NotNil(t, validation.Failures)

	// Log any failures for debugging
	if !validation.Passed {
		t.Logf("Email authentication failures: %v", validation.Failures)
	}

	// Check individual authentication methods
	if email.AuthResults.SPF != nil {
		assert.Regexp(t, regexp.MustCompile(`pass|neutral|softfail`), email.AuthResults.SPF.Status)
	}

	if len(email.AuthResults.DKIM) > 0 {
		assert.NotEmpty(t, email.AuthResults.DKIM[0].Status)
	}
}

func TestStrictEmailAuthentication(t *testing.T) {
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	require.NoError(t, err)
	defer inbox.Delete(ctx)

	requestPasswordReset(inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset your password`)),
	)
	require.NoError(t, err)

	// Use the package-level Validate function for strict validation
	err = authresults.Validate(email.AuthResults)
	if err != nil {
		// Log validation errors but don't fail the test
		// (useful for development environments without full email auth)
		t.Logf("Email authentication validation: %v", err)
	}
}
```

## Testing Reset Token Expiration

Test that your password reset emails include expiration information:

```go
func TestResetEmailIncludesExpiration(t *testing.T) {
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	require.NoError(t, err)
	defer inbox.Delete(ctx)

	requestPasswordReset(inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset your password`)),
	)
	require.NoError(t, err)

	// Validate expiration is mentioned
	textLower := strings.ToLower(email.Text)
	hasExpiration := strings.Contains(textLower, "expires") ||
		strings.Contains(textLower, "valid for") ||
		strings.Contains(textLower, "24 hours") ||
		strings.Contains(textLower, "1 hour")

	assert.True(t, hasExpiration, "Email should mention expiration time")
}
```

## Testing Multiple Reset Requests

Test what happens when a user requests multiple password resets:

```go
func TestMultipleResetRequests(t *testing.T) {
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	require.NoError(t, err)
	defer inbox.Delete(ctx)

	// Request multiple resets
	requestPasswordReset(inbox.EmailAddress())
	requestPasswordReset(inbox.EmailAddress())

	// Wait for both emails
	emails, err := inbox.WaitForEmailCount(ctx, 2,
		vaultsandbox.WithWaitTimeout(15*time.Second),
	)
	require.NoError(t, err)
	assert.Len(t, emails, 2)

	// Both should have reset links
	for _, email := range emails {
		var hasResetLink bool
		for _, link := range email.Links {
			if strings.Contains(link, "/reset-password") {
				hasResetLink = true
				break
			}
		}
		assert.True(t, hasResetLink, "Each email should have a reset link")
	}

	// Extract tokens and verify they're different
	var tokens []string
	for _, email := range emails {
		for _, link := range email.Links {
			if strings.Contains(link, "/reset-password") {
				parsedURL, err := url.Parse(link)
				require.NoError(t, err)
				tokens = append(tokens, parsedURL.Query().Get("token"))
				break
			}
		}
	}

	require.Len(t, tokens, 2)
	assert.NotEqual(t, tokens[0], tokens[1], "Tokens should be different")
}
```

## Best Practices

### Use Specific Subject Filters

Always filter by subject to ensure you're testing the right email:

```go
// Good: Specific subject filter
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithWaitTimeout(10*time.Second),
	vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset your password`)),
)

// Avoid: No filter (might match wrong email in CI)
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithWaitTimeout(10*time.Second),
)
```

### Clean Up Inboxes

Always delete inboxes after tests using defer:

```go
func TestPasswordReset(t *testing.T) {
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	require.NoError(t, err)
	defer inbox.Delete(ctx) // Always clean up

	// ... test code
}
```

### Use Appropriate Timeouts

Set realistic timeouts based on your email delivery speed:

```go
// Local development: shorter timeout
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithWaitTimeout(5*time.Second),
)

// CI/CD: longer timeout to account for slower environments
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithWaitTimeout(15*time.Second),
)
```

### Test Complete Flow

Don't just validate that the email was sent - test that the link actually works:

```go
func TestCompletePasswordResetFlow(t *testing.T) {
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	require.NoError(t, err)
	defer inbox.Delete(ctx)

	// 1. Request reset
	requestPasswordReset(inbox.EmailAddress())

	// 2. Get email
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset your password`)),
	)
	require.NoError(t, err)

	// 3. Extract link
	var resetLink string
	for _, link := range email.Links {
		if strings.Contains(link, "/reset-password") {
			resetLink = link
			break
		}
	}
	require.NotEmpty(t, resetLink)

	// 4. Visit reset page
	resp, err := http.Get(resetLink)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// 5. Submit new password
	newPassword := "NewSecurePassword123!"
	err = submitPasswordReset(resetLink, newPassword)
	require.NoError(t, err)

	// 6. Verify login with new password
	loginSuccess, err := login(inbox.EmailAddress(), newPassword)
	require.NoError(t, err)
	assert.True(t, loginSuccess)
}
```

### Use Table-Driven Tests

For testing multiple scenarios, use Go's table-driven test pattern:

```go
func TestPasswordResetScenarios(t *testing.T) {
	tests := []struct {
		name           string
		subjectPattern *regexp.Regexp
		expectedFrom   string
		shouldHaveLink bool
	}{
		{
			name:           "standard reset",
			subjectPattern: regexp.MustCompile(`(?i)reset your password`),
			expectedFrom:   "noreply@example.com",
			shouldHaveLink: true,
		},
		{
			name:           "security alert reset",
			subjectPattern: regexp.MustCompile(`(?i)security.*reset`),
			expectedFrom:   "security@example.com",
			shouldHaveLink: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			inbox, err := client.CreateInbox(ctx)
			require.NoError(t, err)
			defer inbox.Delete(ctx)

			requestPasswordReset(inbox.EmailAddress())

			email, err := inbox.WaitForEmail(ctx,
				vaultsandbox.WithWaitTimeout(10*time.Second),
				vaultsandbox.WithSubjectRegex(tt.subjectPattern),
			)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedFrom, email.From)

			if tt.shouldHaveLink {
				var hasResetLink bool
				for _, link := range email.Links {
					if strings.Contains(link, "/reset") {
						hasResetLink = true
						break
					}
				}
				assert.True(t, hasResetLink)
			}
		})
	}
}
```

## Next Steps

- [Testing Multi-Email Scenarios](/client-go/testing/multi-email) - Handle multiple emails
- [CI/CD Integration](/client-go/testing/cicd) - Run tests in your pipeline
- [Working with Attachments](/client-go/guides/attachments) - Test emails with attachments
