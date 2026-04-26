# Final Deployment Checklist to Ubuntu VPS

This document is a practical step-by-step checklist to bring GrokPi Lite from a local repository to an Ubuntu VPS ready for internet access with an HTTPS reverse proxy.

Use this checklist alongside the complete guide in `docs/deployment.md`.

## 1. VPS Preparation

1. Login to the VPS as `root`.
```bash
ssh root@VPS_IP
```

2. Update the system.
```bash
apt-get update && apt-get upgrade -y
```

3. Create a non-root deploy user.
```bash
adduser grokdeploy
usermod -aG sudo grokdeploy
```

4. Switch to the deploy user.
```bash
su - grokdeploy
```

## 2. Basic Hardening

1. Install and enable UFW.
```bash
sudo apt-get install -y ufw
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw deny 8080/tcp
sudo ufw deny 8191/tcp
sudo ufw enable
sudo ufw status
```

2. Ensure only ports `22`, `80`, and `443` are open to the public.

## 3. Install Dependencies

1. Install base packages.
```bash
sudo apt-get install -y ca-certificates curl gnupg lsb-release git make
```

2. Install Docker and the Docker Compose plugin.
```bash
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo usermod -aG docker $USER
```

3. Logout and login again so the `docker` group is active.
```bash
exit
ssh grokdeploy@VPS_IP
docker --version
docker compose version
```

## 4. Retrieve Source Code

1. Clone the repository.
```bash
cd ~
git clone https://github.com/startquick/GrokPi-Lite.git
cd GrokPi-Lite
```

2. Verify important files exist.
```bash
ls
ls docs
ls scripts/linux
```

## 5. Setup Production Configuration

1. Copy the config template.
```bash
cp config.defaults.toml config.toml
```

2. Edit `config.toml`.
```bash
nano config.toml
```

3. You must ensure the following:
- `app.app_key` is replaced with a strong new secret
- `db_driver` and `db_path` fit your needs
- `proxy.*` is filled if using FlareSolverr/proxy
- do not use the default value `QUICKstart012345+`

4. Setup the runtime directories.
```bash
mkdir -p data logs
sudo chown -R 1000:1000 data logs
```

## 6. Run the Container

1. Start the service (`--build` directly triggers a multi-stage build inside Docker).
```bash
docker compose up -d --build
```

3. Check the container status.
```bash
docker compose ps
docker compose logs --tail=100 grokpi
docker compose logs --tail=100 flaresolverr
```

4. Check internal health.
```bash
curl -s http://127.0.0.1:8080/health
```

## 7. Verify Network Exposure

1. Ensure the backend only binds to localhost.
```bash
ss -tulpn | grep 8080
```

2. Ensure FlareSolverr is not published to the host.
```bash
ss -tulpn | grep 8191
```

Expectations:
- `8080` only appears on `127.0.0.1`
- `8191` does not appear as a public host port

## 8. Setup Reverse Proxy HTTPS with Caddy

1. Install Caddy.
```bash
sudo apt-get install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt-get update
sudo apt-get install -y caddy
```

2. Point the domain to the VPS IP in your DNS provider.

3. Edit the Caddyfile.
```bash
sudo nano /etc/caddy/Caddyfile
```

4. Provide this minimum configuration:
```text
api.yourdomain.com {
    reverse_proxy 127.0.0.1:8080
}
```

5. Restart and check Caddy.
```bash
sudo systemctl restart caddy
sudo systemctl status caddy --no-pager
curl -I https://api.yourdomain.com/health
```

## 9. Admin Configuration and API Key

### Option A: Admin UI via Browser (Recommended)

GrokPi provides a built-in web-based admin page. After Caddy is active, open your browser and navigate to:

```
https://api.yourdomain.com/admin/access
```

Or if you don't have a domain yet (direct access from the VPS):

```
http://127.0.0.1:8080/admin/access
```

Log in with the `app_key` you set in `config.toml`. From this dashboard you can:
- **Tokens** — import, view status, delete, and refresh Grok SSO tokens
- **API Keys** — create and manage `sk-...` client keys
- **Stats** — view quotas, usage logs, and the token pool
- **Config** — edit runtime config without restarting
- **Cache** — manage video/image cache files

> [!NOTE]
> The Admin UI is *embedded* directly into the binary, so it's automatically available without extra setup.

### Option B: CLI Script (via SSH)

1. Run the admin script.
```bash
cd ~/GrokPi-Lite
chmod +x ./scripts/linux/grokpi_admin.sh
./scripts/linux/grokpi_admin.sh
```

2. Follow these steps:
- log in as admin with the `app_key`
- import Grok SSO tokens
- create a client API key `sk-...`

## 10. API Smoke Test

1. Test the model list.
```bash
curl -s https://api.yourdomain.com/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

2. Test the OpenAI-compatible endpoint.
```bash
curl -s https://api.yourdomain.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "grok-4.1-fast",
    "messages": [{"role":"user","content":"Hello from the VPS"}]
  }'
```

3. Test the Anthropic-compatible endpoint.
```bash
curl -s https://api.yourdomain.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_API_KEY" \
  -d '{
    "model": "grok-4.1-fast",
    "max_tokens": 256,
    "messages": [{"role":"user","content":"Hello from the messages endpoint"}]
  }'
```

## 11. Post-Deploy Smoke Test

After the container, reverse proxy, and admin key are ready, run this quick smoke test from the VPS. Replace `APP_KEY` and `YOUR_API_KEY` with the correct values.

```bash
APP_KEY='your-admin-app-key'
API_KEY='your-sk'
BASE_URL='http://127.0.0.1:8080'

curl -fsS "$BASE_URL/health"
curl -fsS "$BASE_URL/admin/verify" -H "Authorization: Bearer $APP_KEY"
curl -fsS "$BASE_URL/v1/models" -H "Authorization: Bearer $API_KEY"
curl -fsS "$BASE_URL/admin/tokens?page_size=10" -H "Authorization: Bearer $APP_KEY"
```

Alternatively, if `make` is available on the VPS:

```bash
make smoke BASE_URL=http://127.0.0.1:8080 APP_KEY="$APP_KEY" API_KEY="$API_KEY"
```

Expectations:
- `/health` returns an `ok` status
- `/admin/verify` returns `{"status":"ok"}`
- `/v1/models` returns the model list
- `/admin/tokens` returns the admin token list without an auth error

## 12. Initial Backup

Once the system is healthy, create an initial backup.
```bash
cd ~/GrokPi-Lite
tar czf grokpi-backup-$(date +%F).tar.gz config.toml data/
```

## 13. Final Checklist Before Going Live

- `app_key` is not default
- `docker compose ps` shows healthy services
- `curl http://127.0.0.1:8080/health` succeeds on the VPS
- `curl -I https://domain/health` succeeds from outside
- port `8080` is not publicly open
- port `8191` is not publicly open
- admin script can log in
- `/v1/chat/completions` succeeds
- `/v1/messages` succeeds
- initial backup has been created

## 14. Safe Updates After Going Live

Use a non-destructive update flow:
```bash
cd ~/GrokPi-Lite
git pull
docker compose build --no-cache grokpi
docker compose up -d
```

The `--no-cache` option ensures the Docker image discards the old state and completely rebuilds the latest file changes (including the frontend/UI components). You don't need to manually run `make build` because everything is compiled (automatically rebuilt) with the latest code inside the container.

If there are important local changes to `config.toml` or other deploy files, back them up first and resolve any Git conflicts manually.
