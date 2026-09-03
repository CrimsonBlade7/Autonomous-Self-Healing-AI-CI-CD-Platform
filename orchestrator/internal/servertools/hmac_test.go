package servertools

import (
	"testing"
	"time"
)

func TestGenerateAndVerifyHMAC(t *testing.T) {
	t.Parallel()

	msg := []byte(`{"number":1}`)
	sig, err := generateHMAC(msg, "secret")
	if err != nil {
		t.Fatalf("generateHMAC: %v", err)
	}
	if sig == "" {
		t.Fatal("empty signature")
	}

	ok, err := verifyMessage(msg, "secret", sig)
	if err != nil {
		t.Fatalf("verifyMessage: %v", err)
	}
	if !ok {
		t.Fatal("expected valid signature")
	}

	ok, err = verifyMessage(msg, "secret", "deadbeef")
	if err != nil {
		t.Fatalf("verifyMessage: %v", err)
	}
	if ok {
		t.Fatal("expected invalid signature")
	}

	ok, err = verifyMessage(msg, "other", sig)
	if err != nil {
		t.Fatalf("verifyMessage: %v", err)
	}
	if ok {
		t.Fatal("expected mismatch for different secret")
	}
}

func TestSeconds(t *testing.T) {
	t.Parallel()
	if got := seconds(3); got != 3*time.Second {
		t.Errorf("seconds(3) = %v", got)
	}
}
