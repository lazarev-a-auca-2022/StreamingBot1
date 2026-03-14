# streamingbot

телеграм-бот на go для безопасного предоставления доступа к стриминговому контенту. пользователь оплачивает доступ через telegram stars, получает одноразовую подписанную ссылку и оставляет отзыв. центральная задача — надёжность, идемпотентность и защита данных.

---

## архитектура

система построена по принципу явного разделения слоёв: domain → application → adapters → platform. каждый слой зависит только от слоя ниже, transport-код никогда не содержит бизнес-правил.

```
┌──────────────────────────────────────────────────────────────┐
│                      telegram client                         │
└─────────────────────────────┬────────────────────────────────┘
                              │ https webhook / polling
┌─────────────────────────────▼────────────────────────────────┐
│               adapters / telegram (transport)                │
│  webhook handler, message parser, rate limiter, bot sender   │
└──────────┬──────────────────┬───────────────────────────────┘
           │                  │
           ▼                  ▼
┌──────────────────────────────────────────────────────────────┐
│               application layer (use cases)                  │
│  StartPurchase · ConfirmPayment · IssueAccessLink            │
│  ExpireAccess  · RequestReview  · SubmitReview               │
└──────────┬──────────────────┬───────────────────────────────┘
           │                  │
           ▼                  ▼
┌──────────────────────────────────────────────────────────────┐
│                      domain layer                            │
│  user · content · purchase · access · review                 │
│  (сущности, инварианты, переходы состояний)                  │
└──────────┬──────────────────┬───────────────────────────────┘
           │                  │
           ▼                  ▼
┌────────────────────┐  ┌─────────────────────────────────────┐
│ adapters/storage   │  │  adapters/streaming                 │
│ postgres · redis   │  │  http-клиент к внешнему api         │
└────────────────────┘  └─────────────────────────────────────┘
           │
           ▼
┌──────────────────────────────────────────────────────────────┐
│                  platform (инфраструктура)                   │
│  config · logger · crypto · clock · idgen                    │
└──────────────────────────────────────────────────────────────┘
```

---

## структура проекта

```
streamingbot/
├── cmd/
│   └── bot/
│       └── main.go                      # точка входа, wire зависимостей
│
├── internal/
│   │
│   ├── domain/                          # бизнес-правила, без зависимостей на фреймворки
│   │   ├── user/
│   │   │   ├── user.go                  # сущность user, инварианты
│   │   │   └── repository.go           # интерфейс репозитория
│   │   ├── content/
│   │   │   ├── content.go               # сущность content (id, title, price_stars)
│   │   │   └── repository.go
│   │   ├── purchase/
│   │   │   ├── purchase.go              # сущность purchase, fsm состояний
│   │   │   ├── states.go                # pending → paid → access_issued → expired | refunded
│   │   │   └── repository.go
│   │   ├── access/
│   │   │   ├── access_grant.go          # сущность access_grant (token, ttl, used_at)
│   │   │   └── repository.go
│   │   └── review/
│   │       ├── review.go                # сущность review (rating, text, published)
│   │       └── repository.go
│   │
│   ├── app/                             # use cases: координируют домен и адаптеры
│   │   ├── start_purchase.go            # создать purchase, инициировать invoice
│   │   ├── confirm_payment.go           # идемпотентная обработка successful_payment
│   │   ├── issue_access_link.go         # выпустить access_grant, отправить ссылку
│   │   ├── expire_access.go             # фоновая инвалидация просроченных grant-ов
│   │   ├── request_review.go            # scheduler: запросить отзыв через n часов
│   │   └── submit_review.go             # сохранить отзыв, опубликовать при необходимости
│   │
│   ├── adapters/
│   │   ├── telegram/
│   │   │   ├── webhook.go               # приём и верификация webhook от telegram
│   │   │   ├── handler.go               # маршрутизация команд и callback
│   │   │   ├── sender.go                # отправка сообщений пользователю
│   │   │   └── middleware.go            # rate limiting, валидация, логирование запросов
│   │   ├── streaming/
│   │   │   ├── provider.go              # интерфейс StreamingProvider
│   │   │   └── client.go                # http-клиент к внешнему streaming api
│   │   └── storage/
│   │       ├── postgres/
│   │       │   ├── user_repo.go
│   │       │   ├── content_repo.go
│   │       │   ├── purchase_repo.go
│   │       │   ├── access_repo.go
│   │       │   ├── review_repo.go
│   │       │   └── event_log_repo.go    # audit trail платёжных событий
│   │       └── redis/
│   │           ├── session_store.go     # короткоживущие сессии бота
│   │           └── token_store.go       # одноразовые access токены (ttl, инвалидация)
│   │
│   └── platform/
│       ├── config/
│       │   └── config.go                # загрузка конфигурации из env
│       ├── logger/
│       │   └── logger.go                # структурированное логирование без чувствительных данных
│       ├── crypto/
│       │   ├── token.go                 # генерация hmac-подписанных токенов
│       │   └── encrypt.go              # aes-256-gcm шифрование полей
│       ├── clock/
│       │   └── clock.go                 # абстракция времени (тестируемость)
│       └── idgen/
│           └── idgen.go                 # генерация uuid v7 / ulid
│
├── migrations/                          # sql-миграции (golang-migrate)
├── scheduler/
│   └── scheduler.go                     # планировщик фоновых задач (cron / worker)
├── .env.example
├── docker-compose.yml
└── Dockerfile
```

