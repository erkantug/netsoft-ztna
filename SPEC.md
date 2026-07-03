# Netsoft ZTNA Virtual Appliance — Specification

## 1. Overview

**Netsoft ZTNA** is a virtual appliance that simplifies deploying a NetBird Zero Trust Network Access (ZTNA) self-hosted management server. It provides a 5-step web-based setup wizard, Docker-based service orchestration, and first-class support for both local and Entra ID (Azure AD) authentication.

### 1.1 Goals
- Reduce NetBird on-prem setup from hours to minutes
- Eliminate manual Docker Compose and certificate management
- Provide a turnkey virtual appliance for VMware environments
- Support enterprise identity providers (Entra ID as primary, others later)

### 1.2 Non-Goals (MVP)
- High availability / clustering
- Monitoring and observability integration
- Automated backup/restore
- Bare-metal ISO or cloud images (post-MVP)

---

## 2. Technical Architecture

### 2.1 Stack Decisions

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Base OS | Ubuntu 24.04 LTS (minimal) | Long-term support until 2029, wide driver support |
| Container Engine | Docker CE + Compose Plugin | Industry standard, NetBird official images |
| Wizard Backend | Go 1.22+ | Single binary, NetBird codebase alignment |
| Wizard Frontend | Go html/template + HTMX + Tailwind CSS | Zero Node.js build step, single binary delivery |
| Wizard State | SQLite (via modernc.org/sqlite) | No external dependency, CGo-free |
| NetBird Database | PostgreSQL 16 (Alpine image) | NetBird requirement, lightweight container |
| TLS Automation | Let's Encrypt (ACME) via certbot or lego | Free, automated certificate lifecycle |
| OVA Build | HashiCorp Packer + VMware ISO builder | Reproducible builds, CI-friendly |

### 2.2 Services

| Service | Image | Purpose |
|---------|-------|---------|
| Management | `netbirdio/management:latest` | gRPC API, peer/auth/mesh management |
| Dashboard | `netbirdio/dashboard:latest` | React web UI (port 80/443) |
| Signal | `netbirdio/signal:latest` | WebRTC signaling for NAT traversal |
| Coturn | `coturn/coturn:latest` | TURN relay for peers behind symmetric NAT |
| PostgreSQL | `postgres:16-alpine` | Management database |

### 2.3 Port Map

| Port | Service | Purpose |
|------|---------|---------|
| 8080 | Wizard UI | Setup wizard (first boot only, then disabled) |
| 80 | Dashboard / Management | NetBird Web UI (HTTP → HTTPS redirect) |
| 443 | Dashboard / Management | NetBird Web UI (HTTPS, Let's Encrypt) |
| 443 | Management API | gRPC (same port, different path) |
| 33073 | Signal | WebRTC signaling (always) |
| 3478 | Coturn | STUN/TURN (always) |

---

## 3. Wizard Flow

### 3.1 Step 1 — Network Configuration

**Purpose:** Configure the appliance's network identity.

**Fields:**
| Field | Type | Default | Required |
|-------|------|---------|----------|
| Hostname / FQDN | text | auto-detect | ✅ |
| Management IP | text (static/DHCP) | DHCP | optional |
| DNS Servers | text list | system default | optional |
| NTP Server | text | pool.ntp.org | optional |

**Validation:**
- FQDN must be a valid domain name
- IP must be a valid address or "DHCP"

### 3.2 Step 2 — TLS / SSL Certificate

**Purpose:** Secure the NetBird dashboard and management API.

**Options:**

**A) Let's Encrypt (Auto)** — recommended for production
- Domain must be publicly resolvable
- Port 80 must be reachable from internet
- Wizard handles ACME challenge automatically

**B) Custom Certificate (Manual)**
- Upload PEM files: certificate + private key + CA bundle (optional)
- Validation: certificate matches domain, not expired, key matches

**C) Self-signed (Development only)**
- Wizard generates a self-signed cert
- Warning displayed to user

### 3.3 Step 3 — Admin Account

**Purpose:** Create the initial local administrator.

