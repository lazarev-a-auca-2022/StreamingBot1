# streamingbot

телеграм-бот на go для безопасного предоставления доступа к стриминговому контенту через одноразовые ссылки с оплатой через telegram stars.

---

## описание проекта

бот принимает оплату от пользователя, запрашивает у стриминговой системы уникальную ссылку на контент и возвращает её пользователю. после просмотра бот собирает обратную связь. основной приоритет — безопасность: защита базы пользователей и каталога контента.

---

## архитектура

### компоненты системы

```
┌─────────────────────────────────────────────────────────────────┐
│                        telegram client                          │
└───────────────────────────────┬─────────────────────────────────┘
                                │ telegram bot api (webhooks / polling)
┌───────────────────────────────▼─────────────────────────────────┐
│                        bot gateway layer                        │
│  - обработка входящих сообщений и событий                       │
│  - роутинг команд (/start, /pay, /review и др.)                 │
│  - валидация входных данных                                     │
└──────────┬────────────────────┬────────────────────────────────┘
           │                    │
┌──────────▼──────────┐  ┌──────▼──────────────────────────────┐
│   payment service   │  │          user service               │
│                     │  │                                     │
│  - обработка        │  │  - регистрация пользователей        │
│    telegram stars   │  │  - хранение истории покупок         │
│  - верификация      │  │  - управление сессиями              │
│    транзакций       │  │  - сбор отзывов                     │
│  - возврат средств  │  │                                     │
└──────────┬──────────┘  └──────┬──────────────────────────────┘
           │                    │
┌──────────▼────────────────────▼────────────────────────────────┐
│                      streaming service                         │
│  - запрос уникальных ссылок у стриминговой системы             │
│  - управление временем жизни ссылок (ttl)                      │
│  - привязка ссылки к конкретному пользователю и транзакции     │
│  - инвалидация использованных / просроченных ссылок            │
└──────────────────────────────┬─────────────────────────────────┘
                               │
┌──────────────────────────────▼─────────────────────────────────┐
│                       data layer                               │
│  postgresql (основная бд)  │  redis (кэш сессий и ссылок)     │
└────────────────────────────────────────────────────────────────┘
```

---

### структура проекта

```
streamingbot/
├── cmd/
│   └── bot/
│       └── main.go              # точка входа, инициализация зависимостей
├── internal/
│   ├── bot/
│   │   ├── handler.go           # обработчики команд и callback-ов
│   │   ├── middleware.go        # валидация, логирование, rate limiting
│   │   └── router.go            # маршрутизация обновлений
│   ├── payment/
│   │   ├── service.go           # логика обработки telegram stars
│   │   └── verifier.go          # верификация платёжных событий
│   ├── streaming/
│   │   ├── client.go            # http-клиент к стриминговой системе
│   │   └── service.go           # логика получения и выдачи ссылок
│   ├── user/
│   │   ├── repository.go        # работа с бд: crud для пользователей
│   │   └── service.go           # бизнес-логика пользователей
│   ├── review/
│   │   ├── repository.go        # хранение отзывов
│   │   └── service.go           # сбор и публикация отзывов
│   └── config/
│       └── config.go            # загрузка конфигурации из env
├── pkg/
│   ├── crypto/
│   │   └── token.go             # генерация подписанных токенов для ссылок
│   ├── logger/
│   │   └── logger.go            # структурированное логирование (zerolog)
│   └── db/
│       ├── postgres.go          # подключение к postgresql
│       └── redis.go             # подключение к redis
├── migrations/                  # sql-миграции схемы бд
├── .env.example                 # пример переменных окружения
├── docker-compose.yml
├── Dockerfile
└── README.md
```

---

## поток данных (основной сценарий)

```
1. пользователь отправляет /start или выбирает контент
2. бот показывает описание и кнопку оплаты
3. пользователь оплачивает через telegram stars
4. telegram присылает событие successful_payment
5. payment service верифицирует транзакцию
6. streaming service запрашивает у стриминговой системы уникальную ссылку,
   привязанную к user_id + transaction_id + ttl
7. бот отправляет ссылку пользователю в личном сообщении
8. через заданное время (или после просмотра) бот запрашивает отзыв
9. отзыв сохраняется и по необходимости публикуется
```