---

## доменные сущности и их инварианты

### user
| поле        | тип       | описание                       |
|-------------|-----------|--------------------------------|
| id          | int64     | telegram user id (primary key) |
| username    | text      | может быть пустым              |
| created_at  | timestamp |                                |
| banned      | bool      | заблокированные не получают ссылки |

### content
| поле         | тип    | описание                                         |
|--------------|--------|--------------------------------------------------|
| id           | uuid   | внутренний id, никогда не раскрывается клиенту   |
| external_ref | text   | зашифрованный идентификатор у стриминг-провайдера |
| title        | text   |                                                  |
| price_stars  | int    | цена в telegram stars                            |
| active       | bool   | снятый с продажи контент не принимает оплату      |

**инвариант:** external_ref хранится только в зашифрованном виде (aes-256-gcm); ключ — в env, не в бд.

### purchase (центральная сущность заказа)
```
pending ──► paid ──► access_issued ──► expired
                 └──► refunded
```
| поле                | тип       | описание                                  |
|---------------------|-----------|-------------------------------------------|
| id                  | uuid      |                                           |
| user_id             | int64     |                                           |
| content_id          | uuid      |                                           |
| status              | enum      | pending / paid / access_issued / expired / refunded |
| telegram_payload    | text      | payload из invoice, используется как idempotency key |
| telegram_charge_id  | text      | уникальный id от telegram, хранится для дедупликации |
| stars_amount        | int       |                                           |
| created_at          | timestamp |                                           |
| paid_at             | timestamp |                                           |

**инвариант:** переход в `paid` возможен только если `telegram_charge_id` ещё не встречался в таблице (идемпотентость). переход в `access_issued` возможен только из `paid`.

### access_grant
| поле          | тип       | описание                                        |
|---------------|-----------|-------------------------------------------------|
| id            | uuid      |                                                 |
| purchase_id   | uuid      |                                                 |
| user_id       | int64     |                                                 |
| token_hash    | text      | sha-256 хэш токена; сам токен никогда не хранится |
| issued_at     | timestamp |                                                 |
| expires_at    | timestamp |                                                 |
| used_at       | timestamp | null = ещё не использован                      |

**инвариант:** один purchase — один access_grant. ссылка одноразовая: после первого перехода по ссылке `used_at` проставляется и ссылка инвалидируется в redis.