**Fields:**
| Field | Type | Validation |
|-------|------|------------|
| Username | text | 3-32 chars, alphanumeric |
| Password | password | min 8 chars, complexity |
| Confirm Password | password | must match |
| Email | email | valid format |

**Behavior:**
- Creates a local admin user in the wizard database
- On deploy, configures this user in NetBird management
- This same user is used to log into the NetBird dashboard

### 3.4 Step 4 — Identity Provider

**Purpose:** Configure authentication source for NetBird users.

**Options:**

**A) Local Only** (default)
- Users managed directly in NetBird management
- Admin user from Step 3 is the only pre-created user

**B) Entra ID (Azure AD)**
| Field | Type | Required |
|-------|------|----------|
| Tenant ID | UUID | ✅ |
| Client ID | UUID | ✅ |
| Client Secret | password | ✅ |
| Allowed Domain | text | optional |
| Auto-provision | boolean | optional |

**Technical Flow:**
1. Wizard stores OIDC configuration
2. On deploy, configures NetBird management's IdP settings
3. NetBird handles the OAuth2/OIDC flow natively

### 3.5 Step 5 — Summary & Deploy

**Displays:**
- All configuration choices in a clean summary table
- "Edit" links for each step

**Actions:**
- **« Back:** Return to Step 4
- **🚀 Deploy:** Start the deployment process

**Deploy Flow (server-side):**
1. Validate all inputs again
2. Generate `docker-compose.yml` from template
3. Generate `.env` from template
4. Request Let's Encrypt certificate (if selected)
5. Run `docker compose up -d`
6. Wait for services to become healthy
7. Configure admin user in NetBird management API
8. Display completion screen with dashboard URL + QR code

**Deploy Log:** Real-time streaming via Server-Sent Events (SSE)

---

## 4. API Endpoints

All endpoints serve JSON. Wizard UI consumes these via HTMX.

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/` | Serve Wizard UI (HTML) |
| `GET` | `/api/status` | Return wizard state (step, completed) |
| `GET` | `/api/step/{n}` | Return saved data for step N |
| `POST` | `/api/step/{n}` | Save step N data (JSON body) |
| `POST` | `/api/deploy` | Start deployment |
| `GET` | `/api/deploy/log` | SSE stream of deploy progress |
| `GET` | `/api/health` | System health check (always available) |

---

## 5. Directory Structure

```
netsoft-ztna/
├── cmd/
│   └── wizard/main.go          # Entry point
├── internal/
│   ├── api/
│   │   ├── handlers.go         # REST handlers
│   │   ├── wizard.go           # Wizard page handler
│   │   └── middleware.go       # Logging, recovery
│   ├── config/
│   │   └── config.go           # Wizard config (flags, file)
│   ├── db/
│   │   ├── db.go               # SQLite init + migrator
│   │   └── models.go           # Step data structs
│   ├── deploy/
│   │   ├── deploy.go           # Orchestrator
│   │   ├── compose.go          # docker-compose generator
│   │   └── env.go              # .env generator
│   ├── tls/
│   │   ├── tls.go              # Interface
│   │   ├── letsencrypt.go      # ACME client
│   │   └── custom.go           # Custom cert handler
│   └── idp/
│       ├── idp.go              # IDP interface + config
│       ├── local.go            # Local auth config
│       └── entra.go            # Entra ID OIDC config
├── web/
│   ├── templates/
│   │   ├── layout.html
│   │   ├── index.html          # Wizard home (redirect to first incomplete step)
│   │   ├── step1_network.html
│   │   ├── step2_tls.html
│   │   ├── step3_admin.html
│   │   ├── step4_idp.html
│   │   ├── step5_summary.html
│   │   └── complete.html
│   └── static/
│       ├── css/app.css          # TailwindCSS
│       └── js/
│           ├── htmx.min.js
│           └── app.js
├── stack/
│   ├── docker-compose.yml.tmpl  # Go text/template
│   └── env.tmpl                 # Go text/template
├── packer/
│   └── ubuntu-24.04.pkr.hcl
├── scripts/
│   ├── first-boot.sh            # systemd oneshot → wizard
│   └── provision.sh             # Packer provisioning
├── go.mod
├── go.sum
├── .gitignore
├── PLAN.md
├── SPEC.md
└── README.md
```

---

## 6. NetBird Stack Templates

### 6.1 docker-compose.yml (generated)

```yaml
version: "3.8"
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
    depends_on: [postgres]
    ports:
      - "443:443"
      - "33073:33073"
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
    restart: unless-stopped
    networks:
      - netbird

  dashboard:
    image: netbirdio/dashboard:latest
    depends_on: [management]
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
    command: >
      -n --listening-port 3478
      --min-port 49152 --max-port 65535
      --realm ${NETBIRD_DOMAIN}
      --fingerprint --lt-cred-mech
      --no-auth
    restart: unless-stopped
    networks:
      - netbird

