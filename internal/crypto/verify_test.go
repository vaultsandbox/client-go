package crypto

import (
	"bytes"
	"errors"
	"testing"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
)

func TestBuildTranscript(t *testing.T) {
	algs := AlgorithmSuite{
		KEM:  "ML-KEM-768",
		Sig:  "ML-DSA-65",
		AEAD: "AES-256-GCM",
		KDF:  "HKDF-SHA-512",
	}

	transcript := buildTranscript(
		1,                   // version
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

	// Check context is present
	if !bytes.Contains(transcript, []byte(HKDFContext)) {
		t.Error("transcript does not contain HKDF context")
	}

	// Check all components are present
	if !bytes.Contains(transcript, []byte("ct_kem")) {
		t.Error("transcript does not contain ct_kem")
	}
	if !bytes.Contains(transcript, []byte("nonce")) {
		t.Error("transcript does not contain nonce")
	}
	if !bytes.Contains(transcript, []byte("aad")) {
		t.Error("transcript does not contain aad")
	}
	if !bytes.Contains(transcript, []byte("ciphertext")) {
		t.Error("transcript does not contain ciphertext")
	}
	if !bytes.Contains(transcript, []byte("server_pk")) {
		t.Error("transcript does not contain server_pk")
	}
}

func TestBuildTranscript_DifferentVersions(t *testing.T) {
	algs := AlgorithmSuite{
		KEM:  "ML-KEM-768",
		Sig:  "ML-DSA-65",
		AEAD: "AES-256-GCM",
		KDF:  "HKDF-SHA-512",
	}

	tests := []struct {
		version int
	}{
		{0},
		{1},
		{2},
		{255},
	}

	for _, tt := range tests {
		t.Run("version_"+string(rune('0'+tt.version)), func(t *testing.T) {
			transcript := buildTranscript(tt.version, algs, nil, nil, nil, nil, nil)
			if transcript[0] != byte(tt.version) {
				t.Errorf("version byte = %d, want %d", transcript[0], tt.version)
			}
		})
	}
}

func TestVerify_ValidSignature(t *testing.T) {
	// Generate a test keypair
	pub, priv, err := mldsa65.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	pubBytes, err := pub.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("test message to sign")

	sig := make([]byte, mldsa65.SignatureSize)
	mldsa65.SignTo(priv, message, nil, false, sig)

	err = Verify(pubBytes, message, sig)
	if err != nil {
		t.Errorf("Verify() error = %v", err)
	}
}

func TestVerify_InvalidSignature(t *testing.T) {
	pub, _, err := mldsa65.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	pubBytes, err := pub.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("test message")
	invalidSig := make([]byte, MLDSASignatureSize)

	err = Verify(pubBytes, message, invalidSig)
	if !errors.Is(err, ErrSignatureVerificationFailed) {
		t.Errorf("expected ErrSignatureVerificationFailed, got %v", err)
	}
}

func TestVerify_TamperedMessage(t *testing.T) {
	pub, priv, err := mldsa65.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	pubBytes, err := pub.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	originalMessage := []byte("original message")
	sig := make([]byte, mldsa65.SignatureSize)
	mldsa65.SignTo(priv, originalMessage, nil, false, sig)

	tamperedMessage := []byte("tampered message")
	err = Verify(pubBytes, tamperedMessage, sig)
	if !errors.Is(err, ErrSignatureVerificationFailed) {
		t.Errorf("expected ErrSignatureVerificationFailed, got %v", err)
	}
}

func TestVerify_InvalidPublicKey(t *testing.T) {
	message := []byte("test message")
	sig := make([]byte, MLDSASignatureSize)
	invalidPubKey := []byte("invalid public key")

	err := Verify(invalidPubKey, message, sig)
	if err == nil {
		t.Error("expected error for invalid public key")
	}
}

func TestVerifySignature_InvalidBase64(t *testing.T) {
	tests := []struct {
		name    string
		payload *EncryptedPayload
	}{
		{
			name: "invalid ct_kem",
			payload: &EncryptedPayload{
				V:           1,
				CtKem:       "!!!invalid!!!",
				Nonce:       ToBase64URL([]byte("nonce")),
				AAD:         ToBase64URL([]byte("aad")),
				Ciphertext:  ToBase64URL([]byte("ct")),
				ServerSigPk: ToBase64URL(make([]byte, MLDSAPublicKeySize)),
				Sig:         ToBase64URL(make([]byte, MLDSASignatureSize)),
			},
		},
		{
			name: "invalid nonce",
			payload: &EncryptedPayload{
				V:           1,
				CtKem:       ToBase64URL([]byte("kem")),
				Nonce:       "!!!invalid!!!",
				AAD:         ToBase64URL([]byte("aad")),
				Ciphertext:  ToBase64URL([]byte("ct")),
				ServerSigPk: ToBase64URL(make([]byte, MLDSAPublicKeySize)),
				Sig:         ToBase64URL(make([]byte, MLDSASignatureSize)),
			},
		},
		{
			name: "invalid aad",
			payload: &EncryptedPayload{
				V:           1,
				CtKem:       ToBase64URL([]byte("kem")),
				Nonce:       ToBase64URL([]byte("nonce")),
				AAD:         "!!!invalid!!!",
				Ciphertext:  ToBase64URL([]byte("ct")),
				ServerSigPk: ToBase64URL(make([]byte, MLDSAPublicKeySize)),
				Sig:         ToBase64URL(make([]byte, MLDSASignatureSize)),
			},
		},
		{
			name: "invalid ciphertext",
			payload: &EncryptedPayload{
				V:           1,
				CtKem:       ToBase64URL([]byte("kem")),
				Nonce:       ToBase64URL([]byte("nonce")),
				AAD:         ToBase64URL([]byte("aad")),
				Ciphertext:  "!!!invalid!!!",
				ServerSigPk: ToBase64URL(make([]byte, MLDSAPublicKeySize)),
				Sig:         ToBase64URL(make([]byte, MLDSASignatureSize)),
			},
		},
		{
			name: "invalid server_sig_pk",
			payload: &EncryptedPayload{
				V:           1,
				CtKem:       ToBase64URL([]byte("kem")),
				Nonce:       ToBase64URL([]byte("nonce")),
				AAD:         ToBase64URL([]byte("aad")),
				Ciphertext:  ToBase64URL([]byte("ct")),
				ServerSigPk: "!!!invalid!!!",
				Sig:         ToBase64URL(make([]byte, MLDSASignatureSize)),
			},
		},
		{
			name: "invalid sig",
			payload: &EncryptedPayload{
				V:           1,
				CtKem:       ToBase64URL([]byte("kem")),
				Nonce:       ToBase64URL([]byte("nonce")),
				AAD:         ToBase64URL([]byte("aad")),
				Ciphertext:  ToBase64URL([]byte("ct")),
				ServerSigPk: ToBase64URL(make([]byte, MLDSAPublicKeySize)),
				Sig:         "!!!invalid!!!",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a pinned key that matches the payload's ServerSigPk for these tests
			pinnedKey := make([]byte, MLDSAPublicKeySize)
			err := VerifySignature(tt.payload, pinnedKey)
			if err == nil {
				t.Error("expected error for invalid base64")
			}
		})
	}
}

func TestVerifySignature_InvalidPublicKey(t *testing.T) {
	invalidPk := []byte("invalid public key")
	payload := &EncryptedPayload{
		V:           1,
		CtKem:       ToBase64URL([]byte("kem")),
		Nonce:       ToBase64URL([]byte("nonce")),
		AAD:         ToBase64URL([]byte("aad")),
		Ciphertext:  ToBase64URL([]byte("ct")),
		ServerSigPk: ToBase64URL(invalidPk), // Not a valid ML-DSA key
		Sig:         ToBase64URL(make([]byte, MLDSASignatureSize)),
	}

	// Pass the same invalid key as pinned to test that unmarshal fails
	err := VerifySignature(payload, invalidPk)
	if err == nil {
		t.Error("expected error for invalid public key")
	}
}

func TestVerifySignature_ServerKeyMismatch(t *testing.T) {
	// Generate two different keypairs
	pub1, _, err := mldsa65.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	pubBytes1, _ := pub1.MarshalBinary()

	pub2, _, err := mldsa65.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	pubBytes2, _ := pub2.MarshalBinary()

	// Create payload with pub1's key and correct field sizes
	payload := &EncryptedPayload{
		V: 1,
		Algs: AlgorithmSuite{
			KEM:  "ML-KEM-768",
			Sig:  "ML-DSA-65",
			AEAD: "AES-256-GCM",
			KDF:  "HKDF-SHA-512",
		},
		CtKem:       ToBase64URL(make([]byte, MLKEMCiphertextSize)), // 1088 bytes
		Nonce:       ToBase64URL(make([]byte, AESNonceSize)),        // 12 bytes
		AAD:         ToBase64URL([]byte("aad")),
		Ciphertext:  ToBase64URL([]byte("ct")),
		ServerSigPk: ToBase64URL(pubBytes1), // Payload contains pub1
		Sig:         ToBase64URL(make([]byte, MLDSASignatureSize)),
	}

	// But pass pub2 as the pinned key - should fail with key mismatch
	err = VerifySignature(payload, pubBytes2)
	if !errors.Is(err, ErrServerKeyMismatch) {
		t.Errorf("expected ErrServerKeyMismatch, got %v", err)
	}
}

func TestVerifySignature_InvalidSignature(t *testing.T) {
	// Generate a valid public key but provide invalid signature
	pub, _, err := mldsa65.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	pubBytes, err := pub.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	// Use correct field sizes for validation to pass
	payload := &EncryptedPayload{
		V: 1,
		Algs: AlgorithmSuite{
			KEM:  "ML-KEM-768",
			Sig:  "ML-DSA-65",
			AEAD: "AES-256-GCM",
			KDF:  "HKDF-SHA-512",
		},
		CtKem:       ToBase64URL(make([]byte, MLKEMCiphertextSize)), // 1088 bytes
		Nonce:       ToBase64URL(make([]byte, AESNonceSize)),        // 12 bytes
		AAD:         ToBase64URL([]byte("aad")),
		Ciphertext:  ToBase64URL([]byte("ct")),
		ServerSigPk: ToBase64URL(pubBytes),
		Sig:         ToBase64URL(make([]byte, MLDSASignatureSize)), // Invalid signature (all zeros)
	}

	// Pass the correct pinned key to ensure we get past key validation
	err = VerifySignature(payload, pubBytes)
	if !errors.Is(err, ErrSignatureVerificationFailed) {
		t.Errorf("expected ErrSignatureVerificationFailed, got %v", err)
	}
}

func TestVerifySignatureSafe(t *testing.T) {
	t.Run("returns false for invalid payload", func(t *testing.T) {
		pinnedKey := make([]byte, MLDSAPublicKeySize)
		payload := &EncryptedPayload{
			V:           1,
			CtKem:       "!!!invalid!!!",
			Nonce:       ToBase64URL([]byte("nonce")),
			AAD:         ToBase64URL([]byte("aad")),
			Ciphertext:  ToBase64URL([]byte("ct")),
			ServerSigPk: ToBase64URL(pinnedKey),
			Sig:         ToBase64URL(make([]byte, MLDSASignatureSize)),
		}
		if VerifySignatureSafe(payload, pinnedKey) {
			t.Error("VerifySignatureSafe() returned true for invalid payload")
		}
	})

	t.Run("returns false for invalid signature", func(t *testing.T) {
		pub, _, err := mldsa65.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		pubBytes, _ := pub.MarshalBinary()

		// Use correct field sizes for validation to pass
		ctKem := make([]byte, MLKEMCiphertextSize)
		nonce := make([]byte, AESNonceSize)

		payload := &EncryptedPayload{
			V: 1,
			Algs: AlgorithmSuite{
				KEM:  "ML-KEM-768",
				Sig:  "ML-DSA-65",
				AEAD: "AES-256-GCM",
				KDF:  "HKDF-SHA-512",
			},
			CtKem:       ToBase64URL(ctKem),
			Nonce:       ToBase64URL(nonce),
			AAD:         ToBase64URL([]byte("aad")),
			Ciphertext:  ToBase64URL([]byte("ct")),
			ServerSigPk: ToBase64URL(pubBytes),
			Sig:         ToBase64URL(make([]byte, MLDSASignatureSize)),
		}
		if VerifySignatureSafe(payload, pubBytes) {
			t.Error("VerifySignatureSafe() returned true for invalid signature")
		}
	})

	t.Run("returns true for valid signature", func(t *testing.T) {
		// Generate keypair
		pub, priv, err := mldsa65.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		pubBytes, _ := pub.MarshalBinary()

		// Use correct field sizes
		ctKem := make([]byte, MLKEMCiphertextSize)
		nonce := make([]byte, AESNonceSize)

		// Build payload
		payload := &EncryptedPayload{
			V: 1,
			Algs: AlgorithmSuite{
				KEM:  "ML-KEM-768",
				Sig:  "ML-DSA-65",
				AEAD: "AES-256-GCM",
				KDF:  "HKDF-SHA-512",
			},
			CtKem:       ToBase64URL(ctKem),
			Nonce:       ToBase64URL(nonce),
			AAD:         ToBase64URL([]byte("aad")),
			Ciphertext:  ToBase64URL([]byte("ct")),
			ServerSigPk: ToBase64URL(pubBytes),
		}

		// Build transcript and sign it
		transcript := buildTranscript(
			payload.V,
			payload.Algs,
			ctKem,
			nonce,
			[]byte("aad"),
			[]byte("ct"),
			pubBytes,
		)
		sig := make([]byte, mldsa65.SignatureSize)
		mldsa65.SignTo(priv, transcript, nil, false, sig)
		payload.Sig = ToBase64URL(sig)

		if !VerifySignatureSafe(payload, pubBytes) {
			t.Error("VerifySignatureSafe() returned false for valid signature")
		}
	})
}

func TestValidateServerPublicKey(t *testing.T) {
	t.Run("valid public key", func(t *testing.T) {
		pub, _, err := mldsa65.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		pubBytes, _ := pub.MarshalBinary()
		pubB64 := ToBase64URL(pubBytes)

		if !ValidateServerPublicKey(pubB64) {
			t.Error("ValidateServerPublicKey() returned false for valid public key")
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		if ValidateServerPublicKey("!!!invalid!!!") {
			t.Error("ValidateServerPublicKey() returned true for invalid base64")
		}
	})

	t.Run("wrong size", func(t *testing.T) {
		wrongSize := ToBase64URL(make([]byte, 100))
		if ValidateServerPublicKey(wrongSize) {
			t.Error("ValidateServerPublicKey() returned true for wrong size")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		if ValidateServerPublicKey("") {
			t.Error("ValidateServerPublicKey() returned true for empty string")
		}
	})

	t.Run("exact size", func(t *testing.T) {
		exactSize := ToBase64URL(make([]byte, MLDSAPublicKeySize))
		if !ValidateServerPublicKey(exactSize) {
			t.Error("ValidateServerPublicKey() returned false for exact size")
		}
	})
}

func BenchmarkVerify(b *testing.B) {
	pub, priv, _ := mldsa65.GenerateKey(nil)
	pubBytes, _ := pub.MarshalBinary()
	message := []byte("benchmark message for signature verification")

	sig := make([]byte, mldsa65.SignatureSize)
	mldsa65.SignTo(priv, message, nil, false, sig)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Verify(pubBytes, message, sig)
	}
}
