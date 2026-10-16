# Smart Cart API

Бэкенд корзины покупателя с
динамическим расчётом промо-условий.

## Возможности

* **Управление составом корзины:** добавление товара, изменение количества, удаление,
  выбор чекбоксами (`ToggleSelection`), применение промокода.
* **Гибридное хранение (Write-Through / Cache-Aside):** активные корзины кэшируются в **Redis / Valkey**
  (`cart:user:{user_id}`, TTL `24h`) для мгновенного отклика, а
  долгосрочное состояние синхронизируется с **PostgreSQL**.
* **Параллельное обогащение (`errgroup`):** актуальные цены, остатки и промо-данные
  запрашиваются параллельно с ограничением по времени (`context.WithTimeout`).
* **Graceful Degradation:** при недоступности Redis сервис логирует `WARN` через `slog`
  и прозрачно читает данные из PostgreSQL; падение источника аналогов не роняет корзину.
* **Rule Engine (Chain of Responsibility / Strategy):** пайплайн продуктовых правил —
  доступность → снижение цены → промокод → бесплатная доставка.
* **BDUI Builder:** трансформация рассчитанной корзины в дерево виджетов с экшенами.

## Технологический стек

| Слой           | Технология                                 |
|----------------|--------------------------------------------|
| Язык           | Go 1.26+                                   |
| HTTP           | `go-chi/chi/v5`                            |
| Кэш            | Redis 7 / Valkey (`redis/go-redis/v9`)     |
| БД             | PostgreSQL 16 (`jackc/pgx/v5` + `pgxpool`) |
| Миграции       | `pressly/goose/v3`                         |
| Деньги         | `int64` в копейках (никаких `float64`)     |
| Конкурентность | `golang.org/x/sync/errgroup`               |
| Логирование    | `log/slog` (JSON + Request ID)             |
| Тесты          | `stretchr/testify`, `testcontainers-go`    |

## Архитектура

```mermaid
flowchart TD
    Client[Web / Mobile] -->|REST / BDUI| Router

    subgraph Controller [internal/controller/http]
        Router[router.go<br/>Middleware: RequestID, Logging, Recover, Timeout]
        Router --> Handlers[handlers.go]
        Handlers --> DTO[dto.go]
    end

    subgraph UseCase [internal/usecase]
        CartUC[cart_usecase.go<br/>errgroup + context.WithTimeout]
        CartUC --> Engine[rules/engine.go<br/>Chain of Responsibility]
        CartUC --> Builder[bdui/builder.go]
        Engine --> Scarcity[scarcity_rule.go]
        Engine --> PriceDrop[price_drop_rule.go]
        Engine --> Promo[promo_rule.go]
        Engine --> Delivery[delivery_rule.go]
    end

    subgraph Storage [internal/repository]
        PGCart[postgres/CartRepository]
        PGCatalog[postgres/CatalogRepository]
        PGPromo[postgres/PromoRepository]
        Redis[redis/CartCache]
    end

    Handlers --> CartUC
    CartUC -->|Cache - Aside + Fallback| Redis
    CartUC --> PGCart
    CartUC --> PGCatalog
    CartUC --> PGPromo
    Redis -.->|WARN + fallback| PGCart
    PGCart --> PG[(PostgreSQL 16)]
    PGCatalog --> PG
    PGPromo --> PG
    Redis --- R[(Redis / Valkey)]
```

## Структура проекта

```text
.
├── cmd/cart-service/main.go          # Точка входа, DI, миграции, Graceful Shutdown
├── internal/
│   ├── config/config.go              # Конфигурация из переменных окружения
│   ├── domain/                       # Cart, CartItem, Money, Product, PromoRule, BDUI-модели
│   ├── usecase/
│   │   ├── cart_usecase.go           # Оркестрация + errgroup
│   │   ├── rules/                    # Движок правил (Chain of Responsibility)
│   │   └── bdui/builder.go           # Трансформация корзины в BDUI JSON
│   ├── repository/
│   │   ├── postgres/                 # Репозитории на pgx/v5
│   │   └── redis/                    # Кэш корзины на go-redis/v9
│   └── controller/http/              # router.go, handlers.go, dto.go
├── migrations/                       # SQL-миграции goose + seed-данные
├── deploy/docker-compose.yml         # PostgreSQL + Valkey + сервис
├── Dockerfile
└── Makefile
```

