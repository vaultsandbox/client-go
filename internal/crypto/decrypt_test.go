package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"testing"

	"github.com/cloudflare/circl/kem/mlkem/mlkem768"
)

func TestDeriveKey(t *testing.T) {
	t.Parallel()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		salt   []byte
		info   []byte
		length int
	}{
		{"basic 32 bytes", make([]byte, 32), []byte("info"), 32},
		{"empty salt", nil, []byte("info"), 32},
		{"empty info", make([]byte, 32), nil, 32},
		{"16 byte key", make([]byte, 32), []byte("info"), 16},
		{"64 byte key", make([]byte, 32), []byte("info"), 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := DeriveKey(secret, tt.salt, tt.info, tt.length)
			if err != nil {
				t.Fatalf("DeriveKey() error = %v", err)
			}

			if len(key) != tt.length {
				t.Errorf("key length = %d, want %d", len(key), tt.length)
			}
		})
	}
}

func TestDeriveKey_Deterministic(t *testing.T) {
	t.Parallel()
	secret := []byte("test secret key for derivation")
	salt := []byte("test salt value")
	info := []byte("test info value")

	key1, err := DeriveKey(secret, salt, info, 32)
	if err != nil {
		t.Fatal(err)
	}

	key2, err := DeriveKey(secret, salt, info, 32)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(key1, key2) {
		t.Error("DeriveKey not deterministic: same inputs produced different outputs")
	}
}

func TestDeriveKey_ExceedsMaxLength(t *testing.T) {
	t.Parallel()
	secret := []byte("test secret")
	salt := []byte("test salt")
	info := []byte("test info")

	// HKDF-SHA-512 can produce at most 255 * 64 = 16320 bytes
	// Requesting more should fail
	_, err := DeriveKey(secret, salt, info, 16321)
	if err == nil {
		t.Error("expected error when requesting more than HKDF max output")
	}
}

func TestDeriveKey_DifferentInputs(t *testing.T) {
	t.Parallel()
	secret := []byte("test secret key for derivation")
	salt := []byte("test salt value")
	info := []byte("test info value")

	baseKey, _ := DeriveKey(secret, salt, info, 32)

	t.Run("different secret", func(t *testing.T) {
		key, _ := DeriveKey([]byte("different secret"), salt, info, 32)
		if bytes.Equal(key, baseKey) {
			t.Error("different secret produced same key")
		}
	})

	t.Run("different salt", func(t *testing.T) {
		key, _ := DeriveKey(secret, []byte("different salt"), info, 32)
		if bytes.Equal(key, baseKey) {
			t.Error("different salt produced same key")
		}
	})

	t.Run("different info", func(t *testing.T) {
		key, _ := DeriveKey(secret, salt, []byte("different info"), 32)
		if bytes.Equal(key, baseKey) {
			t.Error("different info produced same key")
		}
	})
}

func TestDecryptedEmail_Fields(t *testing.T) {
	t.Parallel()
	// Test that DecryptedEmail struct can be used correctly
	email := DecryptedEmail{
		ID:      "test-id",
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test Subject",
		Text:    "Plain text body",
		HTML:    "<p>HTML body</p>",
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		IsRead: false,
	}

	if email.ID != "test-id" {
		t.Errorf("ID = %s, want test-id", email.ID)
	}
	if email.From != "sender@example.com" {
		t.Errorf("From = %s, want sender@example.com", email.From)
	}
	if len(email.To) != 1 || email.To[0] != "recipient@example.com" {
		t.Errorf("To = %v, want [recipient@example.com]", email.To)
	}
}