### review
| поле           | тип    |
|----------------|--------|
| id             | uuid   |
| user_id        | int64  |
| purchase_id    | uuid   |
| rating         | 1–5    |
| text           | text   |
| published      | bool   |
| created_at     | timestamp |

---

## ключевые use cases

### confirmPayment(paymentEvent)
```
1. извлечь telegram_charge_id из события
2. проверить: существует ли запись с этим charge_id → если да, выйти (идемпотентность)
3. найти purchase по telegram_payload
4. проверить статус purchase = pending
5. перевести purchase в paid, записать charge_id и paid_at
6. сохранить raw event в event_log (audit trail)
7. запустить IssueAccessLink
```

### issueAccessLink(purchaseID)
```
1. загрузить purchase, проверить статус = paid
2. загрузить content, получить зашифрованный external_ref
3. запросить у StreamingProvider уникальную ссылку (передать external_ref, ttl)
4. сгенерировать токен (crypto/token), сохранить token_hash в access_grant
5. положить токен в redis с ttl (короткоживущий, первичная истина — postgres)
6. перевести purchase в access_issued
7. отправить ссылку пользователю через telegram sender
8. поставить задачу в scheduler: через n часов запросить отзыв
```

### requestReview(purchaseID) — фоновая задача
```
1. проверить, что отзыв ещё не оставлен
2. отправить пользователю сообщение с кнопкой оценки
3. сохранить состояние ожидания отзыва в session_store
```

---

## безопасность

### управление ключами и секретами
- все секреты передаются только через переменные окружения, в коде нет хардкода
- `encryption_key` (aes-256-gcm) используется для шифрования `external_ref` в бд
- `token_hmac_secret` используется только в `crypto/token.go`, нигде больше не читается
- ротация ключей: новый ключ добавляется как `encryption_key_v2`, старый держится до перешифровки

### защита content catalog
- клиент никогда не видит `content_id` (uuid) или `external_ref` в явном виде
- `external_ref` хранится только зашифрованным; расшифровывается в памяти только в момент запроса ссылки
- идентификаторы контента в telegram invoice payload — случайные ulid, не связанные с внутренней структурой бд

### защита access токенов
- генерируется криптографически случайный токен (32 байта) + подписывается hmac
- в бд хранится только `sha-256(token)`, никогда сам токен
- в redis хранится только хэш токена как ключ с ttl; значение — purchase_id для look-up
- ссылка одноразовая: при первом использовании `used_at` проставляется и ключ из redis удаляется
- replay protection: даже если токен утёк, повторное использование невозможно

### защита базы пользователей
- доступ к postgres только из внутренней docker-сети, порт не экспонируется наружу
- пользователь бд имеет права только на конкретные таблицы (не superuser, не createdb)
- параметризованные запросы везде (sql injection prevention)
- `banned` пользователи не проходят валидацию в middleware до любого use case

### идемпотентность платежей
- `telegram_charge_id` объявлен unique в таблице `purchases`; повторная обработка события возвращает ошибку-дубликат без создания нового access_grant
- все платёжные события логируются в `event_log` с полным raw payload для последующего аудита

### transport-безопасность
- webhook принимается только по https; проверяется `X-Telegram-Bot-Api-Secret-Token`
- rate limiter в middleware: не более n запросов на user_id в единицу времени
- все входные данные валидируются на уровне telegram adapter до передачи в use case
- логирование: токены, ключи, `external_ref` никогда не попадают в логи (zerolog field sanitizer)
- контейнер запускается от non-root пользователя (uid 1000)

---

## согласованность данных: postgres vs redis

| что хранится       | где            | роль                                          |
|--------------------|----------------|-----------------------------------------------|
| все сущности       | postgres       | source of truth, персистентные данные         |
| access_grant       | postgres       | авторитетный статус: использован / просрочен  |
| token → purchase mapping | redis  | short-lived, ttl = ttl ссылки; при miss — fallback в postgres |
| сессии бота        | redis          | user fsm state (ожидание ввода, ожидание отзыва) |

