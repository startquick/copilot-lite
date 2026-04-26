# Local Deployment Checklist (Windows/PowerShell)

This guide uses **Docker as the main method** — consistent with VPS deployment.
No Go toolchain is needed on Windows; all builds happen inside the container.

---

## Initial Setup (One-time)

**1. Enter the project folder**

```powershell
cd "E:\GrokPi Lite"
```

**2. Setup local config**

Copy the default template:

```powershell
Copy-Item .\config.defaults.toml .\config.toml
```

Edit `config.toml`, at minimum change this section:

```toml
[app]
app_key = "replace-with-your-own-admin-key"  # REQUIRED to be changed from default
host = "0.0.0.0"
port = 8080
```

> [!IMPORTANT]
> **Do not leave `app_key` as `QUICKstart012345+`** — the container will refuse to start if this default value is still used.

**3. Setup local `.env` file**

Create a `.env` file in the project root (if it doesn't exist yet):

```powershell
@"
COMPOSE_FILE=docker-compose.yml:docker-compose.local.yml
"@ | Set-Content .\.env
```

This file tells Docker Compose to merge both compose files automatically,
so you **don't need to pass `-f` flags** every time.

> [!NOTE]
> The `.env` and `docker-compose.local.yml` files are in `.gitignore` — they will not be committed.

**4. Create runtime directories**

```powershell
New-Item -ItemType Directory -Force -Path .\data, .\logs
```

---

## Running the Service

**5. Run the service (main command)**

```powershell
docker compose up -d --build
```

Or via Makefile:

```powershell
make docker-up
```

This command will:
- Build the Go binary inside the container (no Go needed on Windows)
- Start FlareSolverr and GrokPi as registered services
- Ensure all services run in the background (`-d`)

**6. Check service status**

```powershell
docker compose ps
# or
make docker-ps
```

Expectation: both services (`flaresolverr` and `grokpi`) show a `healthy` status.

**7. Check health endpoint**

```powershell
Invoke-RestMethod http://127.0.0.1:8080/health
```

**8. View realtime logs**

```powershell
docker compose logs -f grokpi
# or
make docker-logs
```

---

## Admin & Token Management

**9. Access Admin UI (via browser)**

GrokPi provides a built-in web-based admin page. Once the service is running, open a browser and navigate to:

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
> The Admin UI is *embedded* directly into the binary, so it's automatically available without any extra setup.

**10. Alternative: CLI script (no browser)**

In accordance with this repo's architectural rules, do not manage admin manually with `curl`. Use the interactive Windows script:

```powershell
.\scripts\windows\grokpi_admin.ps1
```

From there the typical flow is:
- Login using the `app_key` created in step 2
- Import Grok SSO tokens
- Create a client API key with an `sk-...` prefix
- View stats/token status

---

## Using the API

**11. Test endpoint locally**

```powershell
$headers = @{
  Authorization = "Bearer sk-xxxxx"
  "Content-Type" = "application/json"
}

$body = @{
  model = "grok-4.1-fast"
  messages = @(
    @{ role = "user"; content = "Hello, answer briefly." }
  )
  stream = $false
} | ConvertTo-Json -Depth 10

Invoke-RestMethod `
  -Uri "http://127.0.0.1:8080/v1/chat/completions" `
  -Method Post `
  -Headers $headers `
  -Body $body
```

Test the Anthropic-compatible endpoint:

```powershell
$headers = @{
  "x-api-key" = "sk-xxxxx"
  "Content-Type" = "application/json"
}

$body = @{
  model = "grok-4.1-fast"
  messages = @(
    @{ role = "user"; content = "Hello from the messages endpoint." }
  )
  max_tokens = 256
  stream = $false
} | ConvertTo-Json -Depth 10

Invoke-RestMethod `
  -Uri "http://127.0.0.1:8080/v1/messages" `
  -Method Post `
  -Headers $headers `
  -Body $body
```

View the model list:

```powershell
Invoke-RestMethod `
  -Uri "http://127.0.0.1:8080/v1/models" `
  -Headers @{ Authorization = "Bearer sk-xxxxx" }
```

---

## Updating the Code

**11. Update after code changes**

```powershell
git pull
docker compose build --no-cache grokpi
docker compose up -d
# or
make docker-up
```

The `--no-cache` option ensures the Docker image discards the old state and completely rebuilds the latest file changes. You don't need to manually run `make build` because everything is compiled (automatically rebuilt) with the latest code inside the container.

---

## Useful Docker Commands

| Command | Description |
|---|---|
| `make docker-up` | Run all services (build + start) |
| `make docker-down` | Stop and remove all containers |
| `make docker-logs` | Follow GrokPi logs in realtime |
| `make docker-ps` | View status of all containers |
| `make docker-restart` | Restart the GrokPi container only |
| `make docker-shell` | Enter the shell inside the GrokPi container |

---

## What Not to Do

> [!CAUTION]
> - Do not run `make clean` carelessly — it will delete the `data/` database folder.
> - Do not manage admin tokens and API Keys using manual `curl` commands; always use `./scripts/windows/grokpi_admin.ps1` or the browser Admin UI.
> - Do not edit `docker-compose.yml` for local needs only — use `docker-compose.local.yml` as an override.
> - **Do not run the service directly with `make run`, `make dev`, or `./bin/grokpi.exe`** — Docker is the only supported deployment method. The native binary method is only for advanced Go development purposes.
