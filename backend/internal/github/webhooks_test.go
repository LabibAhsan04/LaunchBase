package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifyWebhookSignature(t *testing.T) {
	body := []byte(`{"action":"created"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if err := VerifyWebhookSignature("secret", body, sig); err != nil {
		t.Fatal(err)
	}
	if err := VerifyWebhookSignature("secret", body, "sha256=deadbeef"); err == nil {
		t.Fatal("expected mismatch")
	}
	if err := VerifyWebhookSignature("secret", body, ""); err == nil {
		t.Fatal("expected missing")
	}
}
