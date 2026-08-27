package guacamole

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

const testKey = "4c0b569e4c96df157eee1b65dd0e4d41"

func TestBuildToken(t *testing.T) {
	payload := JSONAuthPayload{
		Username: "testuser",
		Expires:  "1735689600000",
		Connections: map[string]JSONAuthConnection{
			"my-desktop": {
				Protocol: "rdp",
				Parameters: map[string]string{
					"hostname": "10.0.0.1",
					"port":     "3389",
					"username": "ubuntu",
					"password": "ubuntu",
				},
			},
		},
	}

	token, err := BuildToken(payload, testKey)
	if err != nil {
		t.Fatalf("BuildToken() error: %v", err)
	}
	if token == "" {
		t.Fatal("BuildToken() returned empty token")
	}

	// Verify by decrypting
	key, _ := hex.DecodeString(testKey)
	ciphertext, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("base64 decode error: %v", err)
	}

	block, _ := aes.NewCipher(key)
	iv := make([]byte, aes.BlockSize)
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// Remove PKCS7 padding
	padLen := int(ciphertext[len(ciphertext)-1])
	plaintext := ciphertext[:len(ciphertext)-padLen]

	// First 32 bytes are HMAC-SHA256 signature
	sig := plaintext[:32]
	jsonBytes := plaintext[32:]

	mac := hmac.New(sha256.New, key)
	mac.Write(jsonBytes)
	expectedSig := mac.Sum(nil)
	if !hmac.Equal(sig, expectedSig) {
		t.Error("HMAC signature mismatch")
	}

	var decoded JSONAuthPayload
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if decoded.Username != "testuser" {
		t.Errorf("Username = %q, want %q", decoded.Username, "testuser")
	}
	if len(decoded.Connections) != 1 {
		t.Errorf("Connections count = %d, want 1", len(decoded.Connections))
	}
}

func TestBuildToken_InvalidKey(t *testing.T) {
	payload := JSONAuthPayload{Username: "test"}
	_, err := BuildToken(payload, "not-hex")
	if err == nil {
		t.Error("BuildToken() should error on invalid hex key")
	}
}
