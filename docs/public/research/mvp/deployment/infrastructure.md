# Infrastructure Requirements

## Hardware

### Minimum Specifications

| Resource | Requirement | Notes |
|----------|-------------|-------|
| CPU | ≥16 vCPU | PostgreSQL and Go API are CPU-bound during batch operations |
| RAM | ≥64 GB | PostgreSQL buffer pool + application memory |
| Storage (OS) | 100 GB SSD | Container images, application binaries |
| Storage (DB) | 500 GB NVMe | PostgreSQL data directory — must be NVMe for IOPS |
| Storage (Objects) | 1 TB | MinIO object storage for documents, images |
| Network | 1 Gbps | Hetzner default is sufficient |

### Recommended Specifications

| Resource | Recommended | Notes |
|----------|-------------|-------|
| CPU | 32 vCPU | Headroom for concurrent reconciliation workers |
| RAM | 128 GB | Larger PostgreSQL shared_buffers (32 GB) |
| Storage (DB) | 1 TB NVMe | Room for growth |
| Storage (Objects) | 2 TB | Product images accumulate quickly |

### NVMe for PostgreSQL

PostgreSQL performance depends heavily on storage IOPS. Use NVMe for the PostgreSQL data volume.

```bash
# Verify NVMe is mounted
lsblk -d -o NAME,ROTA,SIZE
# ROTA=0 indicates SSD/NVMe

# Create dedicated mount for postgres data
sudo mkfs.xfs /dev/nvme0n1p1
sudo mkdir -p /var/lib/postgresql
sudo mount /dev/nvme0n1p1 /var/lib/postgresql
echo '/dev/nvme0n1p1 /var/lib/postgresql xfs defaults,noatime 0 2' | sudo tee -a /etc/fstab
```

## Software

### Required Packages

```bash
# Debian/Ubuntu
sudo apt update
sudo apt install -y podman podman-compose systemd curl git

# For acme.sh (TLS certificates)
curl https://get.acme.sh | sh -s email=admin@hxd3v.com
```

### Podman (Rootless)

```bash
# Verify rootless Podman
podman --version
podman info --format '{{.Host.Security.NamespaceMode}}'
# Should output: "keep-uid"

# Configure subuid/subgid for the deploy user
sudo usermod --add-subuids 100000-165535 --add-subgids 100000-165535 helix
```

### Systemd Integration

```bash
# Enable lingering for the service user (allows systemd services without login)
sudo loginctl enable-linger helix

# Podman generates systemd units
podman generate systemd --new --name helix-stack -f
```

## Networking

### Firewall Rules (nftables / ufw)

```bash
# Required open ports
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 80/tcp    # HTTP (redirects to HTTPS)
sudo ufw allow 443/tcp   # HTTPS
sudo ufw enable

# Internal services must NOT be exposed
# PostgreSQL (5432), Redis (6379), NATS (4222), MinIO (9000/9001)
# Only accessible via 127.0.0.1
```

### DNS Records

| Record | Type | Value | TTL |
|--------|------|-------|-----|
| seller.hxd3v.com | A | `<server-ip>` | 300 |
| sta.seller.hxd3v.com | A | `<server-ip>` | 300 |
| dev.seller.hxd3v.com | A | `<server-ip>` | 300 |

### TLS Certificates

Managed via acme.sh with DNS or HTTP validation:

```bash
# HTTP validation (requires port 80)
~/.acme.sh/acme.sh --issue -d seller.hxd3v.com --standalone
~/.acme.sh/acme.sh --issue -d sta.seller.hxd3v.com --standalone
~/.acme.sh/acme.sh --issue -d dev.seller.hxd3v.com --standalone

# Auto-renewal is configured by acme.sh cron
```

Alternatively, Caddy handles automatic TLS via Let's Encrypt when exposed publicly.

## Storage Layout

```
/var/lib/helix-seller/              # Application root
├── docker-compose.yml              # Podman Compose file
├── .env.production                 # Environment config
├── keys/                           # JWT keys, TLS certs
│   ├── jwt_private.pem
│   └── jwt_public.pem
├── data/
│   ├── postgres/                   # NVMe-backed PostgreSQL data
│   ├── redis/                      # Redis persistence
│   ├── minio/                      # Object storage
│   └── nats/                       # NATS JetStream storage
├── config/
│   ├── prometheus.yml
│   └── grafana/
│       └── provisioning/
└── logs/                           # Application logs
```
