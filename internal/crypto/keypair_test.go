package crypto

import (
	"bytes"
	"errors"
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

	// Check base64 encoding is present
	if kp.PublicKeyB64 == "" {
		t.Error("PublicKeyB64 is empty")
	}

	// Verify base64 decodes back to public key
	decoded, err := FromBase64URL(kp.PublicKeyB64)
	if err != nil {
		t.Fatalf("FromBase64URL() error = %v", err)
	}
	if !bytes.Equal(decoded, kp.PublicKey) {
		t.Error("PublicKeyB64 does not decode to PublicKey")
	}
}

func TestGenerateKeypair_Uniqueness(t *testing.T) {
	kp1, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	kp2, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	if bytes.Equal(kp1.PublicKey, kp2.PublicKey) {
		t.Error("Generated keypairs have identical public keys")
	}

	if bytes.Equal(kp1.SecretKey, kp2.SecretKey) {
		t.Error("Generated keypairs have identical secret keys")
	}
}

func TestKeypairFromSecretKey(t *testing.T) {
	original, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	reconstructed, err := KeypairFromSecretKey(original.SecretKey)
	if err != nil {
		t.Fatalf("KeypairFromSecretKey() error = %v", err)
	}

	// Public key should match
	if !bytes.Equal(original.PublicKey, reconstructed.PublicKey) {
		t.Error("Reconstructed public key does not match original")
	}

	// Secret key should match
	if !bytes.Equal(original.SecretKey, reconstructed.SecretKey) {
		t.Error("Reconstructed secret key does not match original")
	}

	// Base64 should match
	if original.PublicKeyB64 != reconstructed.PublicKeyB64 {
		t.Errorf("PublicKeyB64 mismatch: got %s, want %s", reconstructed.PublicKeyB64, original.PublicKeyB64)
	}
}

func TestKeypairFromSecretKey_InvalidSize(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
	}{
		{"empty", []byte{}},
		{"too short", []byte("too short")},
		{"one byte short", make([]byte, MLKEMSecretKeySize-1)},
		{"one byte long", make([]byte, MLKEMSecretKeySize+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := KeypairFromSecretKey(tt.key)
			if !errors.Is(err, ErrInvalidSecretKeySize) {
				t.Errorf("expected ErrInvalidSecretKeySize, got %v", err)
			}
		})
	}
}

func TestNewKeypairFromBytes(t *testing.T) {
	original, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	kp, err := NewKeypairFromBytes(original.SecretKey, original.PublicKey)
	if err != nil {
		t.Fatalf("NewKeypairFromBytes() error = %v", err)
	}

	if !bytes.Equal(kp.PublicKey, original.PublicKey) {
		t.Error("PublicKey mismatch")
	}

	if !bytes.Equal(kp.SecretKey, original.SecretKey) {
		t.Error("SecretKey mismatch")
	}
}

func TestNewKeypairFromBytes_InvalidSecretKeySize(t *testing.T) {
	_, err := NewKeypairFromBytes([]byte("short"), make([]byte, MLKEMPublicKeySize))
	if !errors.Is(err, ErrInvalidSecretKeySize) {
		t.Errorf("expected ErrInvalidSecretKeySize, got %v", err)
	}
}

func TestNewKeypairFromBytes_InvalidPublicKeySize(t *testing.T) {
	_, err := NewKeypairFromBytes(make([]byte, MLKEMSecretKeySize), []byte("short"))
	if !errors.Is(err, ErrInvalidPublicKeySize) {
		t.Errorf("expected ErrInvalidPublicKeySize, got %v", err)
	}
}

func TestKeypair_Decapsulate(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	// We can't easily test decapsulation without encapsulation,
	// but we can test error cases
	t.Run("invalid ciphertext size", func(t *testing.T) {
		_, err := kp.Decapsulate([]byte("too short"))
		if !errors.Is(err, ErrInvalidCiphertextSize) {
			t.Errorf("expected ErrInvalidCiphertextSize, got %v", err)
		}
	})

	t.Run("ciphertext one byte short", func(t *testing.T) {
		_, err := kp.Decapsulate(make([]byte, MLKEMCiphertextSize-1))
		if !errors.Is(err, ErrInvalidCiphertextSize) {
			t.Errorf("expected ErrInvalidCiphertextSize, got %v", err)
		}
	})
}

