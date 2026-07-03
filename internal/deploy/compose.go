package deploy

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/netsoft/ztna-wizard/internal/db"
)

type ComposeData struct {
	Domain           string
	DatabasePassword string
	SSLKeyPath       string
	SSLCertPath      string
	OIDCEnabled      string
	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	AuthAuthority    string
}

const composeTemplate = `version: "3.8"
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: netbird
      POSTGRES_USER: netbird
      POSTGRES_PASSWORD: ${DATABASE_PASSWORD}
    volumes:
      - netbird_db:/var/lib/postgresql/data
    restart: unless-stopped
    networks:
      - netbird

  management:
    image: netbirdio/management:latest
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "443:443"
    environment:
      NETBIRD_DOMAIN: ${NETBIRD_DOMAIN}
      NETBIRD_DATABASE_HOST: postgres
      NETBIRD_DATABASE_PORT: 5432
      NETBIRD_DATABASE_NAME: netbird
      NETBIRD_DATABASE_USER: netbird
      NETBIRD_DATABASE_PASS: ${DATABASE_PASSWORD}
      NETBIRD_AUTH_OIDC_ENABLED: ${OIDC_ENABLED}
      NETBIRD_AUTH_OIDC_ISSUER: ${OIDC_ISSUER}
      NETBIRD_AUTH_OIDC_CLIENT_ID: ${OIDC_CLIENT_ID}
      NETBIRD_AUTH_OIDC_CLIENT_SECRET: ${OIDC_CLIENT_SECRET}
    volumes:
      - ${SSL_CERT_PATH}:/etc/ssl/certs/netbird:ro
      - ${SSL_KEY_PATH}:/etc/ssl/private/netbird:ro
    healthcheck:
      test: ["CMD", "wget", "--no-check-certificate", "-qO-", "https://localhost:443/management/health"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped
    networks:
      - netbird

  dashboard:
    image: netbirdio/dashboard:latest
    depends_on:
      management:
        condition: service_healthy
    ports:
      - "80:80"
    environment:
      NETBIRD_MANAGEMENT_API: https://${NETBIRD_DOMAIN}:443
      NETBIRD_AUTH_AUTHORITY: ${AUTH_AUTHORITY}
      NETBIRD_AUTH_CLIENT_ID: ${OIDC_CLIENT_ID}
      NETBIRD_AUTH_AUDIENCE: ${NETBIRD_DOMAIN}
      NETBIRD_AUTH_REDIRECT_URI: https://${NETBIRD_DOMAIN}
    restart: unless-stopped
    networks:
      - netbird

  signal:
    image: netbirdio/signal:latest
    ports:
      - "10000:10000"
    restart: unless-stopped
    networks:
      - netbird

  coturn:
    image: coturn/coturn:latest
    ports:
      - "3478:3478"
      - "3478:3478/udp"
      - "49152-65535:49152-65535/udp"
    command: >
      -n --listening-port 3478
      --min-port 49152 --max-port 65535
      --realm ${NETBIRD_DOMAIN}
      --fingerprint --lt-cred-mech
      --no-auth --no-tls --no-dtls
    restart: unless-stopped
    networks:
      - netbird

volumes:
  netbird_db:

networks:
  netbird:
    driver: bridge
`

func GenerateCompose(data *ComposeData) (string, error) {
	tmpl, err := template.New("compose").Parse(composeTemplate)
	if err != nil {
		return "", fmt.Errorf("parse compose template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute compose template: %w", err)
	}
	return buf.String(), nil
}

func ComposeDataFromWizard(all *db.AllData, dbPass string) *ComposeData {
	d := &ComposeData{
		Domain:           all.Network.Domain,
		DatabasePassword: dbPass,
		SSLKeyPath:       "/etc/netsoft/certs/privkey.pem",
		SSLCertPath:      "/etc/netsoft/certs/fullchain.pem",
		OIDCEnabled:      "false",
		OIDCIssuer:       "",
		OIDCClientID:     "",
		OIDCClientSecret: "",
		AuthAuthority:    fmt.Sprintf("https://%s", all.Network.Domain),
	}
	if all.IDP != nil && all.IDP.Provider == "entra" {
		d.OIDCEnabled = "true"
		d.OIDCIssuer = fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", all.IDP.TenantID)
		d.OIDCClientID = all.IDP.ClientID
		d.OIDCClientSecret = all.IDP.ClientSecret
	}
	return d
}
