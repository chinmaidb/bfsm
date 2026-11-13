package main

import (
	"bytes"
	"strings"
	"testing"

	"schemeapp/backend/internal/security"
)

func TestPrintPasswordHash(t *testing.T) {
	const password = "mysecretpassword"
	var output bytes.Buffer

	if err := printPasswordHash(strings.NewReader(password+"\n"), &output); err != nil {
		t.Fatal(err)
	}
	hash := strings.TrimSpace(output.String())
	valid, err := security.VerifyPassword(hash, password)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("generated hash did not verify against input password")
	}
}

func TestPrintPasswordHashRejectsShortPassword(t *testing.T) {
	var output bytes.Buffer
	if err := printPasswordHash(strings.NewReader("short\n"), &output); err == nil {
		t.Fatal("expected short password to be rejected")
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q, want no hash", output.String())
	}
}
