package auth

import "testing"

func TestEncryptDecryptAndMask(t *testing.T) {
	sealed, err := Encrypt("master-key", "smtp-password")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if sealed == "smtp-password" || sealed[:7] != "enc:v1:" {
		t.Fatalf("unexpected ciphertext: %q", sealed)
	}
	plain, err := Decrypt("master-key", sealed)
	if err != nil || plain != "smtp-password" {
		t.Fatalf("Decrypt() = %q, %v", plain, err)
	}
	if _, err := Decrypt("wrong-key", sealed); err == nil {
		t.Fatal("Decrypt() accepted the wrong key")
	}
	if Mask("smtp-password") == "smtp-password" {
		t.Fatal("Mask() exposed the secret")
	}
	if got := Mask("密码安全值"); got != "密码••••全值" {
		t.Fatalf("Mask() broke UTF-8 text: %q", got)
	}
}
