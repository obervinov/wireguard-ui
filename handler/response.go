package handler

type jsonHTTPResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

// loginResponse tells the login page whether a second factor step follows
type loginResponse struct {
	Status       bool   `json:"status"`
	Message      string `json:"message"`
	TOTPRequired bool   `json:"totp_required"`
}
