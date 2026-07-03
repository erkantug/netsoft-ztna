package idp

import (
	"fmt"
)

type ProviderType string

const (
	ProviderLocal ProviderType = "local"
	ProviderEntra ProviderType = "entra"
)

type ProviderConfig struct {
	Type          ProviderType `json:"type"`
	TenantID      string       `json:"tenant_id,omitempty"`
	ClientID      string       `json:"client_id,omitempty"`
	ClientSecret  string       `json:"client_secret,omitempty"`
	AllowedDomain string       `json:"allowed_domain,omitempty"`
}

func (p *ProviderConfig) Validate() error {
	switch p.Type {
	case ProviderLocal:
		return nil
	case ProviderEntra:
		if p.TenantID == "" {
			return fmt.Errorf("tenant ID is required for Entra ID")
		}
		if p.ClientID == "" {
			return fmt.Errorf("client ID is required for Entra ID")
		}
		if p.ClientSecret == "" {
			return fmt.Errorf("client secret is required for Entra ID")
		}
		return nil
	default:
		return fmt.Errorf("unknown provider: %s", p.Type)
	}
}

func (p *ProviderConfig) Issuer() string {
	if p.Type == ProviderEntra {
		return fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", p.TenantID)
	}
	return ""
}
