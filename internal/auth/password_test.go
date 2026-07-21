package auth

import "testing"

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("correct-horse-battery"); err != nil {
		t.Fatalf("valid password rejected: %v", err)
	}
	if err := ValidatePassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	tooLong := make([]byte, 73)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	if err := ValidatePassword(string(tooLong)); err == nil {
		t.Fatal("bcrypt-incompatible password accepted")
	}
}
