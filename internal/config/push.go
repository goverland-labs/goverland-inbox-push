package config

type Push struct {
	Type                string `env:"PUSH_TYPE" json:"type"`
	ProjectID           string `env:"PUSH_PROJECT_ID" json:"project_id"`
	PrivateKeyID        string `env:"PUSH_PRIVATE_KEY_ID" json:"private_key_id"`
	PrivateKey          string `env:"PUSH_PRIVATE_KEY" json:"private_key"`
	ClientEmail         string `env:"PUSH_CLIENT_EMAIL" json:"client_email"`
	ClientID            string `env:"PUSH_CLIENT_ID" json:"client_id"`
	AuthUri             string `env:"PUSH_AUTH_URI" json:"auth_uri"`
	TokenUri            string `env:"PUSH_TOKEN_URI" json:"token_uri"`
	AuthProviderCertURL string `env:"PUSH_AUTH_PROVIDER_CERT_URL" json:"auth_provider_x509_cert_url"`
	ClientCertURL       string `env:"PUSH_CLIENT_CERT_URL" json:"client_x509_cert_url"`
	UniverseDomain      string `env:"PUSH_UNIVERSE_DOMAIN" json:"universe_domain"`
}
