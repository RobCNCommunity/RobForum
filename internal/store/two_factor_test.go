package store

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestValidateTOTPUsesAuthenticatorCompatibleWindow(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	now := time.Unix(1760000000, 0).UTC()
	code, err := totp.GenerateCodeCustom(secret, now, totp.ValidateOpts{Period: 30, Skew: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !validateTOTP(secret, code, now) {
		t.Fatal("valid six-digit TOTP was rejected")
	}
	if validateTOTP(secret, "000000", now) {
		t.Fatal("invalid TOTP was accepted")
	}
}
