package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/ngoduykhanh/wireguard-ui/model"
	"github.com/ngoduykhanh/wireguard-ui/store"
	"github.com/ngoduykhanh/wireguard-ui/twofactor"
	"github.com/ngoduykhanh/wireguard-ui/util"
	"github.com/skip2/go-qrcode"
)

const (
	totpIssuer = "WireGuard UI"
	// maxCodeFailures failed codes in a row lock the user's code step for codeLockout
	maxCodeFailures = 5
	codeLockout     = 5 * time.Minute
)

// codeGuard keeps the per-user state that must not live in the session cookie,
// where an attacker could replay an older copy: failure counts, lockouts and the
// last accepted TOTP step (a code is accepted once).
type codeGuard struct {
	mu          sync.Mutex
	failures    map[string]int
	lockedUntil map[string]time.Time
	lastStep    map[string]int64
}

var guard = &codeGuard{
	failures:    map[string]int{},
	lockedUntil: map[string]time.Time{},
	lastStep:    map[string]int64{},
}

func (g *codeGuard) locked(username string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return time.Now().Before(g.lockedUntil[username])
}

// fail records a wrong code and reports whether the user is now locked out
func (g *codeGuard) fail(username string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failures[username]++
	if g.failures[username] >= maxCodeFailures {
		g.failures[username] = 0
		g.lockedUntil[username] = time.Now().Add(codeLockout)
		return true
	}
	return false
}

func (g *codeGuard) succeed(username string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.failures, username)
	delete(g.lockedUntil, username)
}

// acceptStep refuses a TOTP step that was already used, or an older one
func (g *codeGuard) acceptStep(username string, step int64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if step <= g.lastStep[username] {
		return false
	}
	g.lastStep[username] = step
	return true
}

// verifySecondFactor checks a TOTP code or a recovery code for the user. A used
// recovery code is removed and the user saved; the returned user reflects that.
func verifySecondFactor(db store.IStore, user model.User, code string) (model.User, bool, error) {
	if step, ok := twofactor.Validate(user.TOTPSecret, code, time.Now()); ok {
		return user, guard.acceptStep(user.Username, step), nil
	}
	if i := twofactor.MatchRecoveryCode(user.RecoveryCodes, code); i >= 0 {
		user.RecoveryCodes = append(append([]string{}, user.RecoveryCodes[:i]...), user.RecoveryCodes[i+1:]...)
		if err := db.SaveUser(user); err != nil {
			return user, false, err
		}
		log.Infof("User %s signed in with a recovery code, %d left", user.Username, len(user.RecoveryCodes))
		return user, true, nil
	}
	return user, false, nil
}

func readCode(c echo.Context) (string, bool) {
	var input struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&input); err != nil || input.Code == "" {
		return "", false
	}
	return input.Code, true
}

// LoginTOTP handler is the second login step: a TOTP or recovery code after the password
func LoginTOTP(db store.IStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		username, rememberMe, ok := pendingLogin(c)
		if !ok {
			return c.JSON(http.StatusUnauthorized, jsonHTTPResponse{false, "Sign-in expired, enter your password again"})
		}
		if guard.locked(username) {
			dropPendingLogin(c)
			return c.JSON(http.StatusTooManyRequests, jsonHTTPResponse{false, "Too many wrong codes, try again in a few minutes"})
		}
		code, ok := readCode(c)
		if !ok {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Please enter a code"})
		}

		user, err := db.GetUserByName(username)
		if err != nil || !user.TOTPEnabled() {
			dropPendingLogin(c)
			return c.JSON(http.StatusUnauthorized, jsonHTTPResponse{false, "Sign-in expired, enter your password again"})
		}

		user, valid, err := verifySecondFactor(db, user, code)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, "Cannot save user"})
		}
		if !valid {
			if guard.fail(username) {
				dropPendingLogin(c)
				return c.JSON(http.StatusTooManyRequests, jsonHTTPResponse{false, "Too many wrong codes, try again in a few minutes"})
			}
			return c.JSON(http.StatusUnauthorized, jsonHTTPResponse{false, "Invalid code"})
		}

		guard.succeed(username)
		startSession(c, user, rememberMe)
		return c.JSON(http.StatusOK, jsonHTTPResponse{true, "Logged in successfully"})
	}
}

