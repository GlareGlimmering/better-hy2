package vo

type JwtVo struct {
	PasswordChangeRequired bool   `json:"passwordChangeRequired,omitempty"`
	TokenType              string `json:"tokenType,omitempty"`
	AccessToken            string `json:"accessToken,omitempty"`
}
