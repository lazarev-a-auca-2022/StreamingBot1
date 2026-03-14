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
- BUNNY_LIBRARY_ID=...
- BUNNY_API_KEY=...
- BUNNY_API_BASE_URL=https://video.bunnycdn.com
- BUNNY_EMBED_BASE_URL=https://iframe.mediadelivery.net/embed
- BUNNY_SYNC_INTERVAL_SEC=300
- BUNNY_DEFAULT_PRICE_STARS=25

Defaults in `.env.example` are suitable for local development.

### Bunny Stream mapping

- `content.external_ref` in DB must contain Bunny `videoId` as bytes/text.
- App validates video existence via Bunny API:
  - `GET {BUNNY_API_BASE_URL}/library/{BUNNY_LIBRARY_ID}/videos/{videoId}`
  - Header: `AccessKey: {BUNNY_API_KEY}`
- Issued access links are Bunny embed URLs:
  - `{BUNNY_EMBED_BASE_URL}/{BUNNY_LIBRARY_ID}/{videoId}`
- Content catalog auto-sync:
  - On startup, app fetches Bunny library videos and upserts into local `content` table.
  - Then it refreshes periodically every `BUNNY_SYNC_INTERVAL_SEC`.
  - New videos get `price_stars = BUNNY_DEFAULT_PRICE_STARS` by default.

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

## 12. User Guide

This section explains how to use the system today.

### 12.1 Current user flow

The project now supports Telegram command interaction and API endpoints.

Flow:
1. User opens bot and runs `/start`.
2. User runs `/catalog` or taps buy button.
3. User starts purchase (`/buy <content_id>` or callback button).
4. Payment provider calls successful payment webhook.
5. Scheduler processes outbox and issues access link.
6. User receives link in Telegram message and opens it.
7. User submits review with `/review <purchase_id> <rating> [text]`.

### 12.1.1 Telegram command menu

Bot menu commands configured via Telegram `setMyCommands`:
- `/start`
- `/catalog`
- `/buy`
- `/review`
- `/help`

### 12.1.2 Hidden admin commands

Admin mode is toggled per user and requires webhook secret in chat:

- `/adminmode <webhook_secret>`

If correct, it toggles admin mode on/off for that Telegram user.

Admin-only commands:

- `/createcontent <id> <bunny_video_id> <price> <title>|<description>`
- `/deletecontent <id>`
- `/setcontent <id> <price> <title>|<description>`
- `/forcebuy <content_id>`

Examples:

`/adminmode change-me`

`/createcontent yoga-101 a1b2c3d4e5 25 Yoga Basics|Beginner class`

`/setcontent yoga-101 30 Yoga Basics Updated|New description`

`/deletecontent yoga-101`

`/forcebuy content-demo-1`

### 12.2 Endpoints used in the flow

Health check:

curl http://localhost:8080/healthz

Get catalog:

curl http://localhost:8080/catalog

Start purchase:

curl -X POST http://localhost:8080/purchase/start \
  -H "Content-Type: application/json" \
  -d '{"user_id":12345,"content_id":"content-demo-1"}'

Example response contains purchase_id and invoice_payload.

Confirm successful payment (normally called by payment provider webhook):

curl -X POST http://localhost:8080/webhook/telegram/successful_payment \
  -H "Content-Type: application/json" \
  -H "X-Telegram-Bot-Api-Secret-Token: <WEBHOOK_SECRET>" \
  -d '{"charge_id":"charge-001","amount_stars":25,"invoice_payload":"<payload-from-start-purchase>","raw_payload":"{}"}'

Use issued access token:

curl -X POST http://localhost:8080/access/use \
  -H "Content-Type: application/json" \
  -d '{"token":"<token-from-access-link>"}'

Submit review:

curl -X POST http://localhost:8080/review/submit \
  -H "Content-Type: application/json" \
  -d '{"user_id":12345,"purchase_id":"<purchase_id>","rating":5,"text":"Great stream"}'

### 12.3 Bunny-specific requirement

For catalog items to be playable, each content item must have external_ref set to a valid Bunny videoId in the database.

### 12.4 Where this flow is implemented

- API handlers: [internal/adapters/httpapi/server.go](internal/adapters/httpapi/server.go)
- Purchase start: [internal/app/start_purchase/handler.go](internal/app/start_purchase/handler.go)
- Payment confirmation: [internal/app/confirm_payment/handler.go](internal/app/confirm_payment/handler.go)
- Access issuance: [internal/app/issue_access/handler.go](internal/app/issue_access/handler.go)
- Access consumption: [internal/app/use_access/handler.go](internal/app/use_access/handler.go)
- Review submission: [internal/app/submit_review/handler.go](internal/app/submit_review/handler.go)

---

## 13. Quick Start Summary

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
