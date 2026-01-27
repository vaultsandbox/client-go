package vaultsandbox

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudflare/circl/kem/mlkem/mlkem768"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/vaultsandbox/client-go/internal/api"
	"github.com/vaultsandbox/client-go/internal/crypto"
)

// createTestEncryptedPayload creates a valid encrypted payload for testing.
// It encrypts the plaintext using the provided keypair and returns both the
// payload and the server signing key bytes.
func createTestEncryptedPayload(t *testing.T, plaintext []byte, kp *crypto.Keypair) (*crypto.EncryptedPayload, []byte) {
	t.Helper()

	// Generate server signing keypair
	serverPub, serverPriv, err := mldsa65.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	serverPubBytes, err := serverPub.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	// 1. Encapsulate to get shared secret and KEM ciphertext
	var pubKey mlkem768.PublicKey
	pubKey.Unpack(kp.PublicKey)

	ctKem := make([]byte, crypto.MLKEMCiphertextSize)
	sharedSecret := make([]byte, crypto.MLKEMSharedKeySize)
	pubKey.EncapsulateTo(ctKem, sharedSecret, nil)

	// 2. Derive AES key using the same method as the crypto package
	aad := []byte("test-aad")
	aesKey := deriveTestKey(sharedSecret, aad, ctKem)

	// 3. Encrypt with AES-GCM
	nonce := make([]byte, crypto.AESNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}

	block, _ := aes.NewCipher(aesKey)
	gcm, _ := cipher.NewGCM(block)
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)

	// 4. Create the encrypted payload
	algs := crypto.AlgorithmSuite{
		KEM:  "ML-KEM-768",
		Sig:  "ML-DSA-65",
		AEAD: "AES-256-GCM",
		KDF:  "HKDF-SHA-512",
	}

	payload := &crypto.EncryptedPayload{
		V:           1,
		Algs:        algs,
		CtKem:       crypto.ToBase64URL(ctKem),
		Nonce:       crypto.ToBase64URL(nonce),
		AAD:         crypto.ToBase64URL(aad),
		Ciphertext:  crypto.ToBase64URL(ciphertext),
		ServerSigPk: crypto.ToBase64URL(serverPubBytes),
	}

	// 5. Sign the payload
	transcript := buildTestTranscript(payload.V, algs, ctKem, nonce, aad, ciphertext, serverPubBytes)
	sig := make([]byte, mldsa65.SignatureSize)
	mldsa65.SignTo(serverPriv, transcript, nil, false, sig)
	payload.Sig = crypto.ToBase64URL(sig)

	return payload, serverPubBytes
}

// deriveTestKey mirrors the internal deriveKey function for test purposes.
func deriveTestKey(sharedSecret, aad, ctKem []byte) []byte {
	// Salt is SHA-256 hash of KEM ciphertext
	saltHash := sha256.Sum256(ctKem)
	salt := saltHash[:]

	// Info construction: context || aad_length (4 bytes BE) || aad
	// Must match crypto.HKDFContext = "vaultsandbox:email:v1"
	context := []byte(crypto.HKDFContext)
	aadLength := make([]byte, 4)
	binary.BigEndian.PutUint32(aadLength, uint32(len(aad)))

	info := make([]byte, 0, len(context)+4+len(aad))
	info = append(info, context...)
	info = append(info, aadLength...)
	info = append(info, aad...)

	key, _ := crypto.DeriveKey(sharedSecret, salt, info, crypto.AESKeySize)
	return key
}

// buildTestTranscript constructs the signature transcript.
// Must match crypto/verify.go buildTranscript function.
func buildTestTranscript(version int, algs crypto.AlgorithmSuite, ctKem, nonce, aad, ciphertext, serverSigPk []byte) []byte {
	transcript := []byte{byte(version)}

	algsCiphersuite := algs.KEM + ":" + algs.Sig + ":" + algs.AEAD + ":" + algs.KDF
	transcript = append(transcript, []byte(algsCiphersuite)...)
	// Must use crypto.HKDFContext = "vaultsandbox:email:v1"
	transcript = append(transcript, []byte(crypto.HKDFContext)...)
	transcript = append(transcript, ctKem...)
	transcript = append(transcript, nonce...)
	transcript = append(transcript, aad...)
	transcript = append(transcript, ciphertext...)
	transcript = append(transcript, serverSigPk...)

	return transcript
}

func TestWrapCryptoError_Nil(t *testing.T) {
	result := wrapCryptoError(nil)
	if result != nil {
		t.Errorf("wrapCryptoError(nil) = %v, want nil", result)
	}
}

func TestWrapCryptoError_ServerKeyMismatch(t *testing.T) {
	err := crypto.ErrServerKeyMismatch
	result := wrapCryptoError(err)

	var sigErr *SignatureVerificationError
	if !errors.As(result, &sigErr) {
		t.Fatalf("expected SignatureVerificationError, got %T", result)
	}
	if !sigErr.IsKeyMismatch {
		t.Error("IsKeyMismatch = false, want true")
	}
}

func TestWrapCryptoError_SignatureVerificationFailed(t *testing.T) {
	err := crypto.ErrSignatureVerificationFailed
	result := wrapCryptoError(err)

	var sigErr *SignatureVerificationError
	if !errors.As(result, &sigErr) {
		t.Fatalf("expected SignatureVerificationError, got %T", result)
	}
	if sigErr.IsKeyMismatch {
		t.Error("IsKeyMismatch = true, want false")
	}
}

func TestWrapCryptoError_OtherError(t *testing.T) {
	originalErr := errors.New("some other error")
	result := wrapCryptoError(originalErr)

	if result != originalErr {
		t.Errorf("wrapCryptoError(other) = %v, want %v", result, originalErr)
	}
}

