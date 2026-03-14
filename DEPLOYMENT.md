# StreamingBot Deployment Guide

This guide describes how to run StreamingBot:
- locally (without Docker)
- on a remote Linux server (with Docker Compose)

It also includes update, troubleshooting, and rollback steps.

---

## 1. Prerequisites

### Local (no Docker)
- Go 1.22+
- PostgreSQL 15+
- Redis 7+

### Remote Linux (Docker)
- Docker Engine 24+
- Docker Compose v2+
- Open ports:
  - 8080 (API)
  - 443 (if reverse proxy/SSL is configured)

---

## 2. Environment Variables

Copy and edit environment file:

```bash
cp .env.example .env
```

Minimum required values:

- ENVIRONMENT=local or production
- HTTP_ADDR=:8080
- BOT_TOKEN=...
- WEBHOOK_SECRET=...
- DATABASE_URL=postgres://...
- REDIS_URL=redis://...
- STREAMING_API_URL=...
- STREAMING_API_KEY=...

Defaults in `.env.example` are suitable for local development.

---

## 3. Local Run (No Docker)

1. Start PostgreSQL and Redis locally.
2. Create/update `.env`.
3. Run:

```bash
go mod tidy
go run ./cmd/bot
```

4. Health check:

```bash
curl http://localhost:8080/healthz
```

Expected response:

```json
{"status":"ok"}
```

---

## 4. Remote Linux Deployment (Docker Compose)

### 4.1. Initial deploy

1. Copy project to server:

```bash
rsync -avz ./ user@your-server:/opt/streamingbot/
```

2. SSH into server:

```bash
ssh user@your-server
cd /opt/streamingbot
```

3. Prepare env:

```bash
cp .env.example .env
nano .env
```

4. Start stack:

```bash
docker compose up -d --build
```

5. Verify containers:

```bash
docker compose ps
```

6. Verify app health:

```bash
curl http://127.0.0.1:8080/healthz
```

---

## 5. Optional: Run Behind Nginx + HTTPS

Recommended production setup:
- Nginx as reverse proxy on 80/443
- TLS via Let's Encrypt
- Proxy pass to `http://127.0.0.1:8080`

Minimal nginx site example:

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## 6. Telegram Webhook Setup

After deployment, set webhook to your public HTTPS endpoint.

Example endpoint path:
- `/webhook/telegram/successful_payment`

Server verifies header:
- `X-Telegram-Bot-Api-Secret-Token`

Ensure it matches `WEBHOOK_SECRET` in `.env`.

---

## 7. Update Procedure (Zero-Downtime-ish)

On server:

```bash
cd /opt/streamingbot
git pull
docker compose build app
docker compose up -d app
```

Check logs:

```bash
docker compose logs -f --tail=200 app
```

---

## 8. Rollback Procedure

If new version is bad:

1. Checkout previous commit/tag:

```bash
git checkout <previous-tag-or-commit>
```

2. Rebuild and restart app:

```bash
docker compose build app
docker compose up -d app
```

3. Re-run health check.

---

## 9. Useful Commands

### Logs

```bash
docker compose logs -f app
docker compose logs -f postgres
docker compose logs -f redis
```

### Restart

```bash
docker compose restart app
```

### Stop / Start Stack

```bash
docker compose down
docker compose up -d
```

### Check DB connectivity from app container

```bash
docker compose exec app sh -c 'echo ok'
```

---

## 10. Troubleshooting

### App exits immediately
- Check env file values.
- Check DB and Redis URLs.
- Verify `BOT_TOKEN` and `WEBHOOK_SECRET` are not empty in production.

### Health check fails
- Inspect app logs:

```bash
docker compose logs --tail=200 app
```

- Verify app is listening on `HTTP_ADDR`.

### Payment webhook rejected (401)
- `WEBHOOK_SECRET` mismatch.
- Ensure Telegram sends `X-Telegram-Bot-Api-Secret-Token` correctly.

### Redis errors
- Ensure Redis container is healthy:

```bash
docker compose ps
```

### Database errors
- Ensure postgres container is healthy.
- Check credentials in `DATABASE_URL`.

---

## 11. Security Checklist (Production)

- Use strong random values for `BOT_TOKEN` and `WEBHOOK_SECRET`.
- Do not commit `.env`.
- Restrict server firewall to required ports only.
- Put service behind HTTPS reverse proxy.
- Rotate secrets periodically.
- Back up postgres data volume.

---

## 12. Quick Start Summary

### Local:

```bash
cp .env.example .env
go mod tidy
go run ./cmd/bot
curl http://localhost:8080/healthz
```

### Remote Linux:

```bash
cp .env.example .env
# edit .env
docker compose up -d --build
curl http://127.0.0.1:8080/healthz
```
