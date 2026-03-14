# streamingbot

телеграм-бот на go для безопасного предоставления доступа к стриминговому контенту. пользователь оплачивает доступ через telegram stars, получает одноразовую подписанную ссылку и оставляет отзыв. центральная задача — надёжность, идемпотентность и защита данных.

---

## архитектура

Система построена на **явных bounded contexts** из DDD (Domain-Driven Design). Каждый слой имеет чёткую ответственность и не нарушает границы:

- **platform/** — инфраструктура и утилиты (config, logger, crypto)
- **domain/** — 5 независимых bounded contexts (user, content, purchase, access, review)
- **app/** — use cases, оркестрирующие домены
- **adapters/** — интеграция с внешним миром (telegram, streaming provider, databases)

```
              Telegram Webhook (HTTPS)
                      │
                      ▼
          adapters/telegram/webhook.go
       (verify, parse, duplicate detection)
                      │
                      ▼
    app/confirm_telegram_payment/handler.go
  (ConfirmPaymentCommand: idempotency, state machine)
       domain/purchase + event_log + outbox
                      │
                      ▼
      jobs/scheduler.go (outbox processor)
         PurchaseConfirmed event
                      │
                      ▼
      app/issue_access_link/handler.go
     (call streaming provider, create token, cache)
       domain/access + adapters/storage
```

**Ключевое правило**: бизнес-логика живёт в **domain/** и **app/**, адаптеры — это просто порты ввода-вывода.

---

## идемпотентность платежей (inbox/outbox pattern)

Входящие платёжные события от Telegram могут приходить дважды (network retries). **Решение**: inbox/outbox.

```sql
create table idempotency_keys (
    event_id     text primary key,        -- telegram_charge_id
    event_type   text not null,
    processed_at timestamptz not null default now()
);

create table outbox (
    id           uuid primary key default gen_random_uuid(),
    event_type   text not null,           -- 'PurchaseConfirmed'
    aggregate_id uuid not null,           -- purchase_id
    payload      jsonb not null,
    published    boolean not null default false,
    created_at   timestamptz not null default now()
);
```

**Webhook handler**: 1) CHECK is_processed(charge_id) → если да, return 200. 2) Если нет → ConfirmPayment + mark idempotency_key. 3) Publish to outbox (PurchaseConfirmed).

**Результат**: любое кол-во дубликатов webhook'ов = Purchase создан один раз.

---

## redis vs postgres: source of truth

| Данные | PRIMARY | Cache | Потеря = |
|--------|---------|-------|----------|
| Все бизнес-данные | **Postgres** | - | ❌ потеря данных |
| AccessGrant token_hash lookup | Postgres | Redis (TTL) | ✓ fallback SELECT |
| Session state | - | Redis | ✓ user вводит заново |

**Redis = performance layer, не source of truth**. На потере redis → fallback в postgres (медленно, но работает).

---

## асинхронность (outbox publisher + scheduler)

Outbox publisher читает `outbox` таблицу (stored-in-DB guarantee) и публикует события.

```go
func (s *Scheduler) processOutbox(ctx context.Context) {
    events := s.outboxRepo.GetUnpublished(ctx)
    for _, evt := range events {
        s.issueAccessLinkHandler.Handle(ctx, evt.AggregateID)
        s.outboxRepo.MarkPublished(ctx, evt.ID)
    }
}

func (s *Scheduler) Start(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    for {
        select {
        case <-ticker.C:
            s.processOutbox(ctx)
            s.expireAccessGrants(ctx)
            s.requestPendingReviews(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

**IssueAccessLink** выполняется асинхронно, но гарантированно (stored in outbox table).

---

## ключевые решения и допущения

### Business решения
1. **Telegram Stars как единственный платёж** — мы не переводим деньги, только активируем доступ; отката платежа в архитектуре нет
2. **One-time access links** — ссылка работает один раз, после первого использования `used_at` проставляется и ссылка инвалидируется
3. **Асинхронная выдача ссылки** — `ConfirmPayment` и `IssueAccessLink` — отдельные use cases для достижения консистентности
4. **Отзывы опциональны** — пользователь может не оставить отзыв; bot не блокирует и не напоминает более 1 раза
5. **Нет двойной оплаты одного контента** — в течение TTL ссылки пользователь не может купить тот же контент дважды (check перед ConfirmPayment)
6. **Review = draft by default** — все отзывы сохраняются как `published=false`; publication требует модерации (future admin API)

###架構ные решения
1. **Scheduler в одном процессе (MVP)** — горутина в одном боте выполняет background tasks; при масштабировании выносится в отдельный worker
2. **Redis как performance слой** — source of truth для всех это postgres; redis используется только для TTL и быстрого lookup
3. **IssueAccessLink синхронный** — выполняется immediately после successful_payment (ретрай если упал)
4. **Event log для аудита** — все платёжные события сохраняются в raw виде в `payment_events`
5. **Content management — manual** — администраторы создают контент через SQL миграции; future админ-панель для CRUD

---

## структура проекта

```
streamingbot/
├── cmd/
│   └── bot/
│       └── main.go                      # точка входа, DI, wire зависимостей
│
├── internal/
│   │
│   ├── domain/                          # бизнес-правила, НОЛЬ зависимостей на фреймворки
│   │   ├── user/                        # User bounded context (только базовые данные)
│   │   │   ├── user.go                  # User aggregate root
│   │   │   ├── repository.go            # interface UserRepository
│   │   │   └── errors.go
│   │   ├── content/                     # Content bounded context (каталог, метаданные)
│   │   │   ├── content.go               # Content aggregate root
│   │   │   ├── repository.go
│   │   │   └── errors.go
│   │   ├── purchase/                    # ЦЕНТРАЛЬНЫЙ bounded context: жизненный цикл покупки
│   │   │   ├── purchase.go              # Purchase aggregate root с FSM (pending→paid→issued→expired)
│   │   │   ├── status.go                # enum Status
│   │   │   ├── factory.go               # создание новых Purchase
│   │   │   ├── repository.go
│   │   │   └── errors.go
│   │   ├── access/                      # ДОМЕН ДОСТУПА: одноразовые ссылки/токены
│   │   │   ├── grant.go                 # AccessGrant aggregate root
│   │   │   ├── repository.go
│   │   │   └── errors.go
│   │   └── review/                      # Review bounded context (НЕ часть User!)
│   │       ├── review.go                # Review aggregate
│   │       ├── repository.go
│   │       └── errors.go
│   │
│   ├── app/                             # ORCHESTRATION LAYER: use cases, никакой бизнес-логики
│   │   ├── start_purchase/
│   │   │   ├── command.go
│   │   │   └── handler.go
│   │   ├── confirm_telegram_payment/    # ГЛАВНЫЙ ENTRY POINT для плата
│   │   │   ├── command.go
│   │   │   └── handler.go               # идемпотентная обработка + inbox/outbox
│   │   ├── issue_access_link/
│   │   │   ├── command.go
│   │   │   └── handler.go               # интеграция с StreamingProvider
│   │   ├── request_review/
│   │   │   ├── command.go
│   │   │   └── handler.go
│   │   ├── expire_access/
│   │   │   ├── command.go
│   │   │   └── handler.go
│   │   └── submit_review/
│   │       ├── command.go
│   │       └── handler.go
│   │
│   ├── adapters/                        # ИНТЕГРАЦИЯ С ВНЕШНИМ МИРОМ (входная/выходная)
│   │   ├── telegram/                    # Telegram webhook adapter
│   │   │   ├── webhook.go               # приём и верификация webhook
│   │   │   ├── handler.go               # маршрутизация к use case handlers
│   │   │   ├── sender.go                # отправка сообщений
│   │   │   └── middleware.go            # rate limit, auth, logging
│   │   ├── streaming/                   # Streaming provider adapter
│   │   │   ├── provider.go              # interface StreamingProvider (АДАПТЕР!)
│   │   │   ├── client.go                # реализация: http клиент к external API
│   │   │   └── errors.go
│   │   └── storage/                     # Persistence adapters
│   │       ├── postgres/                # Postgres repository implementations
│   │       │   ├── user_repo.go
│   │       │   ├── content_repo.go
│   │       │   ├── purchase_repo.go
│   │       │   ├── access_repo.go
│   │       │   ├── review_repo.go
│   │       │   ├── event_log_repo.go    # audit trail платёжных событий
│   │       │   └── idempotency_key_repo.go # inbox pattern
│   │       └── redis/                   # Redis: PERFORMANCE LAYER (кэш, НЕ source of truth!)
│   │           ├── token_store.go       # AccessGrant.token_hash lookup + TTL
│   │           └── session_store.go     # краткоживущие сессии бота
│   │
│   ├── platform/                        # ИНФРАСТРУКТУРА (internal/platform, не pkg/!)
│   │   ├── config/
│   │   │   └── config.go                # загрузка из env
│   │   ├── logger/
│   │   │   └── logger.go                # структурированное логирование (redaction)
│   │   ├── crypto/
│   │   │   ├── token.go                 # Token: generate + validate (HMAC-SHA256)
│   │   │   └── encrypt.go              # AES-256-GCM шифрование external_ref
│   │   ├── clock/
│   │   │   └── clock.go                 # абстракция времени
│   │   └── idgen/
│   │       └── idgen.go                 # UUID/ULID generation
│   │
│   └── jobs/                            # АСИНХРОННЫЕ ЗАДАЧИ
│       ├── scheduler.go                 # главный scheduler (outbox processor + cleanup)
│       └── commands.go                  # внутренние команды для фоновых работ
│
├── migrations/                          # SQL migrations (golang-migrate)
├── .env.example
├── docker-compose.yml
└── Dockerfile
```

**Замечание по структуре**: 
- `internal/platform/` а не `pkg/` — потому что это инфраструктура **этого приложения**, не переиспользуемая библиотека
- Каждый `domain/*`완독 независим и имеет свой repository интерфейс
- `adapters/` содержит реализации, бизнес-логика живёт в domain + app

---

## bounded contexts (явные домены ответственности)

### domain/user — только пользовательские данные
Отвечает за: регистрацию, базовые данные, бан-статус.
НЕ отвечает за: историю покупок, отзывы, сессии.

### domain/content — каталог и метаданные
Отвечает за: контент, цены, активность. `external_ref` ВСЕГДА зашифрован.

### domain/purchase — ЦЕНТРАЛЬНАЯ сущность заказа
Отвечает за: жизненный цикл: pending → paid → access_issued → expired | error.
Инварианты:
- `telegram_charge_id` уникален (защита от двойной оплаты)
- Переход `paid → access_issued` только если AccessGrant создан
- На критических ошибках: переход → error (требует retry)

### domain/access — одноразовый доступ
Отвечает за: жизненный цикл токена.
Инварианты:
- `token_hash` уникален; сам token НИКОГДА не храним
- Одноразовость: первое использование → `used_at = now()`, дальше error
- Валидность определяется `expires_at`, а НЕ redis TTL

### domain/review — отзывы НЕ принадлежат User
Отвечает за: отзывы к покупкам.
Инвариант: связан с Purchase, не User.

---

## application layer (use cases / orchestration)

Оркестрирует домены, вызывает адаптеры, управляет транзакциями. **Никогда не содержит бизнес-логику**.

- **ConfirmTelegramPayment**: webhook → Purchase.MarkAsPaid() → audit log → outbox event
- **IssueAccessLink**: outbox event → StreamingProvider → AccessGrant → postgres + redis cache
- **RequestReview**: scheduler → найти Purchase → отправить telegram
- Остальные: по аналогии

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
    │
    └──► error (ошибка при выдаче ссылки, требует retry)
```

| поле                      | тип       | описание                                  |
|---------------------------|-----------|-------------------------------------------|
| id                        | uuid      |                                           |
| user_id                   | int64     |                                           |
| content_id                | uuid      |                                           |
| status                    | enum      | pending / paid / access_issued / expired / error |
| telegram_payload          | text      | JSON {purchase_id, user_id} из invoice; unique idempotency key |
| telegram_charge_id        | text      | уникальный id от telegram, unique для дедупликации |
| stars_amount              | int       |                                           |
| created_at                | timestamp |                                           |
| paid_at                   | timestamp | проставляется при переходе в paid       |
| issue_link_attempts       | int       | количество попыток выдать ссылку (для retry) |
| last_issue_link_error     | text      | последняя ошибка при выдаче ссылки       |
| last_issue_link_at        | timestamp | время последней попытки выдать ссылку    |
| review_requested_at       | timestamp | null = отзыв ещё не запрашивался         |

**инварианты:**
- Переход в `paid`: только если `telegram_charge_id` уникален в таблице (идемпотентность)
- Переход в `access_issued`: только из `paid` и только если `AccessGrant` успешно создан
- Переход в `expired`: автоматически scheduler, когда истёк TTL `access_grant.expires_at`
- Переход в `error`: если `issue_link_attempts >= max_attempts` (обычно 3); требует manual support intervention

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

### confirmPayment(paymentEvent) — идемпотентная обработка платежа

**Ответственность**: application/confirm_payment.go

**Вход**: PaymentEvent{ChargeID, Amount, InvoicePayload, RawPayload, Timestamp}

**Процесс**:
```
1. Валидация: charge_id не пуст, payload валиден
2. Идемпотентность: 
   - Попытка заблокировать (SELECT FOR UPDATE ... WHERE charge_id = ?) 
   - Если уже существует → return OK (идемпотент)
   - Если нет → begin transaction
3. Найти Purchase по telegram_payload (unmarshall JSON {purchase_id, user_id})
4. Проверить: найдена ли, статус = pending, user не забанен, content активен
5. Обновить: purchase.status = paid, charge_id, paid_at
6. Сохранить в payment_events (raw payload для audit trail)
7. Опубликовать событие PurchaseConfirmedEvent (асинхронно запустит IssueAccessLink)
8. Commit transaction
```

**Ошибки**:
- ErrPurchaseNotFound → 400 Bad Request (payload невалиден)
- ErrDuplicateChargeID → 200 OK (идемпотентно; webhook retry от telegram)
- ErrUserBanned → 400 Bad Request
- ErrContentInactive → 400 Bad Request
- ErrDatabaseError → 500 Internal (retry by telegram)

**Note**: На ошибке в шаге 7 (публикация события) платёж уже записан и confirmed. IssueAccessLink может быть переиграна позже из scheduler.

---

### issueAccessLink(purchaseID) — выдача одноразовой ссылки

**Ответственность**: application/issue_access_link.go

**Вход**: purchaseID (UUID)

**Процесс**:
```
1. Загрузить Purchase, проверить status = paid
2. Загрузить Content, расшифровать external_ref в памяти
3. Запросить у StreamingProvider уникальную ссылку
   - Timeout: 5 сек; на ошибке → ErrConnectStreamingProvider
   - Передать: external_ref, ttl_minutes из конфига
4. Сгенерировать криптографический токен (32 random bytes + HMAC-SHA256)
   - Сохранить SHA256(token) → token_hash
5. Создать AccessGrant в postgres:
   - purchase_id, user_id, token_hash, issued_at, expires_at
   - UNIQUE constraint на token_hash
6. Кэшировать в redis: 
   - Redis key: access:token_hash:{hash}:{ulid}
   - Value: {purchase_id, expires_at}
   - TTL: = expires_at
7. Обновить Purchase: status = access_issued
8. Отправить ссылку пользователю через telegram sender
   - На ошибке отправки: логировать, но не откатывать (ссылка уже выдана)
9. Запланировать в scheduler: RequestReview через review_delay_hours
```

**Ошибки**:
- ErrPurchaseNotFound / ErrWrongStatus → 400 (логировать как anomaly)
- ErrConnectStreamingProvider (timeout/5xx) → логировать, increment issue_link_attempts, retry в scheduler через 5 минут
- ErrTelegramSendFailure → логировать (ссылка уже выдана, пользователь может запросить повторно)

**После 3 неудач**: Purchase переходит в status=error, генерируется alert для support.

---

### requestReview(purchaseID) — фоновая задача запроса отзыва

**Ответственность**: application/request_review.go, scheduler/scheduler.go

**Триггер**: Через review_delay_hours после access_issued

**Процесс**:
```
1. Из scheduler: SELECT * FROM purchases 
   WHERE status = access_issued 
   AND review_requested_at IS NULL 
   AND issued_at < now() - interval '{review_delay_hours} hours'
2. Для каждого purchase:
   a. Загрузить user + content (для контекста)
   b. Отправить telegram сообщение с кнопками оценки (1-5 звёзд)
   c. UPDATE purchases SET review_requested_at = now()
3. Сессию ожидания отзыва сохранить в redis (session_store)
```

**Ошибки**:
- ErrTelegramSendFailure → логировать, not retry (отзыв будет запрошен позже)

**После 7 дней**: Если отзыв не дан, scheduler больше не напоминает.

---

### expireAccess() — инвалидация просроченных ссылок

**Ответственность**: scheduler/scheduler.go

**Триггер**: По расписанию каждые 5 минут

**Процесс**:
```
1. SELECT * FROM access_grants WHERE expires_at < now() AND used_at IS NULL
2. Для каждого:
   a. UPDATE access_grants SET used_at = now(), status = expired
   b. DELETE FROM redis key access:token_hash:{hash}:*
   c. UPDATE purchases SET status = expired
```

---

## безопасность (architecture, не checklist)

### управление ключами и секретами
- все секреты передаются только через переменные окружения, в коде нет хардкода
- `encryption_key` (aes-256-gcm) используется для шифрования `external_ref` в бд
- `token_hmac_secret` используется только в `crypto/token.go`, нигде больше не читается
- ротация ключей:
  - Новый ключ добавляется как `ENCRYPTION_KEY_V2` в env
  - Нонс (12 байт) генерируется случайно для каждого шифрования и prepend'ится к ciphertext
  - При дешифровке: читается версия ключа из prefix, выбирается правильный ключ
  - Миграция: batch job перешифровывает `content.external_ref` новым ключом
  - Старый ключ удаляется только после подтверждения успеха миграции

### защита content catalog
- клиент никогда не видит `content_id` (uuid) или `external_ref` в явном виде
- `external_ref` хранится только зашифрованным; расшифровывается в памяти только в момент запроса ссылки
- идентификаторы контента в telegram invoice payload — случайные ulid, не связанные с внутренней структурой бд

### защита access токенов

**Генерация и подпись**:
```
token_raw = crypto/rand.Read(32 bytes)  // криптографически случайные
hmac_sig = HMAC-SHA256(token_raw || purchase_id, TOKEN_HMAC_SECRET)
token_to_send = base64(token_raw || hmac_sig)  // 64 bytes в base64 → ~86 символов
```

**Валидация (constant-time)**:
```
token_raw, hmac_sig = parse_token(user_input)
computed_hmac = HMAC-SHA256(token_raw || purchase_id, TOKEN_HMAC_SECRET)
import "crypto/subtle"
if subtle.ConstantTimeCompare(hmac_sig, computed_hmac) != 1 {
    return ErrInvalidToken  // защита от timing attacks
}
```

**Хранение**:
- в postgres: `access_grants.token_hash = SHA256(token_raw)`  (никогда сам токен)
- в redis: `access:token_hash:{hash}:{ulid} → {purchase_id, expires_at}` с TTL
- при miss redis: fallback→ SELECT FROM postgres и double-check состояния

**Одноразовость**:
- При первом использовании: `UPDATE access_grants SET used_at = now()`
- DELETE из redis сразу
- На повторную попытку: `used_at` уже заполнен → ErrLinkAlreadyUsed

**Replay protection**:
- Даже если токен утёк и перехвачен: `used_at` обновляется атомарно → повторное использование невозможно
- Дополнительно: IP-лог при использовании (future для investigation)

### защита базы пользователей
- доступ к postgres только из внутренней docker-сети, порт не экспонируется наружу
- пользователь бд имеет права только на конкретные таблицы (не superuser, не createdb)
- параметризованные запросы везде (sql injection prevention)
- `banned` пользователи не проходят валидацию в middleware до любого use case

### идемпотентность платежей

**На уровне БД**:
```sql
ALTER TABLE purchases ADD CONSTRAINT purchases_charge_id_unique UNIQUE(telegram_charge_id);
```
Повторная обработка платежа с тем же charge_id вызовет unique constraint violation → обработчик вернёт OK (идемпотент).

**На уровне приложения (webhook handler)**:
```
1. Webhook пришёл с charge_id
2. Попытаться SELECT FOR UPDATE WHERE charge_id = ? LIMIT 1 (блокирующий lock)
3. Если найден и status = paid → return 200 OK (ничего делать не нужно)
4. Если не найден → begin transaction, выполнить ConfirmPayment usecase
5. На успехе: COMMIT, return 200
6. На дубликате (unique violation): return 200 OK (telegram будет ретрайтить, но мы ответим успехом)
```

**Event log**: Все платёжные события сохраняются в `payment_events` с raw payload (для audit trail и recovery).

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

## фоновые задачи (scheduler)

за запрос отзывов и инвалидацию просроченных access_grant отвечает отдельный компонент `scheduler/`. он запускается как горутина внутри того же процесса (MVP) или выносится в отдельный worker процесс при масштабировании.

| задача                    | триггер                               | интервал | идемпотентность |
|---------------------------|---------------------------------------|----------|------------------|
| request_review            | после `access_issued` и по расписанию | 5 мин    | check `review_requested_at` |
| expire_access             | по расписанию                         | 5 мин    | check `used_at` IS NULL |
| retry_issue_link          | issue_link_attempts < max, ошибка > 1h | 5 мин   | check `status = error` |

**Реализация scheduler**:
```go
// scheduler/scheduler.go
type Scheduler struct {
    clock Clock
    db    DB
    telegram TelegramSender
    streaming StreamingProvider
    // ...
}

func (s *Scheduler) Start(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            s.expireAccessLinks(ctx)
            s.requestPendingReviews(ctx)
            s.retryFailedIssueLinks(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (s *Scheduler) expireAccessLinks(ctx context.Context) {
    // SELECT * FROM access_grants WHERE expires_at < now AND used_at IS NULL
    // UPDATE ... SET used_at = now, status = expired
    // DELETE FROM redis
}

func (s *Scheduler) requestPendingReviews(ctx context.Context) {
    // SELECT * FROM purchases 
    // WHERE status = access_issued 
    // AND review_requested_at IS NULL 
    // AND issued_at < now - interval
    // UPDATE review_requested_at, отправить telegram
}

func (s *Scheduler) retryFailedIssueLinks(ctx context.Context) {
    // SELECT * FROM purchases 
    // WHERE status = error 
    // AND issue_link_attempts < 3 
    // AND last_issue_link_at < now - interval '5 min'
    // Переиграть IssueAccessLink use case
}
```

**Graceful shutdown**:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
sched.Start(ctx)  // остановится после context timeout или ctx.Done()
```

---

## переменные окружения

| переменная                    | описание                                                  | пример |
|-------------------------------|-----------------------------------------------------------|---------|
| `BOT_TOKEN`                   | токен telegram-бота                                       | `123456:ABC-DEF` |
| `WEBHOOK_SECRET`              | секрет для верификации `x-telegram-bot-api-secret-token`  | (random 32+ символа) |
| `DATABASE_URL`                | строка подключения к postgresql (DSN)                    | `postgres://user:pass@localhost/streamingbot?sslmode=disable` |
| `REDIS_URL`                   | строка подключения к redis                                | `redis://localhost:6379/0` |
| `STREAMING_API_URL`           | базовый url внешнего streaming api                        | `https://streaming.example.com/api/v1` |
| `STREAMING_API_KEY`           | ключ доступа к streaming api (передаётся в Authorization) | (bearer token) |
| `STREAMING_API_TIMEOUT_SEC`   | timeout запроса к streaming provider (сек)                | `5` |
| `TOKEN_HMAC_SECRET`           | секрет для hmac-подписи access токенов (hex, 32+ байт)    | (openssl rand -hex 32) |
| `ENCRYPTION_KEY`              | ключ aes-256-gcm для content.external_ref (hex, ровно 32 байта | (openssl rand -hex 32) |
| `ENCRYPTION_KEY_V2`           | (optional) ключ для ротации при миграции                  | (для future) |
| `ACCESS_LINK_TTL_MINUTES`     | время жизни выданной ссылки в минутах                     | `1440` (24 часа) |
| `REVIEW_DELAY_HOURS`          | через сколько часов после выдачи ссылки запрашивать отзыв | `24` |
| `RATE_LIMIT_PER_MINUTE`       | макс. запросов от одного user_id в минуту                 | `10` |
| `LOG_LEVEL`                   | уровень логирования (debug/info/warn/error)               | `info` |
| `ENVIRONMENT`                 | окружение (local/dev/staging/prod)                        | `local` |

---

## обработка ошибок и recovery

### IssueAccessLink упал при запросе к StreamingProvider

**Сценарий**: Purchase в статусе `paid`, но ссылка не выдана.

**Обработка**:
1. Catch ошибку в IssueAccessLink use case
2. Increment `purchase.issue_link_attempts++`
3. Save `purchase.last_issue_link_error = error message`
4. Save `purchase.last_issue_link_at = now()`
5. Оставить purchase в статусе `paid` (не переходить в `error` сразу)
6. Logировать с severity=warn
7. Scheduler каждые 5 минут найдёт и переиграет (см. `retryFailedIssueLinks`)

**После 3 неудач**: 
- Перейти в `status = error`
- Отправить alert для support
- Пользователю: сообщение "Ссылка готовится, технический штаб в курсе; попробуйте позже"

### Webhook пришёл дважды (duplicate payment event)

**Обработка** (в webhook handler'е):
1. Извлечь `charge_id` из события
2. SELECT FOR UPDATE FROM purchases WHERE telegram_charge_id = charge_id LIMIT 1
3. Если найден и status = paid → return 200 OK (идемпотент, ничего делать не нужно)
4. Если не найден → выполнить ConfirmPayment
5. На UNIQUE constraint violation → return 200 OK (успех, дубликат)

**Result**: Две тысячи того же события не создадут две ссылки.

### Streaming provider down (недоступен)

**Обработка**:
1. Timeout: все запросы к streaming provider имеют таймаут 5 сек
2. На timeout/5xx → error log, increment attempts, schedule retry
3. Пользователь видит: "Подождите, ссылка готовится" (ждёт scheduler retry)
4. Если после 3 попыток всё ещё упал → support intervention (status = error)

### Redis упал (потеря кэша сессий / токенов)

**Обработка**:
1. При обращении пользователя по токену: попытаться получить из redis
2. На redis miss: fallback → SELECT FROM access_grants WHERE token_hash = ?
3. Double-check: expires_at > now AND used_at IS NULL
4. Если OK → return (медленнее, но функционирует)
5. Если не OK → ErrLinkExpiredOrUsed

**Result**: Потеря redis не приводит к потере данных; только performance degrade.

### Payment записан, но отправка ссылки ошибалась

**Purchase в `paid`, но не выдана**:
1. Purchase не переходит в `access_issued`
2. Scheduler retry'ит IssueAccessLink
3. Если успех → переход в `access_issued`
4. Если ошибка → оказывается в status = error

**Пользователь**:
- После оплаты видит: "Ссылка готовится, обновите через минуту"
- Если scheduler выпустил ссылку → получит сообщение с ней
- Если всё ещё ошибка → "Обратитесь в support" + support contact

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

## тестирование

### Unit tests
- **domain entities**: тесты на все переходы состояния Purchase, инварианты
  - e.g., `TestPurchaseTransition_PendingToPaid`, `TestPurchaseInvariant_ChargeID`
- **crypto**: генерация токенов, HMAC валидация, constant-time comparison
  - e.g., `TestTokenGeneration`, `TestConstantTimeComparison`
- **platform**: конфигурация, логирование без чувствительных данных
  - e.g., `TestConfigLoad`, `TestFieldSanitization`

### Integration tests
- **Полный flow ConfirmPayment → IssueAccessLink**:
  - Используется test DB (testcontainers/docker) и test Redis
  - Проверка: purchase перешёл в paid → access_issued, токен в redis, ссылка отправлена
- **Идемпотентность**: вызвать ConfirmPayment дважды → вторая должна быть идемпотентна (найти существующий purchase)
- **Error recovery**: IssueAccessLink упал → scheduler retry → успех

### Contract tests
- **StreamingProvider adapter**: mock внешнего API, проверка корректности запросов и обработки ошибок
- **TelegramAdapter**: mock telegram API для webhook и отправки сообщений

### E2E tests (optional for MVP)
- Реальный Telegram webhook в test env
- Full cycle: платёж → ссылка → использование → отзыв

---

## масштабирование (roadmap future)

### MVP (текущая архитектура)
| Компонент | Решение | Лимиты |
|-----------|---------|--------|
| Scheduler | Горутина в боте | 1 процесс; на перезагрузке теряются pending задачи |
| Job queue | Нет; immediate IssueAccessLink | Синхронный flow; на ошибке нужен retry |
| Payment backend | Telegram Stars | Только telegram |
| Content management | Manual SQL миграции | Требует deployment |
| Monitoring | Логи в stdout | Нет metrics |

### Production (phase 2)
| Компонент | Решение | Преимущество |
|-----------|---------|-------------|
| Scheduler | Redis-based job queue (gocraft/work) | Distributed, persistent, retry built-in |
| IssueAccessLink | Async event publishing | Immediate response; background processing |
| Payment backend | Wrapper, multi-provider (Telegram + Stripe/Yandex) | Redundancy, lower fees |
| Content management | Admin REST API + web UI | Self-service, no deployment |
| Monitoring | Prometheus metrics + Grafana | Visibility, alerts |

### High-load (phase 3)
| Компонент | Решение |
|-----------|--------|
| Scheduler | Nomad/Kubernetes cronjobs |
| Job queue | Kafka event streaming |
| Database | Postgres replicas, pgBouncer |
| Cache | Redis Cluster |
| CDN | Content distribution |

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

## примеры кода (pseudocode critical paths)

### ConfirmPayment use case (application/confirm_payment.go)
```go
type ConfirmPaymentUseCase struct {
    purchaseRepo purchase.Repository
    eventLog     EventLog
    eventPublisher EventPublisher  // для async IssueAccessLink
    logger Logger
}

func (uc *ConfirmPaymentUseCase) Execute(ctx context.Context, event PaymentEvent) error {
    // 1. Валидация
    if event.ChargeID == "" {
        return ErrInvalidPaymentEvent
    }

    // 2. Идемпотентность: заблокировать обработку дубликата
    p, err := uc.purchaseRepo.FindAndLockByChargeID(ctx, event.ChargeID)
    if err == nil {
        // Уже обработан
        uc.logger.Warn("duplicate payment", zap.String("charge_id", event.ChargeID))
        return nil  // Идемпотентно
    }
    if err != ErrNotFound {
        return err
    }

    // 3. Получить purchase по invoice payload
    invoice := ParseInvoicePayload(event.InvoicePayload)  // {purchase_id, user_id}
    p, err := uc.purchaseRepo.GetByID(ctx, invoice.PurchaseID)
    if err != nil {
        return ErrPurchaseNotFound
    }

    // 4. Проверить инварианты
    if p.Status != purchase.Pending {
        return ErrInvalidPurchaseStatus
    }
    if p.UserID != invoice.UserID {
        return ErrPayloadMismatch
    }

    // 5. Обновить статус
    p.MarkAsPaid(event.ChargeID, uc.clock.Now())
    if err := uc.purchaseRepo.Save(ctx, p); err != nil {
        return err
    }

    // 6. Сохранить в event log (audit trail)
    uc.eventLog.Log(ctx, PaymentConfirmedEvent{
        PurchaseID: p.ID,
        ChargeID:   event.ChargeID,
        Amount:     event.Amount,
        RawPayload: event.RawPayload,
        Timestamp:  uc.clock.Now(),
    })

    // 7. Опубликовать event (асинхронно запустит IssueAccessLink)
    uc.eventPublisher.Publish(ctx, PurchaseConfirmedEvent{
        PurchaseID: p.ID,
    })

    uc.logger.Info("payment confirmed", zap.String("purchase_id", p.ID))
    return nil
}
```

### Token generation and validation (platform/crypto/token.go)
```go
const TokenByteSize = 32

func GenerateToken() (string, error) {
    raw := make([]byte, TokenByteSize)
    if _, err := crand.Read(raw); err != nil {
        return "", err
    }
    
    sig := hmac.New(sha256.New, []byte(os.Getenv("TOKEN_HMAC_SECRET")))
    sig.Write(raw)
    sigBytes := sig.Sum(nil)
    
    // token = raw || sig (64 bytes total)
    tokenBytes := append(raw, sigBytes...)
    return base64.StdEncoding.EncodeToString(tokenBytes), nil
}

func ValidateToken(tokenStr string, expectedPurchaseID string) bool {
    tokenBytes, err := base64.StdEncoding.DecodeString(tokenStr)
    if err != nil || len(tokenBytes) != TokenByteSize + sha256.Size {
        return false
    }
    
    raw := tokenBytes[:TokenByteSize]
    sig := tokenBytes[TokenByteSize:]
    
    computed := hmac.New(sha256.New, []byte(os.Getenv("TOKEN_HMAC_SECRET")))
    computed.Write(raw)
    
    // Constant-time comparison (защита от timing attacks)
    return subtle.ConstantTimeCompare(sig, computed.Sum(nil)) == 1
}
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
    id                      uuid primary key default gen_random_uuid(),
    user_id                 bigint not null references users(id),
    content_id              uuid not null references content(id),
    status                  text not null default 'pending',
    telegram_payload        text not null unique,   -- JSON {purchase_id, user_id}; idempotency key
    telegram_charge_id      text unique not null,   -- unique per telegram payment; проставляется при successful_payment
    stars_amount            integer not null,
    created_at              timestamptz not null default now(),
    paid_at                 timestamptz,
    issue_link_attempts     integer not null default 0,
    last_issue_link_error   text,
    last_issue_link_at      timestamptz,
    review_requested_at     timestamptz
);

create index idx_purchases_status_created on purchases(status, created_at desc);
create index idx_purchases_user_content on purchases(user_id, content_id);

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
    charge_id    text not null unique,
    purchase_id  uuid references purchases(id),
    raw_payload  jsonb not null,
    received_at  timestamptz not null default now()
);

create index idx_payment_events_charge_id on payment_events(charge_id);
```
