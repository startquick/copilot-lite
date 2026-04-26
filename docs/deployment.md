# GrokPi Lite Deployment Guide - Production Ready

This guide is comprehensively structured from scratch (new server), basic security (hardening), to being ready to use in a production stage with a custom domain (HTTPS).

For a brief, step-by-step flow, also use the separate checklists:

- [Final Deployment Checklist to Ubuntu VPS](deployment-checklist-ubuntu.md)
- [Local Deployment Checklist (Docker)](deployment-checklist-lokal.md)

> **Consistency Note**: Both local and VPS use Docker as the main deployment method.
> `Dockerfile.local` uses a multi-stage build, so it does not require a Go toolchain on the host machine.

## 1. Minimum Server Requirements

GrokPi is built using the highly lightweight Go language and does not require many *resources*. Here are the recommended VPS (Virtual Private Server) specifications you need:

*   **Operating System**: Ubuntu 22.04 LTS or 24.04 LTS (Debian 12 is also supported).
*   **CPU**: Minimum 1 vCPU.
*   **RAM**: Minimum 1 GB (Ideally 2 GB).
*   **Storage**: 10 GB SSD.
*   **Access**: Public IPv4.

---

## 2. Server Preparation from Scratch (Init & Hardening)

When first renting a VPS, you will usually get login credentials as `root`. Running applications directly as `root` is very dangerous. Follow the steps below to set up a secure foundation.

### 2.1. First Login & System Update
Access the server using Terminal / PowerShell:
```bash
ssh root@YOUR_SERVER_IP
```
Immediately update the system:
```bash
apt-get update && apt-get upgrade -y
```

### 2.2. Create a New User
We will run the application using a standard user with admin access rights (sudo).
```bash
# Create a new user named "grokdeploy" (you are free to change its name)
adduser grokdeploy

# Add the user to the sudo group so they can execute admin commands
usermod -aG sudo grokdeploy
```

### 2.3. Add Swap (Important for 1GB / 2GB RAM)
Adding Swap will save the server from *crashes/Out of Memory* when there are data spikes.
```bash
# Create a 2GB swap file
fallocate -l 2G /swapfile
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile

# Make the swap permanent across server restarts
echo '/swapfile none swap sw 0 0' | tee -a /etc/fstab
```

### 2.4. Basic Security: UFW (Firewall)
Ensure only the ports we actually use (SSH, HTTP, HTTPS) can be accessed from the outside world.
```bash
# Install ufw if it's not already present (Ubuntu usually has it built-in)
apt-get install ufw -y

# Open OpenSSH access before enabling the firewall to avoid lockouts!
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw deny 8080/tcp
ufw deny 8191/tcp

# Enable the firewall
ufw enable
```
Type `y` and press Enter to effectively enable it.

### 2.5. Switch to the New User
Close the root connection or *switch* directly to the user you just registered:
```bash
su - grokdeploy
```
*(All the steps below will be run as the `grokdeploy` user)*

---

## 3. Installing Dependencies (Docker & Go)

### 3.1. Install *Requirement Tools*
```bash
sudo apt-get install -y ca-certificates curl gnupg lsb-release git make
```

### 3.2. Install Docker & Docker Compose
GrokPi uses Docker to quarantine dependencies (like proxy/flaresolverr if used).
```bash
# Add the Docker repo key
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# Add Docker to the sources.list
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo $VERSION_CODENAME) stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt-get update
# Install the Engine
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Add the grokdeploy user to the docker group to avoid needing `sudo` with docker-compose
sudo usermod -aG docker $USER
```
**IMPORTANT**: *Logout* and *Login* back into the server via SSH as `ssh grokdeploy@YOUR_SERVER_IP` so the docker group configuration applies.

### 3.3. Install Golang 1.24+ (To Build API)
```bash
sudo rm -rf /usr/local/go
curl -fsSL "https://go.dev/dl/go1.24.1.linux-amd64.tar.gz" -o /tmp/go.tar.gz
sudo tar -C /usr/local -xzf /tmp/go.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```
Check the version (ensure v1.24.x appears): `go version`.