func TestConvertDecryptedEmail_WithAttachments(t *testing.T) {
	inbox := &Inbox{}

	decrypted := &crypto.DecryptedEmail{
		ID:      "test-id",
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test Subject",
		Attachments: []crypto.DecryptedAttachment{
			{
				Filename:           "doc.pdf",
				ContentType:        "application/pdf",
				Size:               1024,
				ContentID:          "cid123",
				ContentDisposition: "attachment",
				Content:            []byte("pdf content"),
				Checksum:           "sha256hash",
			},
			{
				Filename:    "image.png",
				ContentType: "image/png",
				Size:        2048,
			},
		},
	}

	email := inbox.convertDecryptedEmail(decrypted)

	if len(email.Attachments) != 2 {
		t.Fatalf("Attachments length = %d, want 2", len(email.Attachments))
	}

	// Check first attachment
	att := email.Attachments[0]
	if att.Filename != "doc.pdf" {
		t.Errorf("Attachment[0].Filename = %s, want doc.pdf", att.Filename)
	}
	if att.ContentType != "application/pdf" {
		t.Errorf("Attachment[0].ContentType = %s, want application/pdf", att.ContentType)
	}
	if att.Size != 1024 {
		t.Errorf("Attachment[0].Size = %d, want 1024", att.Size)
	}
	if att.ContentID != "cid123" {
		t.Errorf("Attachment[0].ContentID = %s, want cid123", att.ContentID)
	}
	if att.ContentDisposition != "attachment" {
		t.Errorf("Attachment[0].ContentDisposition = %s, want attachment", att.ContentDisposition)
	}
	if string(att.Content) != "pdf content" {
		t.Errorf("Attachment[0].Content = %s, want pdf content", string(att.Content))
	}
	if att.Checksum != "sha256hash" {
		t.Errorf("Attachment[0].Checksum = %s, want sha256hash", att.Checksum)
	}

	// Check second attachment
	att2 := email.Attachments[1]
	if att2.Filename != "image.png" {
		t.Errorf("Attachment[1].Filename = %s, want image.png", att2.Filename)
	}
}

func TestDecryptMetadata_NilEncryptedMetadata(t *testing.T) {
	inbox := &Inbox{}
	// When EncryptedMetadata is nil, the email is treated as plain format
	// Testing both cases:
	t.Run("plain email with no metadata", func(t *testing.T) {
		rawEmail := &api.RawEmail{
			ID:                "email-123",
			EncryptedMetadata: nil,
			Metadata:          "", // No plain metadata either
		}

		_, err := inbox.decryptMetadata(rawEmail)
		if err == nil {
			t.Error("expected error for plain email with no metadata")
		}
		if err.Error() != "plain email has no metadata" {
			t.Errorf("error = %q, want 'plain email has no metadata'", err.Error())
		}
	})
}

func TestDecryptEmail_NilEncryptedMetadata(t *testing.T) {
	inbox := &Inbox{}
	// When EncryptedMetadata is nil, the email is treated as plain format
	rawEmail := &api.RawEmail{
		ID:                "email-123",
		EncryptedMetadata: nil,
		Metadata:          "", // No plain metadata either
	}

	_, err := inbox.decryptEmail(rawEmail)
	if err == nil {
		t.Error("expected error for plain email with no metadata")
	}
	if err.Error() != "plain email has no metadata" {
		t.Errorf("error = %q, want 'plain email has no metadata'", err.Error())
	}
}

func TestDecryptMetadata_Success(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create metadata with valid receivedAt timestamp
	metadata := map[string]interface{}{
		"from":       "sender@example.com",
		"to":         "recipient@example.com",
		"subject":    "Test Subject",
		"receivedAt": "2024-01-15T10:30:00Z",
	}
	metadataJSON, _ := json.Marshal(metadata)

	encryptedMetadata, serverPk := createTestEncryptedPayload(t, metadataJSON, kp)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: serverPk,
		encrypted:   true,
	}

	apiReceivedAt := time.Now().Truncate(time.Second)
	rawEmail := &api.RawEmail{
		ID:                "email-123",
		ReceivedAt:        apiReceivedAt,
		IsRead:            true,
		EncryptedMetadata: encryptedMetadata,
	}

	result, err := inbox.decryptMetadata(rawEmail)
	if err != nil {
		t.Fatalf("decryptMetadata() error = %v", err)
	}

	if result.ID != "email-123" {
		t.Errorf("ID = %s, want email-123", result.ID)
	}
	if result.From != "sender@example.com" {
		t.Errorf("From = %s, want sender@example.com", result.From)
	}
	if result.Subject != "Test Subject" {
		t.Errorf("Subject = %s, want Test Subject", result.Subject)
	}
	if result.IsRead != true {
		t.Error("IsRead = false, want true")
	}

	// ReceivedAt should be from metadata, not API
	expected, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	if !result.ReceivedAt.Equal(expected) {
		t.Errorf("ReceivedAt = %v, want %v", result.ReceivedAt, expected)
	}
}

func TestDecryptMetadata_InvalidReceivedAtFallback(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create metadata with invalid receivedAt timestamp (should fallback to API timestamp)
	metadata := map[string]interface{}{
		"from":       "sender@example.com",
		"to":         "recipient@example.com",
		"subject":    "Test Subject",
		"receivedAt": "invalid-date",
	}
	metadataJSON, _ := json.Marshal(metadata)

	encryptedMetadata, serverPk := createTestEncryptedPayload(t, metadataJSON, kp)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: serverPk,
		encrypted:   true,
	}

	apiReceivedAt := time.Now().Truncate(time.Second)
	rawEmail := &api.RawEmail{
		ID:                "email-123",
		ReceivedAt:        apiReceivedAt,
		IsRead:            false,
		EncryptedMetadata: encryptedMetadata,
	}

	result, err := inbox.decryptMetadata(rawEmail)
	if err != nil {
		t.Fatalf("decryptMetadata() error = %v", err)
	}

	// ReceivedAt should fallback to API timestamp
	if !result.ReceivedAt.Equal(apiReceivedAt) {
		t.Errorf("ReceivedAt = %v, want %v (API fallback)", result.ReceivedAt, apiReceivedAt)
	}
}