volumes:
  netbird_db:

networks:
  netbird:
    driver: bridge
```

*Note: Exact environment variables will be finalized during implementation based on NetBird's current management image configuration.*

### 6.2 .env (generated)

```
NETBIRD_DOMAIN=netbird.sirketim.com
DATABASE_PASSWORD=<random-32-chars>
SSL_CERT_PATH=/etc/netsoft/certs/fullchain.pem
SSL_KEY_PATH=/etc/netsoft/certs/privkey.pem
OIDC_ENABLED=false
OIDC_ISSUER=
OIDC_CLIENT_ID=
OIDC_CLIENT_SECRET=
AUTH_AUTHORITY=https://netbird.sirketim.com
```

---

## 7. Packer Build (OVA)

### 7.1 Build Environment
- **Builder:** Packer with VMware ISO builder (VMware Workstation or vCenter)
- **Output:** `.ova` file (VMware compatible)
- **Source:** Ubuntu 24.04 Server LTS ISO

### 7.2 Provisioning Steps

1. **Base OS** — Minimal Ubuntu 24.04 install
   - No snapd
   - No cloud-init
   - OpenSSH server

2. **System Hardening**
   - UFW: allow 22, 80, 443, 8080, 33073, 3478
   - SSH key auth only (password auth disabled after setup)
   - Automatic security updates
   - Kernel livepatch (optional)

3. **Docker Installation**
   - Docker CE from official repo
   - Compose plugin
   - User `netbird` in docker group

4. **Wizard Binary Installation**
   - `/usr/local/bin/netsoft-wizard` (pre-compiled Go binary)
   - `/opt/netsoft/` working directory
   - `/opt/netsoft/certs/` TLS certificates
   - `/opt/netsoft/stack/` generated docker-compose.yml + .env
   - `/opt/netsoft/data/` SQLite + persistent data

5. **Systemd Units**
   - `netsoft-wizard.service` — HTTP on :8080
   - `netsoft-wizard-ready.service` — oneshot that detects first boot and starts wizard
   - `netsoft-first-boot.service` — runs `first-boot.sh` on initial power-on

6. **Cleanup**
   - Remove apt cache
   - Zero-fill disk for compression
   - Remove SSH host keys (regenerated on boot)
   - Remove machine-id

### 7.3 First Boot Behavior

```
Power ON → cloud-init disabled → machine-id generated → SSH host keys generated
    → netsoft-first-boot runs:
        1. Check if /opt/netsoft/.deployed exists
        2. If NOT → start wizard on :8080
        3. Create /opt/netsoft/.first-boot-done
    → User opens browser to http://<ip>:8080
    → Completes 5 steps → clicks Deploy
    → wizard generates stack, requests cert, starts Docker services
    → wizard creates /opt/netsoft/.deployed (flag file)
    → wizard stops itself (systemd disables the service)
    → NetBird dashboard available on https://<domain>
