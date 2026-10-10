package twofactor

import (
	"strings"
	"testing"
	"time"
)

// RFC 6238 appendix B test vectors (SHA1, secret "12345678901234567890"), truncated to 6 digits
func TestCodeMatchesRFC6238Vectors(t *testing.T) {
	secret := b32.EncodeToString([]byte("12345678901234567890"))
	for unix, want := range map[int64]string{
		59:          "287082",
		1111111109:  "081804",
		1111111111:  "050471",
		1234567890:  "005924",
		2000000000:  "279037",
		20000000000: "353130",
	} {
		got, err := Code(secret, unix/Period)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("t=%d: got %s, want %s", unix, got, want)
		}
	}
}

func TestValidateAcceptsOneStepSkewOnly(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	step := Step(now)

	for delta, ok := range map[int64]bool{-2: false, -1: true, 0: true, 1: true, 2: false} {
		code, _ := Code(secret, step+delta)
		matched, valid := Validate(secret, code, now)
		if valid != ok {
			t.Errorf("delta %d: valid=%v, want %v", delta, valid, ok)
		}
		if valid && matched != step+delta {
			t.Errorf("delta %d: matched step %d, want %d", delta, matched, step+delta)
		}
	}

	for _, bad := range []string{"", "12345", "1234567", "abcdef"} {
		if _, ok := Validate(secret, bad, now); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, hashes, err := GenerateRecoveryCodes(RecoveryCodeCount)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != RecoveryCodeCount || len(hashes) != RecoveryCodeCount {
		t.Fatalf("got %d codes, %d hashes", len(codes), len(hashes))
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if seen[c] {
			t.Fatalf("duplicate code %s", c)
		}
		seen[c] = true
	}

	// case, spaces and dashes are ignored
	typed := strings.ToUpper(strings.ReplaceAll(codes[3], "-", " "))
	if i := MatchRecoveryCode(hashes, typed); i != 3 {
		t.Fatalf("match index %d, want 3", i)
	}
	if i := MatchRecoveryCode(hashes, "nope-nope0"); i != -1 {
		t.Fatalf("unknown code matched %d", i)
	}
	if i := MatchRecoveryCode(hashes, " - "); i != -1 {
		t.Fatalf("empty code matched %d", i)
	}
}