func TestDecryptMetadata_EmptyReceivedAtFallback(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create metadata without receivedAt (should fallback to API timestamp)
	metadata := map[string]interface{}{
		"from":    "sender@example.com",
		"to":      "recipient@example.com",
		"subject": "Test Subject",
	}
	metadataJSON, _ := json.Marshal(metadata)

	encryptedMetadata, serverPk := createTestEncryptedPayload(t, metadataJSON, kp)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: serverPk,
		encrypted:   true,
	}

	apiReceivedAt := time.Now().Truncate(time.Second)
	rawEmail := &api.RawEmail{
		ID:                "email-123",
		ReceivedAt:        apiReceivedAt,
		IsRead:            false,
		EncryptedMetadata: encryptedMetadata,
	}

	result, err := inbox.decryptMetadata(rawEmail)
	if err != nil {
		t.Fatalf("decryptMetadata() error = %v", err)
	}

	// ReceivedAt should fallback to API timestamp
	if !result.ReceivedAt.Equal(apiReceivedAt) {
		t.Errorf("ReceivedAt = %v, want %v (API fallback)", result.ReceivedAt, apiReceivedAt)
	}
}

func TestDecryptMetadata_VerifyAndDecryptError(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create a payload signed with different key (will fail verification)
	metadata := map[string]interface{}{
		"from":    "sender@example.com",
		"to":      "recipient@example.com",
		"subject": "Test Subject",
	}
	metadataJSON, _ := json.Marshal(metadata)

	encryptedMetadata, _ := createTestEncryptedPayload(t, metadataJSON, kp)

	// Use a different server key than what signed the payload
	differentServerPk := make([]byte, crypto.MLDSAPublicKeySize)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: differentServerPk,
		encrypted:   true,
	}

	rawEmail := &api.RawEmail{
		ID:                "email-123",
		EncryptedMetadata: encryptedMetadata,
	}

	_, err = inbox.decryptMetadata(rawEmail)
	if err == nil {
		t.Error("expected error for mismatched server key")
	}
}

func TestDecryptMetadata_ParseMetadataError(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create payload with invalid JSON
	invalidJSON := []byte("{invalid json")
	encryptedMetadata, serverPk := createTestEncryptedPayload(t, invalidJSON, kp)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: serverPk,
		encrypted:   true,
	}

	rawEmail := &api.RawEmail{
		ID:                "email-123",
		EncryptedMetadata: encryptedMetadata,
	}

	_, err = inbox.decryptMetadata(rawEmail)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecryptEmail_Success(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create metadata
	metadata := map[string]interface{}{
		"from":       "sender@example.com",
		"to":         "recipient@example.com",
		"subject":    "Test Subject",
		"receivedAt": "2024-01-15T10:30:00Z",
	}
	metadataJSON, _ := json.Marshal(metadata)
	encryptedMetadata, serverPk := createTestEncryptedPayload(t, metadataJSON, kp)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: serverPk,
		encrypted:   true,
	}

	rawEmail := &api.RawEmail{
		ID:                "email-123",
		ReceivedAt:        time.Now(),
		IsRead:            true,
		EncryptedMetadata: encryptedMetadata,
		EncryptedParsed:   nil, // No parsed content
	}

	result, err := inbox.decryptEmail(rawEmail)
	if err != nil {
		t.Fatalf("decryptEmail() error = %v", err)
	}

	if result.ID != "email-123" {
		t.Errorf("ID = %s, want email-123", result.ID)
	}
	if result.From != "sender@example.com" {
		t.Errorf("From = %s, want sender@example.com", result.From)
	}
	if result.Subject != "Test Subject" {
		t.Errorf("Subject = %s, want Test Subject", result.Subject)
	}
}

func TestDecryptEmail_WithParsedContent(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Generate a single server signing keypair to use for both payloads
	serverPub, serverPriv, err := mldsa65.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	serverPubBytes, _ := serverPub.MarshalBinary()

	// Create metadata
	metadata := map[string]interface{}{
		"from":       "sender@example.com",
		"to":         "recipient@example.com",
		"subject":    "Test Subject",
		"receivedAt": "2024-01-15T10:30:00Z",
	}
	metadataJSON, _ := json.Marshal(metadata)
	encryptedMetadata, _ := createTestEncryptedPayloadWithServerKeyPair(t, metadataJSON, kp, serverPub, serverPriv)

	// Create parsed content with the same server key
	parsed := map[string]interface{}{
		"text": "Plain text body",
		"html": "<p>HTML body</p>",
		"headers": map[string]interface{}{
			"X-Custom-Header": "custom-value",
		},
		"links":       []string{"https://example.com"},
		"attachments": []interface{}{},
	}
	parsedJSON, _ := json.Marshal(parsed)
	encryptedParsed, _ := createTestEncryptedPayloadWithServerKeyPair(t, parsedJSON, kp, serverPub, serverPriv)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: serverPubBytes,
		encrypted:   true,
	}

	rawEmail := &api.RawEmail{
		ID:                "email-123",
		ReceivedAt:        time.Now(),
		IsRead:            true,
		EncryptedMetadata: encryptedMetadata,
		EncryptedParsed:   encryptedParsed,
	}

	result, err := inbox.decryptEmail(rawEmail)
	if err != nil {
		t.Fatalf("decryptEmail() error = %v", err)
	}

	if result.Text != "Plain text body" {
		t.Errorf("Text = %s, want 'Plain text body'", result.Text)
	}
	if result.HTML != "<p>HTML body</p>" {
		t.Errorf("HTML = %s, want '<p>HTML body</p>'", result.HTML)
	}
	if len(result.Links) != 1 || result.Links[0] != "https://example.com" {
		t.Errorf("Links = %v, want [https://example.com]", result.Links)
	}
	if result.Headers["X-Custom-Header"] != "custom-value" {
		t.Errorf("Headers[X-Custom-Header] = %s, want custom-value", result.Headers["X-Custom-Header"])
	}
}

