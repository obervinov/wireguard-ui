package model

// User model
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	// PasswordHash takes precedence over Password.
	PasswordHash string `json:"password_hash"`
	Admin        bool   `json:"admin"`
	// TOTPSecret enables the second login step when set (base32, RFC 6238).
	TOTPSecret string `json:"totp_secret,omitempty"`
	// RecoveryCodes holds SHA-256 hashes of the unused one-time recovery codes.
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
}

// TOTPEnabled reports whether the user has to pass the second login step
func (u User) TOTPEnabled() bool {
	return u.TOTPSecret != ""
}

// UserView is a user as returned by the API: no password or second factor secrets
type UserView struct {
	Username    string `json:"username"`
	Admin       bool   `json:"admin"`
	TOTPEnabled bool   `json:"totp_enabled"`
	// RecoveryCodesLeft is the number of unused recovery codes
	RecoveryCodesLeft int `json:"recovery_codes_left"`
}

// View returns the API representation of the user
func (u User) View() UserView {
	return UserView{
		Username:          u.Username,
		Admin:             u.Admin,
		TOTPEnabled:       u.TOTPEnabled(),
		RecoveryCodesLeft: len(u.RecoveryCodes),
	}
}