## Быстрый старт

### Через Docker Compose

```bash
make docker-up          # поднимет PostgreSQL, Valkey и сервис
# сервис доступен на http://localhost:8080, миграции накатываются при старте
```

### Локально

```bash
# 1. Поднять только инфраструктуру
docker compose -f deploy/docker-compose.yml up -d postgres valkey

# 2. Прогнать миграции
make migrate-up

# 3. Запустить сервис
make run
```

## Конфигурация

| Переменная           | По умолчанию                                                  | Описание                                            |
|----------------------|---------------------------------------------------------------|-----------------------------------------------------|
| `HTTP_ADDR`          | `:8080`                                                       | Адрес HTTP-сервера                                  |
| `POSTGRES_DSN`       | `postgres://cart:cart@localhost:5432/cart_db?sslmode=disable` | DSN PostgreSQL                                      |
| `REDIS_ADDR`         | `localhost:6379`                                              | Адрес Redis/Valkey                                  |
| `REDIS_PASSWORD`     | `""`                                                          | Пароль Redis                                        |
| `REDIS_DB`           | `0`                                                           | Номер БД Redis                                      |
| `CART_CACHE_TTL`     | `24h`                                                         | TTL кэша корзины                                    |
| `ENRICHMENT_TIMEOUT` | `2s`                                                          | Таймаут параллельного обогащения                    |
| `SHUTDOWN_TIMEOUT`   | `10s`                                                         | Таймаут Graceful Shutdown                           |
| `LOG_LEVEL`          | `info`                                                        | Уровень логирования (`debug`/`info`/`warn`/`error`) |

## REST API

Все запросы требуют заголовок `X-User-Id` (положительное число).

| Метод    | Путь                                 | Описание                                                  |
|----------|--------------------------------------|-----------------------------------------------------------|
| `GET`    | `/api/v1/cart`                       | Обогащённая и рассчитанная корзина (REST JSON)            |
| `GET`    | `/api/v1/cart/bdui`                  | BDUI-экран с деревом виджетов                             |
| `POST`   | `/api/v1/cart/items`                 | Добавить товар (`{ "sku_id": 101, "quantity": 1 }`)       |
| `PATCH`  | `/api/v1/cart/items/{sku_id}`        | Изменить количество (`{ "quantity": 2 }`, `0` — удаление) |
| `DELETE` | `/api/v1/cart/items/{sku_id}`        | Удалить товар                                             |
| `POST`   | `/api/v1/cart/items/{sku_id}/toggle` | Переключить выбор чекбоксом                               |
| `POST`   | `/api/v1/cart/promo`                 | Применить промокод (`{ "code": "OZON10" }`)               |
| `GET`    | `/healthz`                           | Проверка живости                                          |

### Коды ошибок

| Ошибка                          | HTTP  | Код                            |
|---------------------------------|-------|--------------------------------|
| Позиция не найдена              | `404` | `item_not_found`               |
| Товар не найден                 | `404` | `product_not_found`            |
| Промокод не найден              | `404` | `promo_code_not_found`         |
| Промокод неактивен              | `409` | `promo_code_inactive`          |
| Не достигнута минимальная сумма | `422` | `promo_min_amount_not_reached` |
| Некорректное количество         | `400` | `invalid_quantity`             |

## Демонстрационные сценарии

В seed-данных есть товары разных категорий с остатками `0`, `2`, `3`, `5`, `50` шт.
и промокоды `OZON10` (10%, от 2 000 ₽), `SAVE15` (15%, от 5 000 ₽),
`WELCOME5` (5%, без порога), `OLD20` (неактивен).

### 1. Добавить товар и получить BDUI

```bash
# Товар с ограниченным остатком (3 шт.) и низкой ценой
curl -s -X POST http://localhost:8080/api/v1/cart/items \
  -H "X-User-Id: 42" -H "Content-Type: application/json" \
  -d '{"sku_id": 101, "quantity": 1}'

# BDUI-ответ: прогресс-бар доставки, карточка товара с бейджем дефицита, итоговый футер
curl -s http://localhost:8080/api/v1/cart/bdui -H "X-User-Id: 42" | jq
```