// createTestEncryptedPayloadWithServerKeyPair creates a payload signed with a specific server keypair.
func createTestEncryptedPayloadWithServerKeyPair(t *testing.T, plaintext []byte, kp *crypto.Keypair, serverPub *mldsa65.PublicKey, serverPriv *mldsa65.PrivateKey) (*crypto.EncryptedPayload, []byte) {
	t.Helper()

	serverPubBytes, err := serverPub.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	// 1. Encapsulate to get shared secret and KEM ciphertext
	var pubKey mlkem768.PublicKey
	pubKey.Unpack(kp.PublicKey)

	ctKem := make([]byte, crypto.MLKEMCiphertextSize)
	sharedSecret := make([]byte, crypto.MLKEMSharedKeySize)
	pubKey.EncapsulateTo(ctKem, sharedSecret, nil)

	// 2. Derive AES key
	aad := []byte("test-aad")
	aesKey := deriveTestKey(sharedSecret, aad, ctKem)

	// 3. Encrypt with AES-GCM
	nonce := make([]byte, crypto.AESNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}

	block, _ := aes.NewCipher(aesKey)
	gcm, _ := cipher.NewGCM(block)
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)

	// 4. Create the encrypted payload
	algs := crypto.AlgorithmSuite{
		KEM:  "ML-KEM-768",
		Sig:  "ML-DSA-65",
		AEAD: "AES-256-GCM",
		KDF:  "HKDF-SHA-512",
	}

	payload := &crypto.EncryptedPayload{
		V:           1,
		Algs:        algs,
		CtKem:       crypto.ToBase64URL(ctKem),
		Nonce:       crypto.ToBase64URL(nonce),
		AAD:         crypto.ToBase64URL(aad),
		Ciphertext:  crypto.ToBase64URL(ciphertext),
		ServerSigPk: crypto.ToBase64URL(serverPubBytes),
	}

	// 5. Sign the payload
	transcript := buildTestTranscript(payload.V, algs, ctKem, nonce, aad, ciphertext, serverPubBytes)
	sig := make([]byte, mldsa65.SignatureSize)
	mldsa65.SignTo(serverPriv, transcript, nil, false, sig)
	payload.Sig = crypto.ToBase64URL(sig)

	return payload, serverPubBytes
}

func TestDecryptEmail_VerifyAndDecryptError(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	metadata := map[string]interface{}{
		"from":    "sender@example.com",
		"to":      "recipient@example.com",
		"subject": "Test Subject",
	}
	metadataJSON, _ := json.Marshal(metadata)
	encryptedMetadata, _ := createTestEncryptedPayload(t, metadataJSON, kp)

	// Use wrong server key
	differentServerPk := make([]byte, crypto.MLDSAPublicKeySize)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: differentServerPk,
	}

	rawEmail := &api.RawEmail{
		ID:                "email-123",
		EncryptedMetadata: encryptedMetadata,
	}

	_, err = inbox.decryptEmail(rawEmail)
	if err == nil {
		t.Error("expected error for mismatched server key")
	}
}

func TestDecryptEmail_ParseMetadataError(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	invalidJSON := []byte("{invalid json")
	encryptedMetadata, serverPk := createTestEncryptedPayload(t, invalidJSON, kp)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: serverPk,
	}

	rawEmail := &api.RawEmail{
		ID:                "email-123",
		EncryptedMetadata: encryptedMetadata,
	}

	_, err = inbox.decryptEmail(rawEmail)
	if err == nil {
		t.Error("expected error for invalid metadata JSON")
	}
}

func TestDecryptEmail_ApplyParsedContentError(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create valid metadata
	metadata := map[string]interface{}{
		"from":    "sender@example.com",
		"to":      "recipient@example.com",
		"subject": "Test Subject",
	}
	metadataJSON, _ := json.Marshal(metadata)
	encryptedMetadata, serverPk := createTestEncryptedPayload(t, metadataJSON, kp)

	// Create invalid parsed content (will fail decryption due to wrong server key)
	invalidParsed := []byte("{invalid json")
	encryptedParsed, parsedServerPk := createTestEncryptedPayload(t, invalidParsed, kp)

	// Use a server key that matches metadata but not parsed
	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: serverPk,
	}

	// Set the parsed content to use a different server key
	_ = parsedServerPk // Different key than serverPk

	rawEmail := &api.RawEmail{
		ID:                "email-123",
		EncryptedMetadata: encryptedMetadata,
		EncryptedParsed:   encryptedParsed,
	}

	_, err = inbox.decryptEmail(rawEmail)
	if err == nil {
		t.Error("expected error for mismatched server key in parsed content")
	}
}

func TestApplyParsedContent_VerifyAndDecryptError(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create parsed content with a different server key
	parsed := map[string]interface{}{
		"text": "Body",
	}
	parsedJSON, _ := json.Marshal(parsed)
	encryptedParsed, _ := createTestEncryptedPayload(t, parsedJSON, kp)

	// Use a different server key
	differentServerPk := make([]byte, crypto.MLDSAPublicKeySize)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: differentServerPk,
	}

	decrypted := &crypto.DecryptedEmail{}
	err = inbox.applyParsedContent(encryptedParsed, decrypted)
	if err == nil {
		t.Error("expected error for mismatched server key")
	}
}

