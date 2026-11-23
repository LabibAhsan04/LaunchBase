package envcrypt

import "testing"

func TestValidateKey(t *testing.T) {
	if err := ValidateKey("FINNHUB_API_KEY"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "lowercase", "1BAD", "DATABASE_URL", "LAUNCHBASE_X", "path"} {
		if err := ValidateKey(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestMask(t *testing.T) {
	if Mask("ab") != "••••" {
		t.Fatal(Mask("ab"))
	}
	if Mask("secretvalue") != "••••alue" {
		t.Fatal(Mask("secretvalue"))
	}
}
