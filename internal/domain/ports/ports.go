package ports

type Config struct {
	Environment string // "dev" or "prod"
	AppURL      string // e.g. "http://localhost:8080"
	Addr        string // e.g. ":8080"

	DatabaseURL string

	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCLogoutURL    string // optional: omit to skip provider-side logout

	SecretKey []byte // 32-byte key for token encryption and CSRF protection
}