```

---

## 8. Implementation Order

### Phase 1: Core Wizard Backend

| Step | Package | Files | Est. |
|------|---------|-------|------|
| 1 | config | `internal/config/config.go` | 1 |
| 2 | db | `internal/db/db.go`, `models.go` | 1 |
| 3 | deploy | `internal/deploy/compose.go`, `env.go` | 2 |
| 4 | deploy | `internal/deploy/deploy.go` | 2 |
| 5 | tls | `internal/tls/tls.go`, `letsencrypt.go` | 2 |
| 6 | idp | `internal/idp/*.go` | 2 |
| 7 | api | `internal/api/*.go` | 3 |
| 8 | main | `cmd/wizard/main.go` | 1 |

### Phase 2: Frontend

| Step | Files | Est. |
|------|-------|------|
| 9 | `web/templates/*.html` (7 templates) | 3 |
| 10 | `web/static/js/*.js`, `css/*.css` | 1 |

### Phase 3: Stack & Packaging

| Step | Files | Est. |
|------|-------|------|
| 11 | `stack/docker-compose.yml.tmpl`, `env.tmpl` | 1 |
| 12 | `scripts/first-boot.sh`, `provision.sh` | 1 |
| 13 | `packer/ubuntu-24.04.pkr.hcl` | 2 |

### Phase 4: Integration & Test

| Step | Task | Est. |
|------|------|------|
| 14 | Build wizard binary | 1 |
| 15 | Packer build OVA | 1 |
| 16 | Deploy OVA to vSphere, test wizard | 2 |
| 17 | Bug fixes | 2 |

---

## 9. Directory Layout (Final)

```
/ (root filesystem)
├── etc/
│   └── netsoft/
│       ├── config.yaml          # Wizard configuration
│       └── certs/               # TLS certificates
│           ├── fullchain.pem
│           └── privkey.pem
├── opt/
│   └── netsoft/
│       ├── bin/
│       │   └── netsoft-wizard   # Go binary
│       ├── stack/
│       │   ├── docker-compose.yml
│       │   └── .env
│       └── data/
│           ├── wizard.db        # SQLite wizard state
│           └── postgres/        # PostgreSQL volume
├── usr/
│   └── lib/
│       └── systemd/
│           └── system/
│               ├── netsoft-wizard.service
│               └── netsoft-first-boot.service
```

---

## 10. Dependencies (Go Modules)

| Module | Purpose |
|--------|---------|
| `modernc.org/sqlite` | CGo-free SQLite driver |
| `github.com/go-acme/lego/v4` | Let's Encrypt ACME client |
| `github.com/google/uuid` | UUID generation |
| `golang.org/x/crypto` | Password hashing (bcrypt) |
| `github.com/gorilla/mux` or `net/http` | HTTP routing (stdlib preferred) |

**Zero external frontend build dependencies.** Tailwind CSS used via CDN in MVP (standalone binary CLI later).

---

## 11. Configuration File

```yaml
# /etc/netsoft/config.yaml
listen_addr: ":8080"
data_dir: "/opt/netsoft/data"
stack_dir: "/opt/netsoft/stack"
web_dir: "/opt/netsoft/web"  # embedded in binary, overridable
cert_dir: "/etc/netsoft/certs"
netbird_domain: ""
auto_deploy: false   # if true, skip wizard and auto-configure
```

---

## 12. Security Considerations

1. **First boot only:** Wizard service binds to :8080 on first boot only. Once deployment completes, the service is disabled and port 8080 is closed.
2. **Password hashing:** Admin passwords hashed with bcrypt in SQLite.
3. **TLS everywhere:** NetBird dashboard and API serve exclusively over HTTPS.
4. **No root wizard:** Wizard runs as `netbird` user, not root. Docker access via group membership.
5. **Audit trail:** Wizard logs all steps to syslog.
6. **Least privilege:** NetBird management credentials stored in `.env` with 600 permissions.

---

## 13. Future (Post-MVP)

| Feature | Priority |
|---------|----------|
| ISO builder (bare metal / other hypervisors) | High |
| Cloud images (AWS AMI, Azure VHD) | High |
| Google Workspace OIDC | Medium |
| Okta / generic OIDC | Medium |
| Automated backup/restore | Medium |
| Version upgrade management | Medium |
| Prometheus metrics endpoint | Low |
| Cluster / HA mode | Low |
| Wizard TLS (self-signed for wizard UI) | Low |
| Multi-language UI | Low |

---

*Specification v1.0 — July 2026*