func TestApplyParsedContent_ParseError(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create parsed content with invalid JSON
	invalidJSON := []byte("{invalid json")
	encryptedParsed, serverPk := createTestEncryptedPayload(t, invalidJSON, kp)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: serverPk,
	}

	decrypted := &crypto.DecryptedEmail{}
	err = inbox.applyParsedContent(encryptedParsed, decrypted)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestApplyParsedContent_Success(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create valid parsed content with auth results
	authResults := map[string]interface{}{
		"spf":  map[string]interface{}{"result": "pass"},
		"dkim": []map[string]interface{}{{"result": "pass"}},
	}
	authResultsJSON, _ := json.Marshal(authResults)

	parsed := map[string]interface{}{
		"text": "Plain text body",
		"html": "<p>HTML body</p>",
		"headers": map[string]interface{}{
			"X-Header": "value",
		},
		"links": []string{"https://example.com"},
		"attachments": []map[string]interface{}{
			{
				"filename":    "file.txt",
				"contentType": "text/plain",
				"size":        100,
			},
		},
		"authResults": authResults,
	}
	parsedJSON, _ := json.Marshal(parsed)
	encryptedParsed, serverPk := createTestEncryptedPayload(t, parsedJSON, kp)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: serverPk,
		encrypted:   true,
	}

	decrypted := &crypto.DecryptedEmail{}
	err = inbox.applyParsedContent(encryptedParsed, decrypted)
	if err != nil {
		t.Fatalf("applyParsedContent() error = %v", err)
	}

	if decrypted.Text != "Plain text body" {
		t.Errorf("Text = %s, want 'Plain text body'", decrypted.Text)
	}
	if decrypted.HTML != "<p>HTML body</p>" {
		t.Errorf("HTML = %s, want '<p>HTML body</p>'", decrypted.HTML)
	}
	if decrypted.Headers["X-Header"] != "value" {
		t.Errorf("Headers[X-Header] = %s, want value", decrypted.Headers["X-Header"])
	}

	// Check auth results are raw JSON
	if decrypted.AuthResults == nil {
		t.Error("AuthResults should not be nil")
	}

	// Verify auth results can be unmarshaled
	var ar map[string]interface{}
	if err := json.Unmarshal(decrypted.AuthResults, &ar); err != nil {
		t.Errorf("AuthResults unmarshal error = %v", err)
	}

	// Verify auth results is valid JSON that matches what we expect
	if len(authResultsJSON) > 0 && decrypted.AuthResults != nil {
		// AuthResults were set correctly
	}
}

func TestVerifyAndDecrypt_Success(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("test plaintext data")
	payload, serverPk := createTestEncryptedPayload(t, plaintext, kp)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: serverPk,
		encrypted:   true,
	}

	result, err := inbox.verifyAndDecrypt(payload)
	if err != nil {
		t.Fatalf("verifyAndDecrypt() error = %v", err)
	}

	if string(result) != string(plaintext) {
		t.Errorf("verifyAndDecrypt() = %s, want %s", string(result), string(plaintext))
	}
}

func TestVerifyAndDecrypt_SignatureError(t *testing.T) {
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("test plaintext data")
	payload, _ := createTestEncryptedPayload(t, plaintext, kp)

	// Use wrong server key (will fail verification)
	wrongServerPk := make([]byte, crypto.MLDSAPublicKeySize)

	inbox := &Inbox{
		keypair:     kp,
		serverSigPk: wrongServerPk,
		encrypted:   true,
	}

	_, err = inbox.verifyAndDecrypt(payload)
	if err == nil {
		t.Error("expected error for wrong server key")
	}

	// Should be wrapped as SignatureVerificationError
	var sigErr *SignatureVerificationError
	if !errors.As(err, &sigErr) {
		t.Errorf("expected SignatureVerificationError, got %T: %v", err, err)
	}
}

// =============================================================================
// Plain Email Tests (non-encrypted)
// =============================================================================

func TestDecodePlainEmail_Success(t *testing.T) {
	inbox := &Inbox{}

	// Create Base64-encoded metadata
	metadata := map[string]interface{}{
		"from":       "sender@example.com",
		"to":         "recipient@example.com",
		"subject":    "Test Plain Email",
		"receivedAt": "2024-01-15T10:30:00Z",
	}
	metadataJSON, _ := json.Marshal(metadata)
	metadataB64 := crypto.ToBase64URL(metadataJSON)

	apiReceivedAt := time.Now().Truncate(time.Second)
	rawEmail := &api.RawEmail{
		ID:         "plain-email-123",
		ReceivedAt: apiReceivedAt,
		IsRead:     true,
		Metadata:   metadataB64,
		// No Parsed content - metadata only
	}

	result, err := inbox.decodePlainEmail(rawEmail)
	if err != nil {
		t.Fatalf("decodePlainEmail() error = %v", err)
	}

	if result.ID != "plain-email-123" {
		t.Errorf("ID = %s, want plain-email-123", result.ID)
	}
	if result.From != "sender@example.com" {
		t.Errorf("From = %s, want sender@example.com", result.From)
	}
	if result.Subject != "Test Plain Email" {
		t.Errorf("Subject = %s, want Test Plain Email", result.Subject)
	}
	if result.IsRead != true {
		t.Error("IsRead = false, want true")
	}

	// ReceivedAt should be from metadata
	expected, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	if !result.ReceivedAt.Equal(expected) {
		t.Errorf("ReceivedAt = %v, want %v", result.ReceivedAt, expected)
	}
}

