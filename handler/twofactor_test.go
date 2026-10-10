package handler

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/ngoduykhanh/wireguard-ui/store/jsondb"
	"github.com/ngoduykhanh/wireguard-ui/twofactor"
)

type loginEnv struct {
	t      *testing.T
	db     *jsondb.JsonDB
	server *httptest.Server
	client *http.Client
}

// newLoginEnv serves the login routes and one protected API route, with a
// TOTP-enabled admin (password "admin") and the given recovery codes.
func newLoginEnv(t *testing.T, secret string, recoveryHashes []string) *loginEnv {
	t.Helper()
	t.Setenv("WGUI_ENDPOINT_ADDRESS", "wg.example.com")
	guard = &codeGuard{failures: map[string]int{}, lockedUntil: map[string]time.Time{}, lastStep: map[string]int64{}}

	db, err := jsondb.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Init(); err != nil {
		t.Fatal(err)
	}
	user, err := db.GetUserByName("admin")
	if err != nil {
		t.Fatal(err)
	}
	user.TOTPSecret = secret
	user.RecoveryCodes = recoveryHashes
	if err := db.SaveUser(user); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	e.Use(session.Middleware(sessions.NewCookieStore([]byte(strings.Repeat("a", 32)), []byte(strings.Repeat("b", 32)))))
	e.POST("/login", Login(db))
	e.POST("/login/2fa", LoginTOTP(db))
	e.GET("/api/clients", GetClients(db), ValidSession)
	server := httptest.NewServer(e)
	t.Cleanup(server.Close)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &loginEnv{t: t, db: db, server: server, client: client}
}

func (e *loginEnv) post(path, body string) (int, map[string]interface{}) {
	e.t.Helper()
	resp, err := e.client.Post(e.server.URL+path, echo.MIMEApplicationJSON, strings.NewReader(body))
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	out := map[string]interface{}{}
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (e *loginEnv) apiStatus() int {
	e.t.Helper()
	resp, err := e.client.Get(e.server.URL + "/api/clients")
	if err != nil {
		e.t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func (e *loginEnv) password() {
	e.t.Helper()
	status, out := e.post("/login", `{"username":"admin","password":"admin"}`)
	if status != http.StatusOK || out["totp_required"] != true {
		e.t.Fatalf("password step: %d %v", status, out)
	}
}

func currentCode(t *testing.T, secret string) string {
	code, err := twofactor.Code(secret, twofactor.Step(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestLoginWithTOTPNeedsTheCodeStep(t *testing.T) {
	secret, _ := twofactor.GenerateSecret()
	env := newLoginEnv(t, secret, nil)

	env.password()
	if got := env.apiStatus(); got == http.StatusOK {
		t.Fatalf("password alone opened /api/clients")
	}

	if status, _ := env.post("/login/2fa", `{"code":"000000"}`); status != http.StatusUnauthorized {
		t.Fatalf("wrong code: got %d", status)
	}
	if got := env.apiStatus(); got == http.StatusOK {
		t.Fatalf("wrong code opened /api/clients")
	}

	code := currentCode(t, secret)
	if status, out := env.post("/login/2fa", `{"code":"`+code+`"}`); status != http.StatusOK {
		t.Fatalf("right code: %d %v", status, out)
	}
	if got := env.apiStatus(); got != http.StatusOK {
		t.Fatalf("after the code step /api/clients returned %d", got)
	}

	// the same code cannot sign in a second time
	env.client.Jar, _ = cookiejar.New(nil)
	env.password()
	if status, _ := env.post("/login/2fa", `{"code":"`+code+`"}`); status == http.StatusOK {
		t.Fatalf("replayed code accepted")
	}
}

func TestLoginTOTPWithoutPasswordIsRejected(t *testing.T) {
	secret, _ := twofactor.GenerateSecret()
	env := newLoginEnv(t, secret, nil)

	if status, _ := env.post("/login/2fa", `{"code":"`+currentCode(t, secret)+`"}`); status != http.StatusUnauthorized {
		t.Fatalf("code without password: got %d", status)
	}
	if got := env.apiStatus(); got == http.StatusOK {
		t.Fatalf("code without password opened /api/clients")
	}
}

func TestLoginTOTPLocksOutAfterFiveWrongCodes(t *testing.T) {
	secret, _ := twofactor.GenerateSecret()
	env := newLoginEnv(t, secret, nil)

	env.password()
	for i := 1; i < maxCodeFailures; i++ {
		if status, _ := env.post("/login/2fa", `{"code":"000000"}`); status != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d", i, status)
		}
	}
	if status, _ := env.post("/login/2fa", `{"code":"000000"}`); status != http.StatusTooManyRequests {
		t.Fatalf("attempt %d: got %d, want 429", maxCodeFailures, status)
	}

	// the pending state is gone and the user is locked, even with the password again
	if status, _ := env.post("/login/2fa", `{"code":"`+currentCode(t, secret)+`"}`); status == http.StatusOK {
		t.Fatalf("right code accepted after the lockout")
	}
	env.password()
	if status, _ := env.post("/login/2fa", `{"code":"`+currentCode(t, secret)+`"}`); status != http.StatusTooManyRequests {
		t.Fatalf("locked user: got %d, want 429", status)
	}
}

func TestLoginWithRecoveryCodeConsumesIt(t *testing.T) {
	secret, _ := twofactor.GenerateSecret()
	codes, hashes, err := twofactor.GenerateRecoveryCodes(2)
	if err != nil {
		t.Fatal(err)
	}
	env := newLoginEnv(t, secret, hashes)

	env.password()
	if status, out := env.post("/login/2fa", `{"code":"`+codes[0]+`"}`); status != http.StatusOK {
		t.Fatalf("recovery code: %d %v", status, out)
	}
	if got := env.apiStatus(); got != http.StatusOK {
		t.Fatalf("after a recovery code /api/clients returned %d", got)
	}

	user, _ := env.db.GetUserByName("admin")
	if len(user.RecoveryCodes) != 1 || user.RecoveryCodes[0] != hashes[1] {
		t.Fatalf("recovery codes after use: %v", user.RecoveryCodes)
	}

	env.client.Jar, _ = cookiejar.New(nil)
	env.password()
	if status, _ := env.post("/login/2fa", `{"code":"`+codes[0]+`"}`); status == http.StatusOK {
		t.Fatalf("used recovery code accepted again")
	}
}

func TestLoginWithoutTOTPSkipsTheCodeStep(t *testing.T) {
	env := newLoginEnv(t, "", nil)

	status, out := env.post("/login", `{"username":"admin","password":"admin"}`)
	if status != http.StatusOK || out["totp_required"] != nil {
		t.Fatalf("password step: %d %v", status, out)
	}
	if got := env.apiStatus(); got != http.StatusOK {
		t.Fatalf("/api/clients returned %d", got)
	}
}

func TestUserAPIHidesSecrets(t *testing.T) {
	_, hashes, _ := twofactor.GenerateRecoveryCodes(3)
	env := newLoginEnv(t, "JBSWY3DPEHPK3PXP", hashes)
	user, _ := env.db.GetUserByName("admin")

	raw, err := json.Marshal(user.View())
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"JBSWY3DPEHPK3PXP", hashes[0], user.PasswordHash, "password", "totp_secret", "recovery_codes\""} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("user view leaks %q: %s", leak, raw)
		}
	}
	if !strings.Contains(string(raw), `"totp_enabled":true`) || !strings.Contains(string(raw), `"recovery_codes_left":3`) {
		t.Errorf("user view misses the 2FA state: %s", raw)
	}
}