func TestPublicKeyOffset(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	// Verify the public key is embedded at the correct offset in secret key
	embeddedPK := kp.SecretKey[PublicKeyOffset : PublicKeyOffset+MLKEMPublicKeySize]
	if !bytes.Equal(embeddedPK, kp.PublicKey) {
		t.Error("Public key is not embedded at expected offset in secret key")
	}
}

func TestValidateKeypair(t *testing.T) {
	t.Run("valid keypair", func(t *testing.T) {
		kp, err := GenerateKeypair()
		if err != nil {
			t.Fatalf("GenerateKeypair() error = %v", err)
		}
		if !ValidateKeypair(kp) {
			t.Error("ValidateKeypair() returned false for valid keypair")
		}
	})

	t.Run("nil keypair", func(t *testing.T) {
		if ValidateKeypair(nil) {
			t.Error("ValidateKeypair() returned true for nil keypair")
		}
	})

	t.Run("nil public key", func(t *testing.T) {
		kp := &Keypair{
			SecretKey:    make([]byte, MLKEMSecretKeySize),
			PublicKeyB64: "test",
		}
		if ValidateKeypair(kp) {
			t.Error("ValidateKeypair() returned true for nil public key")
		}
	})

	t.Run("nil secret key", func(t *testing.T) {
		kp := &Keypair{
			PublicKey:    make([]byte, MLKEMPublicKeySize),
			PublicKeyB64: "test",
		}
		if ValidateKeypair(kp) {
			t.Error("ValidateKeypair() returned true for nil secret key")
		}
	})

	t.Run("empty public key b64", func(t *testing.T) {
		kp := &Keypair{
			PublicKey: make([]byte, MLKEMPublicKeySize),
			SecretKey: make([]byte, MLKEMSecretKeySize),
		}
		if ValidateKeypair(kp) {
			t.Error("ValidateKeypair() returned true for empty PublicKeyB64")
		}
	})

	t.Run("wrong public key size", func(t *testing.T) {
		kp := &Keypair{
			PublicKey:    make([]byte, 100),
			SecretKey:    make([]byte, MLKEMSecretKeySize),
			PublicKeyB64: "test",
		}
		if ValidateKeypair(kp) {
			t.Error("ValidateKeypair() returned true for wrong public key size")
		}
	})

	t.Run("wrong secret key size", func(t *testing.T) {
		kp := &Keypair{
			PublicKey:    make([]byte, MLKEMPublicKeySize),
			SecretKey:    make([]byte, 100),
			PublicKeyB64: "test",
		}
		if ValidateKeypair(kp) {
			t.Error("ValidateKeypair() returned true for wrong secret key size")
		}
	})

	t.Run("mismatched public key b64", func(t *testing.T) {
		kp, err := GenerateKeypair()
		if err != nil {
			t.Fatalf("GenerateKeypair() error = %v", err)
		}
		kp.PublicKeyB64 = ToBase64URL(make([]byte, MLKEMPublicKeySize)) // Different key
		if ValidateKeypair(kp) {
			t.Error("ValidateKeypair() returned true for mismatched PublicKeyB64")
		}
	})
}

func TestDerivePublicKeyFromSecret(t *testing.T) {
	t.Run("valid secret key", func(t *testing.T) {
		kp, err := GenerateKeypair()
		if err != nil {
			t.Fatalf("GenerateKeypair() error = %v", err)
		}

		derivedPK, err := DerivePublicKeyFromSecret(kp.SecretKey)
		if err != nil {
			t.Fatalf("DerivePublicKeyFromSecret() error = %v", err)
		}

		if !bytes.Equal(derivedPK, kp.PublicKey) {
			t.Error("Derived public key does not match original")
		}
	})

	t.Run("invalid secret key size", func(t *testing.T) {
		_, err := DerivePublicKeyFromSecret([]byte("too short"))
		if !errors.Is(err, ErrInvalidSecretKeySize) {
			t.Errorf("expected ErrInvalidSecretKeySize, got %v", err)
		}
	})

	t.Run("empty secret key", func(t *testing.T) {
		_, err := DerivePublicKeyFromSecret([]byte{})
		if !errors.Is(err, ErrInvalidSecretKeySize) {
			t.Errorf("expected ErrInvalidSecretKeySize, got %v", err)
		}
	})
}

func BenchmarkGenerateKeypair(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GenerateKeypair()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkKeypairFromSecretKey(b *testing.B) {
	kp, _ := GenerateKeypair()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := KeypairFromSecretKey(kp.SecretKey)
		if err != nil {
			b.Fatal(err)
		}
	}
}