---

## безопасность

### защита базы пользователей
- все персональные данные хранятся только в postgresql с шифрованием чувствительных полей (aes-256-gcm)
- доступ к бд только из внутренней сети (нет прямого доступа из интернета)
- использование параметризованных запросов (sql injection prevention)
- ротация секретов через переменные окружения, без хардкода в коде
- минимальные права для пользователя бд (только необходимые таблицы)

### защита каталога контента
- идентификаторы контента не раскрываются напрямую пользователю
- ссылки генерируются динамически и подписываются hmac
- каждая ссылка привязана к конкретному user_id и имеет ограниченный ttl
- одна транзакция — одна ссылка (предотвращение переиспользования)
- ссылки инвалидируются в redis после использования или истечения ttl

### защита бота
- webhook работает только по https с проверкой secret token от telegram
- rate limiting на уровне middleware (защита от спама и перебора)
- все входные данные валидируются до обработки
- логирование без записи чувствительных данных (токены, пароли не попадают в логи)
- запуск в контейнере с минимальными привилегиями (non-root user)

---

## переменные окружения

| переменная               | описание                                      |
|--------------------------|-----------------------------------------------|
| `bot_token`              | токен telegram-бота                           |
| `webhook_secret`         | секрет для верификации webhook от telegram    |
| `database_url`           | строка подключения к postgresql               |
| `redis_url`              | строка подключения к redis                    |
| `streaming_api_url`      | адрес api стриминговой системы                |
| `streaming_api_key`      | ключ доступа к стриминговому api              |
| `link_hmac_secret`       | секрет для подписи генерируемых ссылок        |
| `link_ttl_minutes`       | время жизни ссылки в минутах                  |
| `encryption_key`         | ключ для шифрования данных в бд (aes-256)     |

---

## технологический стек

| компонент         | технология                              |
|-------------------|-----------------------------------------|
| язык              | go 1.22+                                |
| telegram api      | go-telegram-bot-api                     |
| бд                | postgresql 15+                          |
| кэш / сессии      | redis 7+                                |
| миграции          | golang-migrate                          |
| логирование       | zerolog                                 |
| конфигурация      | godotenv + os.environ                   |
| контейнеризация   | docker + docker compose                 |

---

## запуск

### предварительные требования
- docker и docker compose
- go 1.22+ (для локальной разработки)

### через docker compose

```bash
cp .env.example .env
# заполнить .env нужными значениями
docker compose up -d
```

### локально

```bash
go mod download
go run ./cmd/bot
```

---

## модель данных (основные таблицы)

### users
```sql
create table users (
    id          bigint primary key,          -- telegram user id
    username    text,
    created_at  timestamptz default now(),
    banned      boolean default false
);
```

### transactions
```sql
create table transactions (
    id              uuid primary key default gen_random_uuid(),
    user_id         bigint references users(id),
    telegram_charge_id text unique not null,
    content_id      text not null,           -- внутренний id контента (не раскрывается)
    stars_amount    integer not null,
    status          text not null,           -- pending, completed, refunded
    created_at      timestamptz default now()
);
```

### streaming_links
```sql
create table streaming_links (
    id              uuid primary key default gen_random_uuid(),
    transaction_id  uuid references transactions(id),
    user_id         bigint references users(id),
    link_hash       text not null,           -- хэш ссылки, не сама ссылка
    expires_at      timestamptz not null,
    used            boolean default false,
    created_at      timestamptz default now()
);
```

### reviews
```sql
create table reviews (
    id              uuid primary key default gen_random_uuid(),
    user_id         bigint references users(id),
    transaction_id  uuid references transactions(id),
    rating          smallint check (rating between 1 and 5),
    text            text,
    published       boolean default false,
    created_at      timestamptz default now()
);
```
