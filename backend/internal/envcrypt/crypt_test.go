package envcrypt

import "testing"

func TestSealOpenRoundTrip(t *testing.T) {
	b, err := New("test-secret-for-env-crypt")
	if err != nil {
		t.Fatal(err)
	}
	ct, err := b.Seal("FINNHUB_API_KEY=abc123xyz")
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Open(ct)
	if err != nil || got != "FINNHUB_API_KEY=abc123xyz" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := b.Open([]byte("nope")); err == nil {
		t.Fatal("expected error")
	}
}