func TestDecodePlainEmail_WithParsedContent(t *testing.T) {
	inbox := &Inbox{}

	// Create Base64-encoded metadata
	metadata := map[string]interface{}{
		"from":       "sender@example.com",
		"to":         "recipient@example.com",
		"subject":    "Test Plain Email with Body",
		"receivedAt": "2024-01-15T10:30:00Z",
	}
	metadataJSON, _ := json.Marshal(metadata)
	metadataB64 := crypto.ToBase64URL(metadataJSON)

	// Create Base64-encoded parsed content
	parsed := map[string]interface{}{
		"text": "This is the plain text body",
		"html": "<p>This is the HTML body</p>",
		"headers": map[string]interface{}{
			"X-Custom-Header": "custom-value",
			"X-Priority":      "high",
		},
		"links": []string{"https://example.com", "https://test.com"},
		"attachments": []map[string]interface{}{
			{
				"filename":    "doc.pdf",
				"contentType": "application/pdf",
				"size":        1024,
			},
		},
	}
	parsedJSON, _ := json.Marshal(parsed)
	parsedB64 := crypto.ToBase64URL(parsedJSON)

	rawEmail := &api.RawEmail{
		ID:         "plain-email-456",
		ReceivedAt: time.Now(),
		IsRead:     false,
		Metadata:   metadataB64,
		Parsed:     parsedB64,
	}

	result, err := inbox.decodePlainEmail(rawEmail)
	if err != nil {
		t.Fatalf("decodePlainEmail() error = %v", err)
	}

	if result.Text != "This is the plain text body" {
		t.Errorf("Text = %s, want 'This is the plain text body'", result.Text)
	}
	if result.HTML != "<p>This is the HTML body</p>" {
		t.Errorf("HTML = %s, want '<p>This is the HTML body</p>'", result.HTML)
	}
	if len(result.Links) != 2 {
		t.Errorf("Links length = %d, want 2", len(result.Links))
	}
	if result.Headers["X-Custom-Header"] != "custom-value" {
		t.Errorf("Headers[X-Custom-Header] = %s, want custom-value", result.Headers["X-Custom-Header"])
	}
	if len(result.Attachments) != 1 {
		t.Fatalf("Attachments length = %d, want 1", len(result.Attachments))
	}
	if result.Attachments[0].Filename != "doc.pdf" {
		t.Errorf("Attachments[0].Filename = %s, want doc.pdf", result.Attachments[0].Filename)
	}
}

func TestDecodePlainEmail_WithAuthResults(t *testing.T) {
	inbox := &Inbox{}

	metadata := map[string]interface{}{
		"from":    "sender@example.com",
		"to":      "recipient@example.com",
		"subject": "Auth Results Test",
	}
	metadataJSON, _ := json.Marshal(metadata)
	metadataB64 := crypto.ToBase64URL(metadataJSON)

	// Create parsed content with auth results
	parsed := map[string]interface{}{
		"text": "body",
		"html": "",
		"authResults": map[string]interface{}{
			"spf":  map[string]interface{}{"result": "pass", "domain": "example.com"},
			"dkim": []map[string]interface{}{{"result": "pass", "domain": "example.com"}},
		},
	}
	parsedJSON, _ := json.Marshal(parsed)
	parsedB64 := crypto.ToBase64URL(parsedJSON)

	rawEmail := &api.RawEmail{
		ID:         "plain-email-auth",
		ReceivedAt: time.Now(),
		Metadata:   metadataB64,
		Parsed:     parsedB64,
	}

	result, err := inbox.decodePlainEmail(rawEmail)
	if err != nil {
		t.Fatalf("decodePlainEmail() error = %v", err)
	}

	if result.AuthResults == nil {
		t.Fatal("AuthResults should not be nil")
	}
	if result.AuthResults.SPF == nil {
		t.Fatal("SPF should not be nil")
	}
	if result.AuthResults.SPF.Result != "pass" {
		t.Errorf("SPF.Result = %s, want pass", result.AuthResults.SPF.Result)
	}
}

func TestDecodePlainEmail_NoMetadata(t *testing.T) {
	inbox := &Inbox{}

	rawEmail := &api.RawEmail{
		ID:       "plain-email-no-meta",
		Metadata: "",
	}

	_, err := inbox.decodePlainEmail(rawEmail)
	if err == nil {
		t.Error("expected error for plain email with no metadata")
	}
	if err.Error() != "plain email has no metadata" {
		t.Errorf("error = %q, want 'plain email has no metadata'", err.Error())
	}
}

func TestDecodePlainEmail_InvalidBase64Metadata(t *testing.T) {
	inbox := &Inbox{}

	rawEmail := &api.RawEmail{
		ID:       "plain-email-bad-b64",
		Metadata: "!!!invalid-base64!!!",
	}

	_, err := inbox.decodePlainEmail(rawEmail)
	if err == nil {
		t.Error("expected error for invalid Base64 metadata")
	}
}

func TestDecodePlainEmail_InvalidJSONMetadata(t *testing.T) {
	inbox := &Inbox{}

	// Valid Base64 but invalid JSON
	invalidJSON := crypto.ToBase64URL([]byte("{invalid json"))

	rawEmail := &api.RawEmail{
		ID:       "plain-email-bad-json",
		Metadata: invalidJSON,
	}

	_, err := inbox.decodePlainEmail(rawEmail)
	if err == nil {
		t.Error("expected error for invalid JSON metadata")
	}
}

func TestDecodePlainEmail_InvalidBase64Parsed(t *testing.T) {
	inbox := &Inbox{}

	// Valid metadata
	metadata := map[string]interface{}{
		"from":    "sender@example.com",
		"to":      "recipient@example.com",
		"subject": "Test",
	}
	metadataJSON, _ := json.Marshal(metadata)
	metadataB64 := crypto.ToBase64URL(metadataJSON)

	rawEmail := &api.RawEmail{
		ID:       "plain-email-bad-parsed-b64",
		Metadata: metadataB64,
		Parsed:   "!!!invalid-base64!!!",
	}

	_, err := inbox.decodePlainEmail(rawEmail)
	if err == nil {
		t.Error("expected error for invalid Base64 parsed content")
	}
}

