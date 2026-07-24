#!/bin/bash
# Generate systemd user services from podman-compose
# Run this after creating/modifying the podman-compose file

set -e

COMPOSE_FILE="podman-compose.opendesign.yml"
SERVICE_NAME="opendesign"

echo "Generating systemd user services from ${COMPOSE_FILE}..."

# Stop existing services if running
systemctl --user stop ${SERVICE_NAME}.service 2>/dev/null || true

# Generate systemd units
podman generate systemd \
    --new \
    --files \
    --name ${SERVICE_NAME} \
    --restart-policy=on-failure \
    --restart-sec=10 \
    --time=30

# Move generated files to systemd user directory
mv container-*.service ~/.config/systemd/user/ 2>/dev/null || true
mv pod-*.service ~/.config/systemd/user/ 2>/dev/null || true

# Create a target service that depends on all containers
cat > ~/.config/systemd/user/${SERVICE_NAME}.service << 'EOF'
[Unit]
Description=OpenDesign Design Tool Stack
After=network-online.target container-opendesign-server.service container-opendesign-postgres.service container-opendesign-redis.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/true
ExecStop=/bin/true

[Install]
WantedBy=default.target
EOF

# Reload systemd
systemctl --user daemon-reload

echo "Systemd services generated. Start with:"
echo "  systemctl --user start ${SERVICE_NAME}.service"
echo ""
echo "Enable on boot with:"
echo "  systemctl --user enable ${SERVICE_NAME}.service"
