# OpenDesign Integration

OpenCode commands for interacting with the OpenDesign design tool.

## Available Commands

### `opendesign:start`
Start the OpenDesign server stack (Podman Compose).

### `opendesign:stop`
Stop the OpenDesign server stack.

### `opendesign:status`
Check the status of the OpenDesign server stack.

### `opendesign:logs`
View logs from the OpenDesign server.

### `opendesign:open`
Open the OpenDesign web interface in the default browser.

## Setup

1. Install OpenDesign via Podman:
   ```bash
   podman-compose -f podman-compose.opendesign.yml up -d
   ```

2. Generate systemd services:
   ```bash
   ./scripts/generate-systemd-services.sh
   ```

3. Enable the service:
   ```bash
   systemctl --user enable opendesign.service
   systemctl --user start opendesign.service
   ```

4. Access OpenDesign at: http://localhost:3000

## Configuration

- Configuration directory: `~/.config/opendesign/`
- Compose file: `podman-compose.opendesign.yml`
- Systemd service: `~/.config/systemd/user/opendesign.service`

## API Access

The OpenDesign server exposes a REST API at `http://localhost:3000/api/v1/`

Common endpoints:
- `GET /api/v1/projects` - List all projects
- `POST /api/v1/projects` - Create a new project
- `GET /api/v1/projects/:id` - Get project details
- `PUT /api/v1/projects/:id` - Update a project
- `DELETE /api/v1/projects/:id` - Delete a project

## Troubleshooting

### Service won't start
```bash
# Check podman status
podman ps -a

# Check logs
podman logs opendesign-server

# Restart the stack
systemctl --user restart opendesign.service
```

### Database issues
```bash
# Check PostgreSQL status
podman exec opendesign-postgres pg_isready

# Reset database
podman-compose -f podman-compose.opendesign.yml down -v
podman-compose -f podman-compose.opendesign.yml up -d
```

### Port conflicts
If ports 3000, 5432, or 6379 are in use, edit `podman-compose.opendesign.yml` and change the port mappings.
