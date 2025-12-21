# 08 - Testing

## Overview

The testing strategy covers:
1. **Unit tests**: Isolated component testing
2. **Integration tests**: End-to-end with live server
3. **Crypto tests**: Verification against known test vectors
4. **Concurrency tests**: Race condition detection

## Test Organization

```
client-go/
├── client_test.go           # Client unit tests
├── inbox_test.go            # Inbox unit tests
├── email_test.go            # Email unit tests
├── options_test.go          # Options tests
├── errors_test.go           # Error type tests
│
├── internal/
│   ├── api/
│   │   ├── client_test.go   # API client tests
│   │   └── retry_test.go    # Retry logic tests
│   │
│   ├── crypto/
│   │   ├── keypair_test.go  # Keypair generation tests
│   │   ├── decrypt_test.go  # Decryption tests
│   │   ├── verify_test.go   # Signature verification tests
│   │   ├── hkdf_test.go     # Key derivation tests
│   │   ├── aes_test.go      # AES-GCM tests
│   │   └── base64_test.go   # Base64url tests
│   │
│   └── delivery/
│       ├── sse_test.go      # SSE strategy tests
│       ├── polling_test.go  # Polling strategy tests
│       └── auto_test.go     # Auto strategy tests
│
├── authresults/
│   └── validate_test.go     # Auth validation tests
│
└── integration/
    ├── integration_test.go  # Main integration tests
    └── testdata/
        └── vectors/         # Test vectors
```

## Unit Tests

### Crypto Layer Tests

```go
// internal/crypto/keypair_test.go
package crypto

import (
    "testing"
)

func TestGenerateKeypair(t *testing.T) {
    kp, err := GenerateKeypair()
    if err != nil {
        t.Fatalf("GenerateKeypair() error = %v", err)
    }

    // Check key sizes
    if len(kp.PublicKey) != MLKEMPublicKeySize {
        t.Errorf("PublicKey size = %d, want %d", len(kp.PublicKey), MLKEMPublicKeySize)
    }

    if len(kp.SecretKey) != MLKEMSecretKeySize {
        t.Errorf("SecretKey size = %d, want %d", len(kp.SecretKey), MLKEMSecretKeySize)
    }

    // Check base64 encoding
    if kp.PublicKeyB64 == "" {
        t.Error("PublicKeyB64 is empty")
    }
}

func TestKeypairFromSecretKey(t *testing.T) {
    original, _ := GenerateKeypair()

    reconstructed, err := KeypairFromSecretKey(original.SecretKey)
    if err != nil {
        t.Fatalf("KeypairFromSecretKey() error = %v", err)
    }

    // Public key should match
    if !bytes.Equal(original.PublicKey, reconstructed.PublicKey) {
        t.Error("Reconstructed public key does not match original")
    }
}

func TestKeypairFromSecretKey_InvalidSize(t *testing.T) {
    _, err := KeypairFromSecretKey([]byte("too short"))
    if !errors.Is(err, ErrInvalidSecretKeySize) {
        t.Errorf("expected ErrInvalidSecretKeySize, got %v", err)
    }
}
```

### Base64url Tests

```go
// internal/crypto/base64_test.go
package crypto

import "testing"

func TestBase64URLRoundTrip(t *testing.T) {
    tests := []struct {
        name string
        data []byte
    }{
        {"empty", []byte{}},
        {"simple", []byte("hello")},
        {"binary", []byte{0x00, 0xff, 0x7f, 0x80}},
        {"url unsafe chars", []byte{0xfb, 0xf0}}, // Would produce + or / in standard base64
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            encoded := ToBase64URL(tt.data)
            decoded, err := FromBase64URL(encoded)
            if err != nil {
                t.Fatalf("FromBase64URL() error = %v", err)
            }
            if !bytes.Equal(decoded, tt.data) {
                t.Errorf("round trip failed: got %v, want %v", decoded, tt.data)
            }
        })
    }
}

func TestBase64URL_NoPadding(t *testing.T) {
    // Encoding should not include padding
    data := []byte("a") // Would normally have == padding
    encoded := ToBase64URL(data)
    if strings.Contains(encoded, "=") {
        t.Errorf("encoded string contains padding: %s", encoded)
    }
}
```

### Signature Verification Tests