// TwoFactorSetup handler starts enrollment: a new secret, kept in the session until confirmed
func TwoFactorSetup(db store.IStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		if util.DisableLogin {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Login is disabled"})
		}
		user, err := db.GetUserByName(currentUser(c))
		if err != nil {
			return c.JSON(http.StatusNotFound, jsonHTTPResponse{false, "User not found"})
		}
		if user.TOTPEnabled() {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Two-factor authentication is already enabled"})
		}

		secret, err := twofactor.GenerateSecret()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, "Cannot generate a secret"})
		}
		uri := twofactor.URI(totpIssuer, user.Username, secret)
		png, err := qrcode.Encode(uri, qrcode.Medium, 256)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, "Cannot render the QR code"})
		}

		sess, _ := session.Get("session", c)
		sess.Values["totp_setup_secret"] = secret
		sess.Save(c.Request(), c.Response())

		return c.JSON(http.StatusOK, map[string]string{
			"secret": secret,
			"uri":    uri,
			"qr":     "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		})
	}
}

// TwoFactorEnable handler confirms enrollment with a code and returns the recovery codes, once
func TwoFactorEnable(db store.IStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		if util.DisableLogin {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Login is disabled"})
		}
		code, ok := readCode(c)
		if !ok {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Please enter a code"})
		}

		sess, _ := session.Get("session", c)
		secret, _ := sess.Values["totp_setup_secret"].(string)
		if secret == "" {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Start the setup again"})
		}

		user, err := db.GetUserByName(currentUser(c))
		if err != nil {
			return c.JSON(http.StatusNotFound, jsonHTTPResponse{false, "User not found"})
		}
		if user.TOTPEnabled() {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Two-factor authentication is already enabled"})
		}
		step, valid := twofactor.Validate(secret, code, time.Now())
		if !valid {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Invalid code, check the time on your phone"})
		}
		guard.acceptStep(user.Username, step)

		codes, hashes, err := twofactor.GenerateRecoveryCodes(twofactor.RecoveryCodeCount)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, "Cannot generate recovery codes"})
		}
		user.TOTPSecret = secret
		user.RecoveryCodes = hashes
		if err := db.SaveUser(user); err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, "Cannot save user"})
		}

		// the user record changed: keep this session, other sessions of the user end
		delete(sess.Values, "totp_setup_secret")
		sess.Save(c.Request(), c.Response())
		setUser(c, user.Username, user.Admin, util.GetDBUserCRC32(user))
		log.Infof("User %s enabled two-factor authentication", user.Username)

		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":         true,
			"message":        "Two-factor authentication enabled",
			"recovery_codes": codes,
		})
	}
}

// TwoFactorDisable handler turns the second factor off; it takes a current code or a recovery code
func TwoFactorDisable(db store.IStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		if util.DisableLogin {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Login is disabled"})
		}
		code, ok := readCode(c)
		if !ok {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Please enter a code"})
		}

		user, err := db.GetUserByName(currentUser(c))
		if err != nil {
			return c.JSON(http.StatusNotFound, jsonHTTPResponse{false, "User not found"})
		}
		if !user.TOTPEnabled() {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Two-factor authentication is not enabled"})
		}
		if guard.locked(user.Username) {
			return c.JSON(http.StatusTooManyRequests, jsonHTTPResponse{false, "Too many wrong codes, try again in a few minutes"})
		}

		user, valid, err := verifySecondFactor(db, user, code)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, "Cannot save user"})
		}
		if !valid {
			guard.fail(user.Username)
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Invalid code"})
		}
		guard.succeed(user.Username)

		user.TOTPSecret = ""
		user.RecoveryCodes = nil
		if err := db.SaveUser(user); err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, "Cannot save user"})
		}
		setUser(c, user.Username, user.Admin, util.GetDBUserCRC32(user))
		log.Infof("User %s disabled two-factor authentication", user.Username)

		return c.JSON(http.StatusOK, jsonHTTPResponse{true, "Two-factor authentication disabled"})
	}
}

// TwoFactorReset handler lets an admin remove the second factor of another user,
// e.g. after a lost phone and lost recovery codes
func TwoFactorReset(db store.IStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		var input struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(c.Request().Body).Decode(&input); err != nil || !usernameRegexp.MatchString(input.Username) {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Please provide a valid username"})
		}
		if input.Username == currentUser(c) {
			return c.JSON(http.StatusForbidden, jsonHTTPResponse{false, "Use your profile page to turn off your own two-factor authentication"})
		}

		user, err := db.GetUserByName(input.Username)
		if err != nil {
			return c.JSON(http.StatusNotFound, jsonHTTPResponse{false, "User not found"})
		}
		user.TOTPSecret = ""
		user.RecoveryCodes = nil
		if err := db.SaveUser(user); err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, "Cannot save user"})
		}
		guard.succeed(user.Username)
		log.Infof("Admin %s reset two-factor authentication of %s", currentUser(c), user.Username)

		return c.JSON(http.StatusOK, jsonHTTPResponse{true, "Two-factor authentication reset"})
	}
}