### 2. Снижение цены

Цена фиксируется в `price_at_addition` в момент добавления. Чтобы
воспроизвести бейдж «Цена снизилась на N ₽», установите цену добавления выше текущей:

```bash
docker compose -f deploy/docker-compose.yml exec postgres \
  psql -U cart -d cart_db -c \
  "UPDATE cart_items SET price_at_addition = 135000 WHERE user_id = 42 AND sku_id = 101;"

curl -s http://localhost:8080/api/v1/cart/bdui -H "X-User-Id: 42" | jq \
  '.widgets[] | select(.id == "item_sku_101") | .payload.badges'
# → [{ "text": "Цена снизилась на 200 ₽", "color": "green" }, ...]
```

### 3. Подбор аналогов для закончившегося товара

```bash
# sku 205 — мышь с остатком 0; сервис подберёт аналоги из той же категории
curl -s -X POST http://localhost:8080/api/v1/cart/items \
  -H "X-User-Id: 42" -H "Content-Type: application/json" \
  -d '{"sku_id": 205, "quantity": 1}'

curl -s http://localhost:8080/api/v1/cart/bdui -H "X-User-Id: 42" | jq \
  '.widgets[] | select(.type == "out_of_stock_group")'
```

### 4. Прогресс-бар бесплатной доставки

Порог бесплатной доставки — 1 500 ₽. Добавьте товаров меньше порога, чтобы увидеть
дельту в `progress_bar`:

```bash
curl -s -X POST http://localhost:8080/api/v1/cart/items \
  -H "X-User-Id: 7" -H "Content-Type: application/json" \
  -d '{"sku_id": 206, "quantity": 1}'   # Razer Pro Click, 890 ₽

curl -s http://localhost:8080/api/v1/cart/bdui -H "X-User-Id: 7" | jq \
  '.widgets[] | select(.type == "progress_bar")'
```

### 5. Промокод

```bash
curl -s -X POST http://localhost:8080/api/v1/cart/promo \
  -H "X-User-Id: 42" -H "Content-Type: application/json" \
  -d '{"code": "WELCOME5"}'

curl -s http://localhost:8080/api/v1/cart -H "X-User-Id: 42" | jq '.promo, .totals'
```

### 6. Отказоустойчивость кэша

Остановите Valkey — сервис продолжит работать, читая данные из PostgreSQL и логируя `WARN`:

```bash
docker compose -f deploy/docker-compose.yml stop valkey
curl -s http://localhost:8080/api/v1/cart -H "X-User-Id: 42" -o /dev/null -w "%{http_code}\n"  # 200
docker compose -f deploy/docker-compose.yml start valkey
```

## Правила расчёта

1. **Доступность (`scarcity_rule.go`)** — при `stock == 0` (или отсутствии товара в
   каталоге) позиция переносится в секцию «Недоступны для заказа»; при `0 < stock <= 5`
   добавляется бейдж «Осталось N шт.»; при `quantity > stock` количество для расчёта
   ограничивается остатком и показывается бейдж «Доступно только N шт.».
2. **Снижение цены (`price_drop_rule.go`)** — если текущая цена ниже цены добавления,
   добавляется бейдж «Цена снизилась на X ₽».
3. **Промокод (`promo_rule.go`)** — проверяются существование, активность и минимальная
   сумма заказа; скидка считается от суммы выбранных доступных позиций.
4. **Бесплатная доставка (`delivery_rule.go`)** — порог 1 500 ₽ от итоговой суммы;
   рассчитываются остаток и процент заполнения прогресс-бара.

## Тестирование

```bash
make test          # все тесты, включая интеграционные (нужен Docker)
make test-unit     # только unit-тесты (go test -short)
```

* Табличные unit-тесты для каждого правила (`internal/usecase/rules/`) — граничные
  случаи: пустая корзина, все товары не выбраны, ровно порог доставки, закончившийся товар.
* Unit-тесты `bdui/builder.go`, в т.ч. совпадение с примером из ТЗ (`require.JSONEq`).
* Интеграционный тест репозитория корзины на `testcontainers-go` (PostgreSQL + Redis),
  включая сценарий Fallback при отказе Redis.