func TestDecryptedAttachment_Fields(t *testing.T) {
	t.Parallel()
	attachment := DecryptedAttachment{
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
	if attachment.Size != 1024 {
		t.Errorf("Size = %d, want 1024", attachment.Size)
	}
}

func TestBase64Bytes_UnmarshalJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected []byte
		wantErr  bool
	}{
		{
			name:     "standard base64 string",
			input:    `"SGVsbG8gV29ybGQh"`, // "Hello World!" in base64
			expected: []byte("Hello World!"),
			wantErr:  false,
		},
		{
			name:     "url-safe base64 string",
			input:    `"SGVsbG8tV29ybGRf"`, // base64url encoded
			expected: []byte("Hello-World_"),
			wantErr:  false,
		},
		{
			name:     "empty string",
			input:    `""`,
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "null value",
			input:    `null`,
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "base64 with padding",
			input:    `"dGVzdA=="`, // "test" with padding
			expected: []byte("test"),
			wantErr:  false,
		},
		{
			name:     "url-safe base64 with special chars",
			input:    `"PDw_Pj4-"`, // "<<?>>>" encoded with URL-safe base64 (has - and _)
			expected: []byte("<<?>>>"),
			wantErr:  false,
		},
		{
			name:     "invalid base64 both standard and url-safe",
			input:    `"!!!totally-invalid-base64!!!"`,
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "raw json bytes (unquoted)",
			input:    `123`,
			expected: []byte("123"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Base64Bytes
			err := b.UnmarshalJSON([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(b, tt.expected) {
				t.Errorf("UnmarshalJSON() = %v, want %v", b, tt.expected)
			}
		})
	}
}

func TestDecryptedAttachment_JSONUnmarshal(t *testing.T) {
	t.Parallel()
	// Test JSON unmarshaling with camelCase field names (as sent by server)
	jsonData := `{
		"filename": "test.pdf",
		"contentType": "application/pdf",
		"size": 1024,
		"contentId": "cid123",
		"contentDisposition": "attachment",
		"content": "SGVsbG8gV29ybGQh",
		"checksum": "abc123"
	}`

	var attachment DecryptedAttachment
	if err := json.Unmarshal([]byte(jsonData), &attachment); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if attachment.Filename != "test.pdf" {
		t.Errorf("Filename = %s, want test.pdf", attachment.Filename)
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
	if string(attachment.Content) != "Hello World!" {
		t.Errorf("Content = %s, want Hello World!", string(attachment.Content))
	}
	if attachment.Checksum != "abc123" {
		t.Errorf("Checksum = %s, want abc123", attachment.Checksum)
	}
}

func TestDecryptedAttachment_JSONUnmarshal_OptionalFields(t *testing.T) {
	t.Parallel()
	// Test JSON unmarshaling with optional fields omitted
	jsonData := `{
		"filename": "test.txt",
		"contentType": "text/plain",
		"size": 100
	}`

	var attachment DecryptedAttachment
	if err := json.Unmarshal([]byte(jsonData), &attachment); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if attachment.Filename != "test.txt" {
		t.Errorf("Filename = %s, want test.txt", attachment.Filename)
	}
	if attachment.ContentID != "" {
		t.Errorf("ContentID = %s, want empty", attachment.ContentID)
	}
	if attachment.Content != nil {
		t.Errorf("Content = %v, want nil", attachment.Content)
	}
}


func TestDecrypt_Success(t *testing.T) {
	t.Parallel()
	// Generate a keypair for testing
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Prepare the plaintext
	plaintext := []byte(`{"from":"test@example.com","subject":"Test"}`)

	// Create an encrypted payload manually using the crypto primitives
	// 1. Encapsulate to get shared secret and KEM ciphertext
	var pubKey mlkem768.PublicKey
	pubKey.Unpack(kp.PublicKey)

	ctKem := make([]byte, MLKEMCiphertextSize)
	sharedSecret := make([]byte, MLKEMSharedKeySize)
	pubKey.EncapsulateTo(ctKem, sharedSecret, nil)

	// 2. Derive AES key using the same method as deriveKey
	aad := []byte("test-aad")
	aesKey := deriveKey(sharedSecret, aad, ctKem)

	// 3. Encrypt with AES-GCM
	nonce := make([]byte, AESNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}

	block, _ := aes.NewCipher(aesKey)
	gcm, _ := cipher.NewGCM(block)
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)

	// 4. Create the encrypted payload
	payload := &EncryptedPayload{
		V:          1,
		CtKem:      ToBase64URL(ctKem),
		Nonce:      ToBase64URL(nonce),
		AAD:        ToBase64URL(aad),
		Ciphertext: ToBase64URL(ciphertext),
	}

	// 5. Decrypt
	result, err := Decrypt(payload, kp)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(result, plaintext) {
		t.Errorf("Decrypt() = %s, want %s", string(result), string(plaintext))
	}
}

func TestDecrypt_InvalidPrivateKey(t *testing.T) {
	t.Parallel()
	// Create a keypair with invalid secret key
	invalidKp := &Keypair{
		SecretKey: make([]byte, 10), // Wrong size - will fail Unpack
		PublicKey: make([]byte, MLKEMPublicKeySize),
	}

	payload := &EncryptedPayload{
		V:          1,
		CtKem:      ToBase64URL(make([]byte, MLKEMCiphertextSize)),
		Nonce:      ToBase64URL(make([]byte, AESNonceSize)),
		AAD:        ToBase64URL([]byte("aad")),
		Ciphertext: ToBase64URL(make([]byte, 100)),
	}

	_, err := Decrypt(payload, invalidKp)
	if err == nil {
		t.Error("expected error for invalid private key")
	}
}

func TestDecrypt_DecryptionFailed(t *testing.T) {
	t.Parallel()
	// Generate a valid keypair
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create a payload with mismatched ciphertext (wrong key or tampered data)
	payload := &EncryptedPayload{
		V:          1,
		CtKem:      ToBase64URL(make([]byte, MLKEMCiphertextSize)),
		Nonce:      ToBase64URL(make([]byte, AESNonceSize)),
		AAD:        ToBase64URL([]byte("aad")),
		Ciphertext: ToBase64URL(make([]byte, 100)), // Invalid ciphertext
	}

	_, err = Decrypt(payload, kp)
	if err == nil {
		t.Error("expected decryption error for invalid ciphertext")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	t.Parallel()
	kp, _ := GenerateKeypair()

	tests := []struct {
		name    string
		payload *EncryptedPayload
	}{
		{
			name: "invalid ct_kem",
			payload: &EncryptedPayload{
				V:          1,
				CtKem:      "!!!invalid!!!",
				Nonce:      ToBase64URL(make([]byte, AESNonceSize)),
				AAD:        ToBase64URL([]byte("aad")),
				Ciphertext: ToBase64URL(make([]byte, 100)),
			},
		},
		{
			name: "invalid nonce",
			payload: &EncryptedPayload{
				V:          1,
				CtKem:      ToBase64URL(make([]byte, MLKEMCiphertextSize)),
				Nonce:      "!!!invalid!!!",
				AAD:        ToBase64URL([]byte("aad")),
				Ciphertext: ToBase64URL(make([]byte, 100)),
			},
		},
		{
			name: "invalid aad",
			payload: &EncryptedPayload{
				V:          1,
				CtKem:      ToBase64URL(make([]byte, MLKEMCiphertextSize)),
				Nonce:      ToBase64URL(make([]byte, AESNonceSize)),
				AAD:        "!!!invalid!!!",
				Ciphertext: ToBase64URL(make([]byte, 100)),
			},
		},
		{
			name: "invalid ciphertext",
			payload: &EncryptedPayload{
				V:          1,
				CtKem:      ToBase64URL(make([]byte, MLKEMCiphertextSize)),
				Nonce:      ToBase64URL(make([]byte, AESNonceSize)),
				AAD:        ToBase64URL([]byte("aad")),
				Ciphertext: "!!!invalid!!!",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.payload, kp)
			if err == nil {
				t.Error("expected error for invalid base64")
			}
		})
	}
}

func BenchmarkDeriveKey(b *testing.B) {
	secret := make([]byte, 32)
	salt := make([]byte, 32)
	info := []byte("benchmark info")

	rand.Read(secret)
	rand.Read(salt)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DeriveKey(secret, salt, info, 32)
	}
}