func TestDecodePlainEmail_InvalidJSONParsed(t *testing.T) {
	inbox := &Inbox{}

	// Valid metadata
	metadata := map[string]interface{}{
		"from":    "sender@example.com",
		"to":      "recipient@example.com",
		"subject": "Test",
	}
	metadataJSON, _ := json.Marshal(metadata)
	metadataB64 := crypto.ToBase64URL(metadataJSON)

	// Valid Base64 but invalid JSON for parsed content
	invalidJSON := crypto.ToBase64URL([]byte("{invalid json"))

	rawEmail := &api.RawEmail{
		ID:       "plain-email-bad-parsed-json",
		Metadata: metadataB64,
		Parsed:   invalidJSON,
	}

	_, err := inbox.decodePlainEmail(rawEmail)
	if err == nil {
		t.Error("expected error for invalid JSON parsed content")
	}
}

func TestDecryptMetadata_PlainEmail_Success(t *testing.T) {
	inbox := &Inbox{}

	// Create Base64-encoded metadata for plain email
	metadata := map[string]interface{}{
		"from":       "sender@example.com",
		"to":         "recipient@example.com",
		"subject":    "Plain Metadata Test",
		"receivedAt": "2024-01-15T10:30:00Z",
	}
	metadataJSON, _ := json.Marshal(metadata)
	metadataB64 := crypto.ToBase64URL(metadataJSON)

	apiReceivedAt := time.Now().Truncate(time.Second)
	rawEmail := &api.RawEmail{
		ID:                "plain-meta-123",
		ReceivedAt:        apiReceivedAt,
		IsRead:            true,
		EncryptedMetadata: nil, // Makes it a plain email
		Metadata:          metadataB64,
	}

	result, err := inbox.decryptMetadata(rawEmail)
	if err != nil {
		t.Fatalf("decryptMetadata() error = %v", err)
	}

	if result.ID != "plain-meta-123" {
		t.Errorf("ID = %s, want plain-meta-123", result.ID)
	}
	if result.From != "sender@example.com" {
		t.Errorf("From = %s, want sender@example.com", result.From)
	}
	if result.Subject != "Plain Metadata Test" {
		t.Errorf("Subject = %s, want Plain Metadata Test", result.Subject)
	}
	if result.IsRead != true {
		t.Error("IsRead = false, want true")
	}

	// ReceivedAt should be from metadata
	expected, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	if !result.ReceivedAt.Equal(expected) {
		t.Errorf("ReceivedAt = %v, want %v", result.ReceivedAt, expected)
	}
}

func TestDecryptMetadata_PlainEmail_InvalidBase64(t *testing.T) {
	inbox := &Inbox{}

	rawEmail := &api.RawEmail{
		ID:                "plain-meta-bad-b64",
		EncryptedMetadata: nil,
		Metadata:          "!!!invalid-base64!!!",
	}

	_, err := inbox.decryptMetadata(rawEmail)
	if err == nil {
		t.Error("expected error for invalid Base64 metadata")
	}
}

func TestDecryptMetadata_PlainEmail_InvalidJSON(t *testing.T) {
	inbox := &Inbox{}

	invalidJSON := crypto.ToBase64URL([]byte("{invalid json"))

	rawEmail := &api.RawEmail{
		ID:                "plain-meta-bad-json",
		EncryptedMetadata: nil,
		Metadata:          invalidJSON,
	}

	_, err := inbox.decryptMetadata(rawEmail)
	if err == nil {
		t.Error("expected error for invalid JSON metadata")
	}
}

func TestDecryptMetadata_PlainEmail_ReceivedAtFallback(t *testing.T) {
	inbox := &Inbox{}

	// Create metadata without receivedAt
	metadata := map[string]interface{}{
		"from":    "sender@example.com",
		"to":      "recipient@example.com",
		"subject": "No Timestamp",
	}
	metadataJSON, _ := json.Marshal(metadata)
	metadataB64 := crypto.ToBase64URL(metadataJSON)

	apiReceivedAt := time.Now().Truncate(time.Second)
	rawEmail := &api.RawEmail{
		ID:                "plain-meta-fallback",
		ReceivedAt:        apiReceivedAt,
		EncryptedMetadata: nil,
		Metadata:          metadataB64,
	}

	result, err := inbox.decryptMetadata(rawEmail)
	if err != nil {
		t.Fatalf("decryptMetadata() error = %v", err)
	}

	// ReceivedAt should fallback to API timestamp
	if !result.ReceivedAt.Equal(apiReceivedAt) {
		t.Errorf("ReceivedAt = %v, want %v (API fallback)", result.ReceivedAt, apiReceivedAt)
	}
}

func TestDecryptEmail_PlainEmail_ViaDecryptEmail(t *testing.T) {
	// Test that decryptEmail correctly routes plain emails to decodePlainEmail
	inbox := &Inbox{}

	metadata := map[string]interface{}{
		"from":       "sender@example.com",
		"to":         "recipient@example.com",
		"subject":    "Plain Email via decryptEmail",
		"receivedAt": "2024-01-15T10:30:00Z",
	}
	metadataJSON, _ := json.Marshal(metadata)
	metadataB64 := crypto.ToBase64URL(metadataJSON)

	parsed := map[string]interface{}{
		"text": "Plain text body",
		"html": "<p>HTML body</p>",
	}
	parsedJSON, _ := json.Marshal(parsed)
	parsedB64 := crypto.ToBase64URL(parsedJSON)

	rawEmail := &api.RawEmail{
		ID:                "plain-via-decrypt",
		ReceivedAt:        time.Now(),
		IsRead:            true,
		EncryptedMetadata: nil, // Plain email
		Metadata:          metadataB64,
		Parsed:            parsedB64,
	}

	result, err := inbox.decryptEmail(rawEmail)
	if err != nil {
		t.Fatalf("decryptEmail() error = %v", err)
	}

	if result.ID != "plain-via-decrypt" {
		t.Errorf("ID = %s, want plain-via-decrypt", result.ID)
	}
	if result.From != "sender@example.com" {
		t.Errorf("From = %s, want sender@example.com", result.From)
	}
	if result.Text != "Plain text body" {
		t.Errorf("Text = %s, want 'Plain text body'", result.Text)
	}
}

