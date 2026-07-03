# Netsoft ZTNA — Virtual Appliance Plan

## 1. Proje Künyesi

| Özellik | Değer |
|---------|-------|
| Proje Adı | Netsoft ZTNA |
| Amaç | NetBird on-prem kurulumunu wizard ile kolaylaştıran sanal appliance |
| Depo | github.com/erkantug/netsoft-ztna |
| Lisans | Apache 2.0 |
| Build Server | 192.168.200.68 (Ubuntu 24.04, Go 1.22, Docker) |

## 2. Dağıtım Formatları (Sıralı)

| # | Format | Hedef | Durum |
|---|--------|-------|-------|
| 1 | OVA | VMware vSphere/ESXi | **İlk aşama** |
| 2 | ISO | Bare metal / diğer hypervisor'lar | Sonraki aşama |
| 3 | Cloud Image | AWS AMI, Azure VHD | Sonraki aşama |

## 3. Teknik Kararlar

| Karar | Seçim | Gerekçe |
|-------|-------|---------|
| Base OS | Ubuntu 24.04 LTS minimal | 2030'a kadar support, geniş uyum |
| Container Engine | Docker + Compose plugin | NetBird stack izolasyonu, operasyon kolaylığı |
| Wizard Backend | **Go** | NetBird ile aynı dil, tek binary, düşük footprint |
| Wizard Frontend | **Go templates + HTMX + Tailwind CSS** | Node.js derleme adımı yok, tek binary'de her şey |
| Database | SQLite (wizard state) + PostgreSQL (NetBird) | Wizard basit state'i için SQLite yeterli |
| TLS | Let's Encrypt (ACME) + custom cert upload | Otomatik veya manuel sertifika |
| OVA Builder | **Packer** + VMware (govc) | Tekrarlanabilir, CI uyumlu |
| Auth | Local (built-in) + Entra ID (OIDC) | Enterprise hazır |

## 4. NetBird Stack (Docker Compose)

```
netbird-stack/
├── docker-compose.yml    # mgmt + dashboard + signal + coturn + postgres
├── .env                  # tüm değişkenler wizard'dan doldurulur
└── certs/                # Let's Encrypt veya custom cert
```

| Servis | İmaj | Rol |
|--------|------|-----|
| Management | netbirdio/management:latest | gRPC API, peer/auth yönetimi |
| Dashboard | netbirdio/dashboard:latest | Web UI |
| Signal | netbirdio/signal:latest | WebRTC signaling |
| Coturn | coturn/coturn:latest | TURN relay |
| PostgreSQL | postgres:16-alpine | Veritabanı |

## 5. Wizard Akışı (5 Adım)

```
┌─────────────────────────────────────────────┐
│ Step 1: Network                              │
│ - Hostname / FQDN                           │
│ - Management IP (statik/DHCP)               │
│ - DNS sunucuları                            │
│ - NTP sunucuları                            │
├─────────────────────────────────────────────┤
│ Step 2: TLS / SSL                           │
│ ▸ Let's Encrypt (otomatik, port 80 açık)    │
│ ▸ Custom certificate (PEM yükle)            │
│ ▸ Self-signed (test içi)                    │
├─────────────────────────────────────────────┤
│ Step 3: Admin Hesabı                        │
│ - Local admin kullanıcı adı                │
│ - Şifre                                     │
│ - E-posta                                   │
├─────────────────────────────────────────────┤
│ Step 4: Identity Provider                   │
│ ▸ Local only (built-in users)              │
│ ▸ Entra ID (Azure AD) OIDC:                │
│   · Tenant ID                               │
│   · Client ID                               │
│   · Client Secret                          │
│   · Allowed domain (opsiyonel)             │
│ ▸ Google Workspace (ileri)                 │
│ ▸ Okta / generic OIDC (ileri)              │
├─────────────────────────────────────────────┤
│ Step 5: Özet & Deploy                       │
│ - Tüm ayarların özeti                      │
│ - "Deploy" butonu                          │
│ - Canlı log (SSE)                          │
│ - Tamamlanınca dashboard URL + QR          │
└─────────────────────────────────────────────┘
```

## 6. Wizard Backend API

| Method | Path | Açıklama |
|--------|------|----------|
| GET | /api/status | Wizard durumu (completed/pending) |
| GET | /api/step/:n | Adım verilerini getir |
| POST | /api/step/:n | Adım verilerini kaydet |
| POST | /api/deploy | Deploy'u başlat |
| GET | /api/deploy/log | SSE log stream |
| GET | /api/health | Sistem sağlık kontrolü |
| GET | / | Wizard UI (HTML) |

