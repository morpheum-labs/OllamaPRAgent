package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifyGitHubSignature256(t *testing.T) {
	t.Parallel()
	secret := "whsec_test"
	payload := []byte(`{"hook":true}`)
	m := hmac.New(sha256.New, []byte(secret))
	_, _ = m.Write(payload)
	mac := "sha256=" + hex.EncodeToString(m.Sum(nil))
	if !verifyGitHubSignature256(payload, secret, mac) {
		t.Fatal("expected valid signature")
	}
	if verifyGitHubSignature256(payload, secret, "sha256=deadbeef") {
		t.Fatal("expected invalid signature")
	}
	if verifyGitHubSignature256(payload, "", mac) {
		t.Fatal("expected reject empty secret")
	}
}
