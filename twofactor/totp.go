// Package twofactor implements TOTP (RFC 6238) second factor and one-time recovery
// codes for web UI users.
package twofactor

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// Period is the TOTP time step in seconds
	Period = 30
	// Digits is the length of a TOTP code
	Digits = 6
	// Skew is how many steps before and after the current one are accepted
	Skew = 1
	// RecoveryCodeCount is how many recovery codes are issued on enrollment
	RecoveryCodeCount = 8
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret returns a new random base32 TOTP secret (160 bits, as RFC 4226 recommends)
func GenerateSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return b32.EncodeToString(buf), nil
}

// URI returns the otpauth:// URI that authenticator apps read from a QR code
func URI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", fmt.Sprint(Digits))
	v.Set("period", fmt.Sprint(Period))
	return "otpauth://totp/" + label + "?" + v.Encode()
}

// Code returns the TOTP code of the given secret for the time step
func Code(secret string, step int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.TrimRight(secret, "=")))
	if err != nil {
		return "", fmt.Errorf("invalid TOTP secret: %w", err)
	}
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", Digits, value%1_000_000), nil
}

// Step returns the TOTP time step of t
func Step(t time.Time) int64 {
	return t.Unix() / Period
}

// Validate checks a code against the secret at time t, accepting Skew steps around it.
// It returns the matched step, so the caller can refuse a code that was already used.
func Validate(secret, code string, t time.Time) (int64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != Digits {
		return 0, false
	}
	current := Step(t)
	for delta := int64(-Skew); delta <= Skew; delta++ {
		want, err := Code(secret, current+delta)
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return current + delta, true
		}
	}
	return 0, false
}

// GenerateRecoveryCodes returns n codes to show the user once, and their hashes to store
func GenerateRecoveryCodes(n int) (codes []string, hashes []string, err error) {
	for i := 0; i < n; i++ {
		buf := make([]byte, 7)
		if _, err := rand.Read(buf); err != nil {
			return nil, nil, err
		}
		raw := strings.ToLower(b32.EncodeToString(buf))[:10]
		codes = append(codes, raw[:5]+"-"+raw[5:])
		hashes = append(hashes, HashRecoveryCode(raw))
	}
	return codes, hashes, nil
}

// HashRecoveryCode hashes a recovery code, ignoring case, spaces and dashes.
// The codes carry 50 random bits, so a plain SHA-256 is enough.
func HashRecoveryCode(code string) string {
	sum := sha256.Sum256([]byte(normalizeRecoveryCode(code)))
	return hex.EncodeToString(sum[:])
}

// MatchRecoveryCode returns the index of the hash that matches code, or -1
func MatchRecoveryCode(hashes []string, code string) int {
	if normalizeRecoveryCode(code) == "" {
		return -1
	}
	want := HashRecoveryCode(code)
	match := -1
	for i, h := range hashes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(want)) == 1 {
			match = i
		}
	}
	return match
}

func normalizeRecoveryCode(code string) string {
	code = strings.ToLower(code)
	return strings.NewReplacer(" ", "", "-", "", "\t", "").Replace(code)
}
