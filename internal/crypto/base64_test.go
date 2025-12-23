package crypto

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestBase64URLRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"simple", []byte("hello")},
		{"hello world", []byte("hello world")},
		{"binary zeros", []byte{0x00, 0x00, 0x00}},
		{"binary all ones", []byte{0xff, 0xff, 0xff}},
		{"binary mixed", []byte{0x00, 0xff, 0x7f, 0x80}},
		{"url unsafe chars", []byte{0xfb, 0xf0}}, // Would produce + or / in standard base64
		{"single byte", []byte{0x42}},
		{"two bytes", []byte{0x42, 0x43}},
		{"three bytes", []byte{0x42, 0x43, 0x44}},
		{"large data", make([]byte, 10000)},
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
	tests := []struct {
		name string
		data []byte
	}{
		{"one byte", []byte("a")},     // Would normally have == padding
		{"two bytes", []byte("ab")},   // Would normally have = padding
		{"three bytes", []byte("abc")}, // No padding needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := ToBase64URL(tt.data)
			if strings.Contains(encoded, "=") {
				t.Errorf("encoded string contains padding: %s", encoded)
			}
		})
	}
}

func TestBase64URL_URLSafe(t *testing.T) {
	// Generate data that would produce + and / in standard base64
	// 0xfb will produce + and 0x3f will produce /
	data := []byte{0xfb, 0xff, 0x3f, 0xff}

	encoded := ToBase64URL(data)

	if strings.Contains(encoded, "+") {
		t.Errorf("encoded contains '+' which is not URL-safe: %s", encoded)
	}
	if strings.Contains(encoded, "/") {
		t.Errorf("encoded contains '/' which is not URL-safe: %s", encoded)
	}
}

func TestFromBase64URL_InvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"invalid chars", "!!!invalid!!!"},
		{"spaces in middle", "aGVs bG8"}, // Invalid: space in middle of base64
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromBase64URL(tt.input)
			if err == nil {
				t.Error("expected error for invalid input")
			}
		})
	}
}

func TestDecodeBase64_MultipleFormats(t *testing.T) {
	original := []byte("hello world")

	tests := []struct {
		name    string
		encoded string
	}{
		{"raw url encoding", "aGVsbG8gd29ybGQ"},
		{"url encoding with padding", "aGVsbG8gd29ybGQ="},
		{"standard encoding", "aGVsbG8gd29ybGQ="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := DecodeBase64(tt.encoded)
			if err != nil {
				t.Fatalf("DecodeBase64() error = %v", err)
			}
			if !bytes.Equal(decoded, original) {
				t.Errorf("DecodeBase64() = %v, want %v", decoded, original)
			}
		})
	}
}

func TestDecodeBase64_URLSafeChars(t *testing.T) {
	// Test decoding with URL-safe characters
	// "-" and "_" should work (URL-safe replacements for "+" and "/")
	encoded := "-_8" // Contains URL-safe characters
	_, err := DecodeBase64(encoded)
	// This should not error (even if the decoded content is meaningless)
	if err != nil {
		t.Errorf("DecodeBase64() with URL-safe chars failed: %v", err)
	}
}

func TestToBase64_StandardEncoding(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{"empty", []byte{}, ""},
		{"hello", []byte("hello"), "aGVsbG8="},
		{"hello world", []byte("hello world"), "aGVsbG8gd29ybGQ="},
		{"one byte", []byte("a"), "YQ=="},
		{"two bytes", []byte("ab"), "YWI="},
		{"three bytes", []byte("abc"), "YWJj"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := ToBase64(tt.data)
			if encoded != tt.expected {
				t.Errorf("ToBase64() = %s, want %s", encoded, tt.expected)
			}
		})
	}
}

func TestFromBase64_StandardDecoding(t *testing.T) {
	tests := []struct {
		name     string
		encoded  string
		expected []byte
	}{
		{"empty", "", []byte{}},
		{"hello", "aGVsbG8=", []byte("hello")},
		{"hello world", "aGVsbG8gd29ybGQ=", []byte("hello world")},
		{"one byte", "YQ==", []byte("a")},
		{"two bytes", "YWI=", []byte("ab")},
		{"three bytes", "YWJj", []byte("abc")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := FromBase64(tt.encoded)
			if err != nil {
				t.Fatalf("FromBase64() error = %v", err)
			}
			if !bytes.Equal(decoded, tt.expected) {
				t.Errorf("FromBase64() = %v, want %v", decoded, tt.expected)
			}
		})
	}
}

func TestBase64StandardRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"simple", []byte("hello")},
		{"binary", []byte{0x00, 0xff, 0x7f, 0x80}},
		{"large", make([]byte, 1000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := ToBase64(tt.data)
			decoded, err := FromBase64(encoded)
			if err != nil {
				t.Fatalf("FromBase64() error = %v", err)
			}
			if !bytes.Equal(decoded, tt.data) {
				t.Errorf("round trip failed: got %v, want %v", decoded, tt.data)
			}
		})
	}
}

func TestFromBase64_InvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
	}{
		{"invalid chars", "!!!invalid!!!"},
		{"url-safe chars", "-_8"}, // URL-safe chars don't work with standard base64
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromBase64(tt.encoded)
			if err == nil {
				t.Error("expected error for invalid input")
			}
		})
	}
}

func TestToBase64_WithPadding(t *testing.T) {
	// Standard base64 SHOULD include padding
	tests := []struct {
		name string
		data []byte
	}{
		{"one byte", []byte("a")},
		{"two bytes", []byte("ab")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := ToBase64(tt.data)
			if !strings.Contains(encoded, "=") {
				t.Errorf("encoded string should contain padding: %s", encoded)
			}
		})
	}
}

func BenchmarkToBase64URL(b *testing.B) {
	data := make([]byte, 1000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ToBase64URL(data)
	}
}

func BenchmarkFromBase64URL(b *testing.B) {
	data := make([]byte, 1000)
	for i := range data {
		data[i] = byte(i % 256)
	}
	encoded := ToBase64URL(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FromBase64URL(encoded)
	}
}

// Example_base64Encoding demonstrates the two base64 encoding variants.
func Example_base64Encoding() {
	data := []byte("Hello, World!")

	// URL-safe base64 without padding (for protocol values).
	urlSafe := ToBase64URL(data)
	fmt.Printf("URL-safe: %s\n", urlSafe)

	// Standard base64 with padding (for attachments).
	standard := ToBase64(data)
	fmt.Printf("Standard: %s\n", standard)

	// Decode both variants.
	decoded1, _ := FromBase64URL(urlSafe)
	decoded2, _ := FromBase64(standard)
	fmt.Printf("Both decode to: %s\n", string(decoded1))
	fmt.Printf("Decoded match: %v\n", bytes.Equal(decoded1, decoded2))

	// Output:
	// URL-safe: SGVsbG8sIFdvcmxkIQ
	// Standard: SGVsbG8sIFdvcmxkIQ==
	// Both decode to: Hello, World!
	// Decoded match: true
}