// =============================================================================
// GetRawEmail Tests (using httptest)
// =============================================================================

func TestGetRawEmail_PlainEmail_Success(t *testing.T) {
	rawEmailContent := "From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: Test\r\n\r\nHello World"
	rawB64 := crypto.ToBase64URL([]byte(rawEmailContent))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":  "email-123",
			"raw": rawB64,
			// No encryptedRaw - this is a plain email
		})
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL), api.WithRetries(0))
	client := &Client{apiClient: apiClient}
	inbox := &Inbox{
		emailAddress: "test@example.com",
		client:       client,
		encrypted:    false,
	}

	result, err := inbox.GetRawEmail(context.Background(), "email-123")
	if err != nil {
		t.Fatalf("GetRawEmail() error = %v", err)
	}

	if result != rawEmailContent {
		t.Errorf("GetRawEmail() = %q, want %q", result, rawEmailContent)
	}
}

func TestGetRawEmail_PlainEmail_EmptyRaw(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":  "email-123",
			"raw": "", // Empty raw content
		})
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL), api.WithRetries(0))
	client := &Client{apiClient: apiClient}
	inbox := &Inbox{
		emailAddress: "test@example.com",
		client:       client,
		encrypted:    false,
	}

	_, err := inbox.GetRawEmail(context.Background(), "email-123")
	if err == nil {
		t.Fatal("GetRawEmail() expected error for empty raw content")
	}
	if err.Error() != "plain email has no raw content" {
		t.Errorf("error = %q, want 'plain email has no raw content'", err.Error())
	}
}

func TestGetRawEmail_PlainEmail_InvalidBase64(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":  "email-123",
			"raw": "!!!invalid-base64!!!", // Invalid base64
		})
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL), api.WithRetries(0))
	client := &Client{apiClient: apiClient}
	inbox := &Inbox{
		emailAddress: "test@example.com",
		client:       client,
		encrypted:    false,
	}

	_, err := inbox.GetRawEmail(context.Background(), "email-123")
	if err == nil {
		t.Fatal("GetRawEmail() expected error for invalid base64")
	}
}

// =============================================================================
// GetRawEmail Encrypted Tests
// =============================================================================

func TestGetRawEmail_EncryptedEmail_Success(t *testing.T) {
	// Same raw email content as plain test - should get identical result
	rawEmailContent := "From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: Test\r\n\r\nHello World"
	rawB64 := crypto.ToBase64URL([]byte(rawEmailContent))

	// Generate keypair for encryption
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}

	// Create encrypted payload containing the base64-encoded raw email
	encryptedRaw, serverPk := createTestEncryptedPayload(t, []byte(rawB64), kp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":           "email-123",
			"encryptedRaw": encryptedRaw,
			// No plain raw - this is an encrypted email
		})
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL), api.WithRetries(0))
	client := &Client{apiClient: apiClient}
	inbox := &Inbox{
		emailAddress: "test@example.com",
		client:       client,
		keypair:      kp,
		serverSigPk:  serverPk,
		encrypted:    true,
	}

	result, err := inbox.GetRawEmail(context.Background(), "email-123")
	if err != nil {
		t.Fatalf("GetRawEmail() error = %v", err)
	}

	// Should get the exact same content as plain email test
	if result != rawEmailContent {
		t.Errorf("GetRawEmail() = %q, want %q", result, rawEmailContent)
	}
}

func TestGetRawEmail_EncryptedEmail_DecryptionError(t *testing.T) {
	// Create encrypted payload with one keypair
	kp, _ := crypto.GenerateKeypair()
	rawB64 := crypto.ToBase64URL([]byte("raw email content"))
	encryptedRaw, _ := createTestEncryptedPayload(t, []byte(rawB64), kp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":           "email-123",
			"encryptedRaw": encryptedRaw,
		})
	}))
	defer server.Close()

	// Use a different server key - decryption should fail
	differentServerPk := make([]byte, crypto.MLDSAPublicKeySize)

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL), api.WithRetries(0))
	client := &Client{apiClient: apiClient}
	inbox := &Inbox{
		emailAddress: "test@example.com",
		client:       client,
		keypair:      kp,
		serverSigPk:  differentServerPk, // Wrong key
		encrypted:    true,
	}

	_, err := inbox.GetRawEmail(context.Background(), "email-123")
	if err == nil {
		t.Fatal("GetRawEmail() expected error for decryption failure")
	}

	// Should be a signature verification error
	var sigErr *SignatureVerificationError
	if !errors.As(err, &sigErr) {
		t.Errorf("expected SignatureVerificationError, got %T: %v", err, err)
	}
}

func TestGetRawEmail_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "email not found"})
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL), api.WithRetries(0))
	client := &Client{apiClient: apiClient}
	inbox := &Inbox{
		emailAddress: "test@example.com",
		client:       client,
	}

	_, err := inbox.GetRawEmail(context.Background(), "email-123")
	if err == nil {
		t.Fatal("GetRawEmail() expected error for API error")
	}
}
