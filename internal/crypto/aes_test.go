package crypto

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"testing"
)

func TestEncryptAES_DecryptAES_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"simple", []byte("hello world")},
		{"json", []byte(`{"foo": "bar", "num": 123}`)},
		{"binary", []byte{0x00, 0xff, 0x7f, 0x80}},
		{"large", make([]byte, 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, AESKeySize)
			if _, err := rand.Read(key); err != nil {
				t.Fatal(err)
			}

			nonce := make([]byte, AESNonceSize)
			if _, err := rand.Read(nonce); err != nil {
				t.Fatal(err)
			}

			ciphertext, err := EncryptAES(key, tt.plaintext, nonce)
			if err != nil {
				t.Fatalf("EncryptAES() error = %v", err)
			}

			// Ciphertext should be nonce + ciphertext + tag
			expectedLen := AESNonceSize + len(tt.plaintext) + AESTagSize
			if len(ciphertext) != expectedLen {
				t.Errorf("ciphertext length = %d, want %d", len(ciphertext), expectedLen)
			}

			// First 12 bytes should be the nonce
			if !bytes.Equal(ciphertext[:AESNonceSize], nonce) {
				t.Error("ciphertext doesn't start with nonce")
			}

			decrypted, err := DecryptAES(key, ciphertext)
			if err != nil {
				t.Fatalf("DecryptAES() error = %v", err)
			}

			if !bytes.Equal(decrypted, tt.plaintext) {
				t.Errorf("decrypted = %v, want %v", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptAES_InvalidKeySize(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
	}{
		{"empty", 0},
		{"too short", 16},
		{"too long", 64},
	}

	nonce := make([]byte, AESNonceSize)
	plaintext := []byte("test")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keySize)
			_, err := EncryptAES(key, plaintext, nonce)
			if !errors.Is(err, ErrInvalidKeySize) {
				t.Errorf("expected ErrInvalidKeySize, got %v", err)
			}
		})
	}
}

func TestEncryptAES_InvalidNonceSize(t *testing.T) {
	tests := []struct {
		name      string
		nonceSize int
	}{
		{"empty", 0},
		{"too short", 8},
		{"too long", 16},
	}

	key := make([]byte, AESKeySize)
	plaintext := []byte("test")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nonce := make([]byte, tt.nonceSize)
			_, err := EncryptAES(key, plaintext, nonce)
			if !errors.Is(err, ErrInvalidNonceSize) {
				t.Errorf("expected ErrInvalidNonceSize, got %v", err)
			}
		})
	}
}

func TestDecryptAES_InvalidKeySize(t *testing.T) {
	key := make([]byte, 16) // Wrong size
	ciphertext := make([]byte, AESNonceSize+AESTagSize+10)

	_, err := DecryptAES(key, ciphertext)
	if !errors.Is(err, ErrInvalidKeySize) {
		t.Errorf("expected ErrInvalidKeySize, got %v", err)
	}
}

func TestDecryptAES_CiphertextTooShort(t *testing.T) {
	key := make([]byte, AESKeySize)

	tests := []struct {
		name   string
		length int
	}{
		{"empty", 0},
		{"only nonce", AESNonceSize},
		{"nonce plus partial tag", AESNonceSize + AESTagSize - 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext := make([]byte, tt.length)
			_, err := DecryptAES(key, ciphertext)
			if err == nil {
				t.Error("expected error for short ciphertext")
			}
		})
	}
}

func TestDecryptAES_TamperedCiphertext(t *testing.T) {
	key := make([]byte, AESKeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	nonce := make([]byte, AESNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("sensitive data")
	ciphertext, err := EncryptAES(key, plaintext, nonce)
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with the ciphertext (flip a bit in the middle)
	ciphertext[len(ciphertext)/2] ^= 0xff

	_, err = DecryptAES(key, ciphertext)
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("expected ErrDecryptionFailed, got %v", err)
	}
}

func TestDecryptAES_WrongKey(t *testing.T) {
	key1 := make([]byte, AESKeySize)
	key2 := make([]byte, AESKeySize)
	if _, err := rand.Read(key1); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(key2); err != nil {
		t.Fatal(err)
	}

	nonce := make([]byte, AESNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("sensitive data")
	ciphertext, err := EncryptAES(key1, plaintext, nonce)
	if err != nil {
		t.Fatal(err)
	}

	// Try to decrypt with wrong key
	_, err = DecryptAES(key2, ciphertext)
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("expected ErrDecryptionFailed, got %v", err)
	}
}

func TestDecryptAESGCM_WithAAD(t *testing.T) {
	key := make([]byte, AESKeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	nonce := make([]byte, AESNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}

	aad := []byte("additional authenticated data")
	plaintext := []byte("secret message")

	// We need to encrypt with AAD to test decryption with AAD
	// Since EncryptAES doesn't support AAD, we test error cases
	t.Run("invalid key size", func(t *testing.T) {
		_, err := decryptAESGCM(make([]byte, 16), nonce, aad, plaintext)
		if !errors.Is(err, ErrInvalidKeySize) {
			t.Errorf("expected ErrInvalidKeySize, got %v", err)
		}
	})

	t.Run("invalid nonce size", func(t *testing.T) {
		_, err := decryptAESGCM(key, make([]byte, 8), aad, plaintext)
		if !errors.Is(err, ErrInvalidNonceSize) {
			t.Errorf("expected ErrInvalidNonceSize, got %v", err)
		}
	})
}

func BenchmarkEncryptAES(b *testing.B) {
	key := make([]byte, AESKeySize)
	nonce := make([]byte, AESNonceSize)
	plaintext := make([]byte, 1000)

	rand.Read(key)
	rand.Read(nonce)
	rand.Read(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EncryptAES(key, plaintext, nonce)
	}
}

func BenchmarkDecryptAES(b *testing.B) {
	key := make([]byte, AESKeySize)
	nonce := make([]byte, AESNonceSize)
	plaintext := make([]byte, 1000)

	rand.Read(key)
	rand.Read(nonce)
	rand.Read(plaintext)

	ciphertext, _ := EncryptAES(key, plaintext, nonce)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecryptAES(key, ciphertext)
	}
}

// Example_encryptDecrypt demonstrates encrypting and decrypting data with AES-256-GCM.
func Example_encryptDecrypt() {
	// Generate a random 256-bit key.
	key := make([]byte, AESKeySize)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}

	// Generate a random 96-bit nonce.
	// IMPORTANT: Never reuse a nonce with the same key.
	nonce := make([]byte, AESNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}

	// Encrypt the plaintext.
	plaintext := []byte("Hello, World!")
	ciphertext, err := EncryptAES(key, plaintext, nonce)
	if err != nil {
		panic(err)
	}

	// Decrypt the ciphertext.
	decrypted, err := DecryptAES(key, ciphertext)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(decrypted))
	// Output: Hello, World!
}
