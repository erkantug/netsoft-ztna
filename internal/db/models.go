package db

type Step int

const (
	StepNetwork Step = iota + 1
	StepTLS
	StepAdmin
	StepIDP
	StepDeploy
)

var StepNames = map[Step]string{
	StepNetwork: "Network",
	StepTLS:     "TLS",
	StepAdmin:   "Admin",
	StepIDP:     "Identity Provider",
	StepDeploy:  "Deploy",
}

// Step data structures stored as JSON in SQLite

type NetworkData struct {
	Domain    string `json:"domain"`
	IP        string `json:"ip"`
	DNS       string `json:"dns"`
	NTP       string `json:"ntp"`
	UseDHCP   bool   `json:"use_dhcp"`
}

type TLSData struct {
	Method      string `json:"method"` // "letsencrypt", "custom", "selfsigned"
	Email       string `json:"email,omitempty"`
	CertPEM     string `json:"cert_pem,omitempty"`
	KeyPEM      string `json:"key_pem,omitempty"`
	AcmeTermsAgreed bool `json:"acme_terms_agreed"`
}

type AdminData struct {
	Username string `json:"username"`
	Password string `json:"password_hash"`
	Email    string `json:"email"`
}

type IDPData struct {
	Provider     string `json:"provider"` // "local", "entra"
	TenantID    string `json:"tenant_id,omitempty"`
	ClientID    string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	AllowedDomain string `json:"allowed_domain,omitempty"`
}

type AllData struct {
	Network *NetworkData `json:"network,omitempty"`
	TLS     *TLSData     `json:"tls,omitempty"`
	Admin   *AdminData   `json:"admin,omitempty"`
	IDP     *IDPData     `json:"idp,omitempty"`
	Deployed bool        `json:"deployed"`
}