## 7. Dosya Yapısı

```
netsoft-ztna/
├── cmd/
│   └── wizard/
│       └── main.go                  # Entry point
├── internal/
│   ├── api/
│   │   ├── handlers.go             # REST handlers
│   │   ├── wizard.go               # Wizard UI handler
│   │   └── middleware.go           # Auth, logging
│   ├── config/
│   │   └── config.go               # Wizard configuration
│   ├── db/
│   │   ├── db.go                   # SQLite init/migration
│   │   └── models.go               # Step data models
│   ├── deploy/
│   │   ├── deploy.go               # Deploy orchestrator
│   │   ├── compose.go              # docker-compose generator
│   │   └── env.go                  # .env generator
│   ├── tls/
│   │   ├── tls.go                  # TLS manager interface
│   │   ├── letsencrypt.go          # ACME / Let's Encrypt
│   │   └── custom.go               # Custom cert handler
│   └── idp/
│       ├── idp.go                  # IDP interface
│       ├── local.go                # Local auth provider
│       └── entra.go                # Entra ID OIDC provider
├── web/
│   ├── templates/
│   │   ├── layout.html             # Base layout
│   │   ├── index.html              # Wizard ana sayfa
│   │   ├── step1.html              # Network
│   │   ├── step2.html              # TLS
│   │   ├── step3.html              # Admin
│   │   ├── step4.html              # IDP
│   │   ├── step5.html              # Özet / Deploy
│   │   └── complete.html           # Kurulum tamamlandı
│   └── static/
│       ├── css/
│       │   └── app.css             # Tailwind CSS
│       └── js/
│           ├── htmx.min.js         # HTMX
│           └── app.js              # Custom JS
├── stack/
│   ├── docker-compose.yml.tmpl     # Docker Compose template
│   └── env.tmpl                    # .env template
├── packer/
│   └── ubuntu-24.04.pkr.hcl        # Packer template
├── scripts/
│   ├── first-boot.sh               # First boot detection & wizard start
│   └── post-install.sh             # Packer provisioning script
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```

## 8. Packer OVA Build Akışı

```
Ubuntu 24.04 ISO
    ↓
Packer (VMware ISO builder)
    ↓
┌────────────────────────────────┐
│ Provisioning:                  │
│ 1. OS updates                  │
│ 2. Docker kurulumu             │
│ 3. Go wizard binary install    │
│ 4. systemd unit + timer setup  │
│ 5. SSH hardening               │
│ 6. Firewall (UFW)              │
│ 7. Cleanup + shrink            │
└────────────────────────────────┘
    ↓
OVA output (VMware compatible)
    ↓
First boot → systemd oneshot → Wizard UI:80
```

## 9. İlk Sürüm Kapsamı (MVP)

| Özellik | MVP | Sonraki |
|---------|-----|---------|
| Wizard UI (5 adım) | ✅ | — |
| NetBird stack deploy | ✅ | — |
| Let's Encrypt TLS | ✅ | — |
| Local admin auth | ✅ | — |
| Entra ID (Azure AD) | ✅ | — |
| Custom cert upload | ✅ | — |
| Google Workspace OIDC | — | ✅ |
| Okta / generic OIDC | — | ✅ |
| ISO format | — | ✅ |
| Cloud images | — | ✅ |
| Cluster / HA | — | ✅ |
| Monitoring (Prometheus) | — | ✅ |
| Backup / Restore | — | ✅ |
| Upgrade mgmt | — | ✅ |

## 10. Kodlama Sırası

| # | Modül | Bağımlılık | Süre |
|---|-------|-----------|------|
| 1 | config.go | — | 1 birim |
| 2 | db.go (SQLite models) | config | 1 |
| 3 | Stack templates (.tmpl) | — | 1 |
| 4 | deploy/compose.go + env.go | templates | 2 |
| 5 | tls.go + letsencrypt.go | config | 2 |
| 6 | idp.go + local.go + entra.go | config | 2 |
| 7 | api/handlers.go + wizard.go | tümü | 3 |
| 8 | HTML templates | api | 2 |
| 9 | main.go | tümü | 1 |
| 10 | Packer template | — | 2 |
| 11 | Derleme + test | tümü | 1 |
| | **Toplam** | | **~18 birim** |
