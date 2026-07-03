# Netsoft ZTNA — Virtual Appliance Plan (Final)

## Project Identity
| Field | Value |
|-------|-------|
| Name | **Netsoft ZTNA** |
| Purpose | NetBird on-prem setup wizard virtual appliance |
| Repository | github.com/erkantug/netsoft-ztna |
| License | Apache 2.0 |
| Build Server | 192.168.200.68 (Ubuntu 24.04) |

## Delivery Formats (Ordered)
1. OVA (VMware) — first release
2. ISO (bare metal) — future
3. Cloud images (AMI, VHD) — future

## Technical Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Base OS | Ubuntu 24.04 LTS minimal | 2029 support |
| Container | Docker + Compose plugin | Stack isolation |
| Wizard Backend | Go | Single binary, NetBird alignment |
| Wizard Frontend | Go templates + HTMX | No Node.js build step |
| Wizard State | SQLite (modernc.org/sqlite) | CGo-free |
| NetBird DB | PostgreSQL 16 Alpine | NetBird requirement |
| TLS | Let's Encrypt + Custom + Self-signed | Flexibility |
| OVA Build | Packer + VMware ISO builder | Reproducible |
| Wizard Port | 8080 | Non-privileged port |
| Branch | master | Default |
| NetBird Version | latest stable | Always up-to-date |

## Wizard Steps (5 Steps)
Step 1: Network (FQDN, DNS, NTP)
Step 2: TLS (Let's Encrypt / Custom / Self-signed)
Step 3: Admin Account (local user)
Step 4: Identity Provider (Local / Entra ID OIDC)
Step 5: Summary and Deploy (with SSE live log)

## MVP Scope
- 5-step web wizard on :8080
- NetBird stack (mgmt + dashboard + signal + coturn + postgres)
- Let's Encrypt automated TLS
- Custom certificate upload and Self-signed option
- Local admin authentication
- Entra ID (Azure AD) OIDC integration
- Docker Compose generation from templates
- First-boot detection and auto-start
- Packer-based OVA build

## Implementation Order
1. internal/config/config.go
2. internal/db/ (db.go + models.go)
3. internal/deploy/ (compose.go + env.go + deploy.go)
4. internal/tls/ (tls.go + letsencrypt.go)
5. internal/idp/ (idp.go + local.go + entra.go)
6. internal/api/ (handlers.go + wizard.go)
7. cmd/wizard/main.go
8. web/templates/*.html (7 files)
9. web/static/ (HTMX + CSS)
10. stack/ (docker-compose.yml.tmpl + env.tmpl)
11. scripts/ (first-boot.sh + provision.sh)
12. packer/ubuntu-24.04.pkr.hcl
13. Build + test
