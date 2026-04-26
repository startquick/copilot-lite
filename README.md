# Grokpi — Self-Hosted Guide

Grokpi is an OpenAI-compatible gateway for chat, image, and video workloads using Grok.
This guide focuses on self-hosting on your own server or VPS.

## 1. Key Features

- Dual-Format Endpoints: Supports both **OpenAI-compatible** (`/v1/chat/completions`) and **Anthropic-compatible** (`/v1/messages`) protocols.
- Admin API Endpoints for token pools, API keys, usage history, settings, and cache.
- Single API-first Go binary with a lightweight embedded admin access console.
- Smart Cloudflare recovery mechanism (*CF Challenge Bypass*) with automatic *circuit breaker*.
- Proactive alerts on upstream xAI status via Telegram Webhook notifications.
- SQLite as default, PostgreSQL as an option.
- Deployment support via Docker Compose.

## 2. System Requirements & Deployment

GrokPi is deployed using **Docker Compose** — both locally and on a VPS. No Go toolchain is needed on the host machine; binary builds happen inside the container via a multi-stage build.

Complete guides are available in separate documents:

- 👉 **[Local Deployment Checklist (Windows)](docs/deployment-checklist-lokal.md)**
- 👉 **[Ubuntu VPS Deployment Checklist](docs/deployment-checklist-ubuntu.md)**
- 👉 **[Complete Deployment Guide](docs/deployment.md)**



## 5. Initial Configuration via Admin Access Console

For quick changes without SSH or logging into the server, open:

`http://127.0.0.1:8080/admin/access`

Log in using the `app_key`, then manage:

1. Grok upstream tokens
2. Client API keys

This console uses the same admin endpoints as the script, making it suitable for lightweight servers without a separate frontend.

## 6. Alternative via Admin Script

We have provided interactive scripts to simplify initial configuration without needing to remember *curl* commands.

**For Windows users:**
Open PowerShell, navigate to the project directory, and run:
```powershell
.\scripts\windows\grokpi_admin.ps1
```

**For Linux / macOS users:**
Open the terminal, navigate to the project directory, grant execution permissions if needed, and run:
```bash
chmod +x ./scripts/linux/grokpi_admin.sh
./scripts/linux/grokpi_admin.sh
```

An interactive menu will appear. Follow the on-screen instructions to:
1. Add Grok Upstream Tokens (you can paste multiple token sets at once separated by commas).
2. Create an API Key (Use this in your Apps like AnythingLLM/Dify).

*This API Key will be used for requests to the `/v1/chat/completions` endpoint or registered in LLM Apps like AnythingLLM, Dify, etc.*

## 7. Minimal Configuration Example

```toml
[app]
app_key = "REPLACE_WITH_STRONG_PASSWORD"
host = "0.0.0.0"
port = 8080

db_driver = "sqlite"
db_path = "data/grokpi.db"

log_level = "info"
log_json = false

[proxy]
base_proxy_url = ""
asset_proxy_url = ""
enabled = false
# Optional - Cloudflare Failure Alerts
telegram_bot_token = ""
telegram_chat_id = ""
```

Important notes:

- An empty `app_key` will block admin access.
- Do not share the `config.toml` file publicly.
- For public deployment, use a reverse proxy with TLS.

## 8. API Usage Examples

### 7.1 List Models

```bash
curl -s http://127.0.0.1:8080/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### 7.2 Chat Completion

```bash
curl -s http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "grok-3-mini",
    "messages": [
      {"role": "user", "content": "Hello from self-hosted Grokpi"}
    ]
  }'
```

### 7.3 Chat Completion (Anthropic / Claude Format)

```bash
curl -s http://127.0.0.1:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_API_KEY" \
  -d '{
    "model": "grok-3",
    "max_tokens": 1024,
    "system": "You are a helpful assistant.",
    "messages": [
      {"role": "user", "content": "Hello from Grokpi with Anthropic API format"}
    ]
  }'
```

### 7.4 Image Generation

```bash
curl -s http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "grok-imagine-1.0",
    "messages": [
      {"role":"user","content":"Mountain lake at sunrise"}
    ],
    "image_config": {
      "aspect_ratio": "16:9"
    }
  }'
```

### 7.5 Video Generation

```bash
curl -s http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "grok-imagine-1.0-video",
    "messages": [
      {"role":"user","content":"Cinematic drone shot over green rice fields"}
    ],
    "video_config": {
      "aspect_ratio": "16:9",
      "video_length": 8,
      "resolution_name": "480p",
      "preset": "normal"
    }
  }'
```