```go
// internal/crypto/verify_test.go
package crypto

import (
    "testing"
)

func TestBuildTranscript(t *testing.T) {
    algs := AlgorithmSuite{
        KEM:  "ML-KEM-768",
        Sig:  "ML-DSA-65",
        AEAD: "AES-256-GCM",
        KDF:  "HKDF-SHA-512",
    }

    transcript := buildTranscript(
        1,                    // version
        algs,
        []byte("ct_kem"),
        []byte("nonce"),
        []byte("aad"),
        []byte("ciphertext"),
        []byte("server_pk"),
    )

    // Verify structure
    if transcript[0] != 1 {
        t.Errorf("first byte (version) = %d, want 1", transcript[0])
    }

    // Check ciphersuite string is present
    expected := "ML-KEM-768:ML-DSA-65:AES-256-GCM:HKDF-SHA-512"
    if !bytes.Contains(transcript, []byte(expected)) {
        t.Error("transcript does not contain ciphersuite string")
    }
}

func TestVerifySignature_InvalidSignature(t *testing.T) {
    payload := &EncryptedPayload{
        V:           1,
        Sig:         ToBase64URL([]byte("invalid signature")),
        ServerSigPk: ToBase64URL(make([]byte, MLDSAPublicKeySize)),
        // ... other fields
    }

    err := VerifySignature(payload)
    if err == nil {
        t.Error("expected error for invalid signature")
    }
}
```

### HTTP Client Tests

```go
// internal/api/client_test.go
package api

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestClient_Retry(t *testing.T) {
    attempts := 0

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        attempts++
        if attempts < 3 {
            w.WriteHeader(http.StatusServiceUnavailable)
            return
        }
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"ok": true}`))
    }))
    defer server.Close()

    client := NewClient(Config{
        BaseURL:    server.URL,
        APIKey:     "test-key",
        MaxRetries: 3,
        RetryDelay: time.Millisecond,
    })

    var result struct{ OK bool }
    err := client.Do(context.Background(), "GET", "/test", nil, &result)

    if err != nil {
        t.Fatalf("Do() error = %v", err)
    }
    if attempts != 3 {
        t.Errorf("attempts = %d, want 3", attempts)
    }
}

func TestClient_NoRetryOn4xx(t *testing.T) {
    attempts := 0

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        attempts++
        w.WriteHeader(http.StatusBadRequest)
    }))
    defer server.Close()

    client := NewClient(Config{
        BaseURL:    server.URL,
        APIKey:     "test-key",
        MaxRetries: 3,
    })

    err := client.Do(context.Background(), "GET", "/test", nil, nil)

    if err == nil {
        t.Fatal("expected error for 400 response")
    }
    if attempts != 1 {
        t.Errorf("attempts = %d, want 1 (no retry on 4xx)", attempts)
    }
}
```

### Delivery Strategy Tests

```go
// internal/delivery/polling_test.go
package delivery

import (
    "testing"
    "time"
)

func TestPollingStrategy_Backoff(t *testing.T) {
    inbox := &polledInbox{
        interval: PollingInitialInterval,
    }

    // First backoff
    inbox.interval = time.Duration(float64(inbox.interval) * PollingBackoffMultiplier)
    expected := time.Duration(float64(PollingInitialInterval) * PollingBackoffMultiplier)
    if inbox.interval != expected {
        t.Errorf("interval = %v, want %v", inbox.interval, expected)
    }

    // Should cap at max
    for i := 0; i < 20; i++ {
        inbox.interval = time.Duration(float64(inbox.interval) * PollingBackoffMultiplier)
        if inbox.interval > PollingMaxBackoff {
            inbox.interval = PollingMaxBackoff
        }
    }
    if inbox.interval != PollingMaxBackoff {
        t.Errorf("interval = %v, want max %v", inbox.interval, PollingMaxBackoff)
    }
}
```

## Integration Tests

### Environment Configuration

Integration tests load configuration from `.env` files using `godotenv`:

```bash
# Add to go.mod
go get github.com/joho/godotenv
```

**.env.example** (commit this):
```env
VAULTSANDBOX_API_KEY=your-api-key-here
VAULTSANDBOX_BASE_URL=https://smtp.vaultsandbox.com
```

**.env** (do not commit - add to .gitignore):
```env
VAULTSANDBOX_API_KEY=actual-secret-key
VAULTSANDBOX_BASE_URL=https://smtp.vaultsandbox.com
```

**.gitignore**:
```
.env
!.env.example
```

### Test Setup

```go
// integration/integration_test.go
//go:build integration