redis — исключительно performance и ttl слой. потеря redis не приводит к потере данных: все критические состояния хранятся в postgres.

---

## фоновые задачи

за запрос отзывов и инвалидацию просроченных access_grant отвечает отдельный компонент `scheduler/`. он запускается как горутина внутри того же процесса (допустимо для mvp) или выносится в отдельный worker процесс при росте нагрузки.

| задача                    | триггер                               |
|---------------------------|---------------------------------------|
| request_review            | через n часов после `access_issued`   |
| expire_access             | по расписанию (каждые m минут)        |

---

## переменные окружения

| переменная               | описание                                                  |
|--------------------------|-----------------------------------------------------------|
| `bot_token`              | токен telegram-бота                                       |
| `webhook_secret`         | секрет для верификации `x-telegram-bot-api-secret-token`  |
| `database_url`           | строка подключения к postgresql                           |
| `redis_url`              | строка подключения к redis                                |
| `streaming_api_url`      | базовый url внешнего streaming api                        |
| `streaming_api_key`      | ключ доступа к streaming api (передаётся в заголовке)     |
| `token_hmac_secret`      | секрет для hmac-подписи access токенов (32+ байт, hex)    |
| `encryption_key`         | ключ aes-256-gcm для полей content.external_ref (32 байт) |
| `access_link_ttl_minutes`| время жизни выданной ссылки в минутах                     |
| `review_delay_hours`     | через сколько часов после просмотра запрашивать отзыв     |
| `rate_limit_per_minute`  | максимальное число запросов от одного user_id в минуту    |

---

## технологический стек

| компонент      | технология                            |
|----------------|---------------------------------------|
| язык           | go 1.22+                              |
| telegram api   | go-telegram-bot-api v5                |
| бд             | postgresql 15+                        |
| кэш / ttl      | redis 7+                              |
| миграции       | golang-migrate                        |
| логирование    | zerolog                               |
| конфигурация   | godotenv + os.getenv                  |
| контейнеры     | docker + docker compose               |

---

## запуск

```bash
cp .env.example .env
# заполнить .env реальными значениями
docker compose up -d
```

для локальной разработки без docker:

```bash
go mod download
go run ./cmd/bot
```

---

## sql-схема

```sql
create table users (
    id         bigint primary key,
    username   text,
    created_at timestamptz not null default now(),
    banned     boolean not null default false
);

create table content (
    id            uuid primary key default gen_random_uuid(),
    external_ref  bytea not null,        -- aes-256-gcm зашифрованный идентификатор
    title         text not null,
    price_stars   integer not null check (price_stars > 0),
    active        boolean not null default true
);

create table purchases (
    id                  uuid primary key default gen_random_uuid(),
    user_id             bigint not null references users(id),
    content_id          uuid not null references content(id),
    status              text not null default 'pending',
    telegram_payload    text not null unique,   -- idempotency key из invoice
    telegram_charge_id  text unique,            -- проставляется при successful_payment
    stars_amount        integer not null,
    created_at          timestamptz not null default now(),
    paid_at             timestamptz
);

create table access_grants (
    id           uuid primary key default gen_random_uuid(),
    purchase_id  uuid not null unique references purchases(id),
    user_id      bigint not null references users(id),
    token_hash   text not null unique,          -- sha-256 от токена
    issued_at    timestamptz not null default now(),
    expires_at   timestamptz not null,
    used_at      timestamptz                    -- null = не использован
);

create table reviews (
    id           uuid primary key default gen_random_uuid(),
    user_id      bigint not null references users(id),
    purchase_id  uuid not null unique references purchases(id),
    rating       smallint not null check (rating between 1 and 5),
    text         text,
    published    boolean not null default false,
    created_at   timestamptz not null default now()
);

-- audit trail: все raw события от telegram payments
create table payment_events (
    id           uuid primary key default gen_random_uuid(),
    charge_id    text not null,
    raw_payload  jsonb not null,
    received_at  timestamptz not null default now()
);
```