---

## 4. Install GrokPi Lite

Now you are ready to set up the core machine.

### 4.1. Clone the Repository
```bash
# Move to the user's home directory
cd ~
git clone https://github.com/startquick/GrokPi-Lite.git
cd GrokPi-Lite
```

### 4.2. Setup the `config.toml` Configuration
Copy the built-in template:
```bash
cp config.defaults.toml config.toml
nano config.toml
```
Ensure you **replace** `app_key` with an admin password that is secure and hard to guess. *(Example: `app_key = "V3ryStr0ng$P4ssw0rd!"`)*. After editing, save (Ctrl+O, Enter, Ctrl+X).

### 4.3. Run Docker
The Docker script in this repo uses a **multi-stage build** - the Go binary is compiled inside the container, no need for `make build` beforehand:

```bash
# 1. Create database folders and set permissions to be docker writeable
mkdir -p data logs
sudo chown -R 1000:1000 data logs

# 2. Run the Container (the service only binds to localhost)
docker compose up -d --build
```
Perform a quick test to see if the server is running internally: `curl -s http://127.0.0.1:8080/health`.

---

## 5. Custom Domain (Reverse Proxy & HTTPS SSL)

So that *clients* can connect to your server via SSL (`https://api.yourdomain.com`), you can use the Caddy Server. Caddy is highly recommended over NGINX as it will *automatically* issue and rotate SSL certificates for free!

### 5.1. Domain Preparation (A Record)
Log into the settings of Cloudflare / your Domain DNS Provider:
- Create an **A Record** type (e.g.: `api.domainname.com`)
- Target **IP Address**: Fill with your VPS IP.
- **IMPORTANT IF USING CLOUDFLARE**: Ensure the cloud status is grey (*Proxy Status: DNS Only*) during the initial Caddy installation.

### 5.2. Install Caddy Server
Run the commands below one by one:
```bash
sudo apt-get install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt-get update
sudo apt-get install caddy
```

### 5.3. Connect Caddy with GrokPi
Open the system default Caddyfile:
```bash
sudo nano /etc/caddy/Caddyfile
```
Empty all the file's contents (or delete what's not important), and insert the code block below. Replace `api.domainname.com` with your real domain:

```text
api.domainname.com {
    reverse_proxy 127.0.0.1:8080
}
```
Save the file (Ctrl+O, Enter, Ctrl+X), then *restart* the service:
```bash
sudo systemctl restart caddy
```
Caddy will take about ~10 seconds to order a certificate from Let's Encrypt / ZeroSSL.

Congratulations! Your GrokPi Lite server is now live on the public network via `https://api.domainname.com`!

Important notes:
- `docker-compose.yml` only binds GrokPi to `127.0.0.1:8080`, so public access must proceed through Caddy.
- FlareSolverr is not published to the host and is only available on the internal Docker network.
- The container will fail to start if `config.toml` isn't mounted or still uses the default `app_key`.

---

## 6. Daily Operations (Maintenance)

All tokens, API keys, and Grok quotas are stored transparently in `/data/grokpi.db` with the SQLite driver.

### 6.1. Admin Token Settings (CLI via SSH)
Whenever you need to add/remove x.com tokens, or issue API Keys for user *clients*, use the built-in script:
```bash
cd ~/GrokPi-Lite
chmod +x ./scripts/linux/grokpi_admin.sh
./scripts/linux/grokpi_admin.sh
```

### 6.2. Database Backup
Performing a backup is very easy, simply package the configuration and data folder into a single archive:
```bash
cd ~/GrokPi-Lite
tar czf grokpi-backup-$(date +%F).tar.gz config.toml data/
```

### 6.3. Updating the Main Application
When you receive an update notification on Github:
```bash
cd ~/GrokPi-Lite
git pull
docker compose build --no-cache grokpi
docker compose up -d
```
The command `build --no-cache` ensures that the image is purely recompiled from scratch using the newest multi-stage layer.
If you kept local changes in `config.toml` or other deploy files, back them up before updating, and resolve the Git conflicts manually.