package integration

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/joho/godotenv"
    "github.com/vaultsandbox/client-go"
)

var (
    apiKey  string
    baseURL string
)

func TestMain(m *testing.M) {
    // Load .env file if it exists (won't error if missing)
    _ = godotenv.Load("../.env") // Load from project root

    apiKey = os.Getenv("VAULTSANDBOX_API_KEY")
    baseURL = os.Getenv("VAULTSANDBOX_BASE_URL")

    if apiKey == "" {
        // Skip integration tests if no API key
        os.Exit(0)
    }
    os.Exit(m.Run())
}

func newClient(t *testing.T) *vaultsandbox.Client {
    t.Helper()

    opts := []vaultsandbox.Option{}
    if baseURL != "" {
        opts = append(opts, vaultsandbox.WithBaseURL(baseURL))
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

    // Delete inbox
    if err := inbox.Delete(ctx); err != nil {
        t.Errorf("Delete() error = %v", err)
    }
}

func TestIntegration_ExportImport(t *testing.T) {
    client := newClient(t)
    ctx := context.Background()

    // Create and export
    inbox, _ := client.CreateInbox(ctx)
    exported := inbox.Export()

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

    // Clean up
    inbox.Delete(ctx)
}

func TestIntegration_WaitForEmail(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping in short mode")
    }

    client := newClient(t)
    ctx := context.Background()

    inbox, _ := client.CreateInbox(ctx)
    defer inbox.Delete(ctx)

    t.Logf("Send test email to: %s", inbox.EmailAddress())

    // This test requires manual email sending or a test harness
    // that can send emails to the created inbox

    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(2*time.Minute),
    )
    if err != nil {
        t.Fatalf("WaitForEmail() error = %v", err)
    }

    t.Logf("Received email: %s", email.Subject)
}
```

## Test Fixtures

### Test Vectors

```go
// integration/testdata/vectors/crypto_vectors.go
package vectors

// DecryptionVector contains known-good inputs and outputs
type DecryptionVector struct {
    Name           string
    SecretKeyB64   string
    EncryptedJSON  string // Full EncryptedPayload as JSON
    ExpectedPlain  string
}

var DecryptionVectors = []DecryptionVector{
    {
        Name:         "basic_email_metadata",
        SecretKeyB64: "...",
        EncryptedJSON: `{
            "v": 1,
            "algs": {...},
            ...
        }`,
        ExpectedPlain: `{"from":"sender@example.com","to":["recipient@test.com"],"subject":"Test"}`,
    },
}
```

## Mocking

### Mock API Client

```go
// internal/api/mock_client.go
package api

type MockClient struct {
    CreateInboxFunc  func(ctx context.Context, req CreateInboxRequest) (*CreateInboxResponse, error)
    GetEmailsFunc    func(ctx context.Context, email string) ([]RawEmail, error)
    // ... other methods
}

func (m *MockClient) CreateInbox(ctx context.Context, req CreateInboxRequest) (*CreateInboxResponse, error) {
    if m.CreateInboxFunc != nil {
        return m.CreateInboxFunc(ctx, req)
    }
    return nil, errors.New("not implemented")
}
```

## Running Tests

```bash
# Unit tests
go test ./...

# With race detection
go test -race ./...

# Integration tests (requires API key)
VAULTSANDBOX_API_KEY=xxx go test -tags=integration ./integration/

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Benchmark crypto operations
go test -bench=. ./internal/crypto/
```

## CI Configuration

```yaml
# .github/workflows/test.yml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Unit Tests
        run: go test -race -coverprofile=coverage.out ./...

      - name: Upload Coverage
        uses: codecov/codecov-action@v4
        with:
          files: coverage.out

  integration:
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Integration Tests
        env:
          VAULTSANDBOX_API_KEY: ${{ secrets.VAULTSANDBOX_API_KEY }}
        run: go test -tags=integration ./integration/
```

## Coverage Goals

| Package | Target Coverage |
|---------|-----------------|
| `internal/crypto` | 95%+ |
| `internal/api` | 85%+ |
| `internal/delivery` | 80%+ |
| Root package | 80%+ |
| `authresults` | 90%+ |
