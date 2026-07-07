# Inventory / Ordering — учебный прототип микросервисов на Go

MVP из двух микросервисов на gRPC (без брокера сообщений, без Saga — это будет добавлено в дипломе).

- **Inventory** (gRPC `:50052`) — остатки товара, резервирование с optimistic locking.
- **Ordering** (gRPC `:50051`, HTTP `:8080`) — заказы, синхронно вызывает Inventory.Reserve.

## Архитектура

Каждый сервис — модульный монолит в духе DDD:

```
<service>/
  domain/       — агрегат + доменные сервисы (порты вроде StockRepository определены здесь)
  repository/   — Postgres-реализация портов (pgx)
  grpc/         — сгенерированный код (в /proto) + обработчики / клиенты
  httpapi/      — только Ordering: REST-обёртка над тем же domain.OrderService
  cmd/server/   — точка входа, конфигурация из env, graceful shutdown
  migrations/   — golang-migrate SQL
proto/          — общий .proto + сгенерированный Go-код, свой go.mod,
                  оба сервиса подключают его через `replace` (монорепо, без публикации)
loadtest/       — нагрузочный клиент (см. ниже)
```

## Запуск

```bash
docker-compose up --build
```

Поднимаются `postgres-inventory`, `postgres-ordering`, одноразовые `*-migrate` контейнеры (накатывают миграции и завершаются до старта сервисов — через `depends_on: condition: service_completed_successfully`), затем `inventory` и `ordering`.

После старта доступны:
- Inventory gRPC: `localhost:50052`
- Ordering gRPC: `localhost:50051`
- Ordering HTTP: `localhost:8080`
- Postgres inventory_db: `localhost:5433`, ordering_db: `localhost:5434` (для отладки через psql)

В Inventory миграция сразу сеет тестовые данные: `SKU-001` (остаток 100), `SKU-002` (остаток 10), `SKU-003` (остаток 0) — чтобы вызовы ниже работали сразу после старта.

## Примеры вызовов

### grpcurl → Inventory

Оба сервиса регистрируют server reflection, так что `.proto`-файл grpcurl не нужен.

```bash
grpcurl -plaintext -d '{"sku": "SKU-002", "quantity": 3}' \
  localhost:50052 inventory.v1.InventoryService/Reserve

grpcurl -plaintext -d '{"sku": "SKU-002", "quantity": 3}' \
  localhost:50052 inventory.v1.InventoryService/Release
```

### grpcurl → Ordering (второй адаптер поверх того же domain.OrderService)

```bash
grpcurl -plaintext -d '{"sku": "SKU-001", "quantity": 5}' \
  localhost:50051 ordering.v1.OrderingService/CreateOrder
```

### curl → Ordering HTTP (сценарий из ТЗ)

```bash
curl -X POST localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -d '{"sku": "SKU-001", "quantity": 5}'
```

Успешный ответ:

```json
{
  "id": "5b1e...",
  "sku": "SKU-001",
  "quantity": 5,
  "status": "reserved",
  "created_at": "2026-07-07T00:00:00Z",
  "inventory_attempts": 1
}
```

Отказ (например, `SKU-003` с нулевым остатком):

```json
{
  "id": "b2f0...",
  "sku": "SKU-003",
  "quantity": 1,
  "status": "failed",
  "failure_reason": "insufficient_stock",
  "created_at": "2026-07-07T00:00:00Z",
  "inventory_attempts": 1
}
```

## Тесты

```bash
cd inventory && go test ./... -v   # unit + concurrency-тест StockItem/ReservationService
cd ordering  && go test ./... -v   # unit-тесты Order/OrderService
```

Concurrency-тест (`inventory/domain/concurrency_test.go`) поднимает 50 горутин против одного SKU с остатком 10 (потокобезопасный in-memory репозиторий с настоящим CAS, не мок с заранее заданным сценарием) и проверяет: `reserved_quantity == количество успешных резерваций <= total_quantity`.

> На машине, где это разрабатывалось, не было `gcc`, поэтому `-race` недоступен (`CGO_ENABLED` требуется для race detector). Сами тесты проходят и без него; на машине с gcc рекомендуется гонять с `-race`.

## Нагрузочный тест

```bash
cd loadtest
go run . -url http://localhost:8080/orders -sku SKU-002 -clients 50 -quantity 1
```

Флаги: `-url`, `-sku`, `-quantity`, `-clients`, `-timeout`. Выводит человекочитаемый отчёт и CSV-строку (для вставки в отчёт по практике).

### Реальный прогон (фактический вывод)

Docker на машине, где выполнялась разработка, не установлен (см. раздел «Ограничения окружения» ниже), поэтому нагрузочный тест был прогнан не через `docker-compose`, а напрямую против собранных бинарников `inventory`/`ordering`, работающих с временным (embedded, без установки в систему) PostgreSQL — то есть **тот же код**, реальный HTTP/gRPC трафик, реальная БД, просто без Docker-обвязки. Ниже — фактический вывод трёх прогонов.

**Прогон 1 — остаток 10 (`SKU-002`), 50 параллельных клиентов по 1 шт. (пример из ТЗ):**

```
=== Load test report ===
SKU:                        SKU-002
Concurrent clients:         50
Successful (reserved):      10
Failed (business reject):   40
Errors (transport/HTTP):    0
Avg Inventory attempts/success: 2.00
Total wall-clock time:      389.9864ms
Failure reasons:
  insufficient_stock             40

=== CSV ===
sku,clients,reserved,failed,errors,avg_attempts_per_success,total_duration_ms
SKU-002,50,10,40,0,2.00,389
```

Инвариант соблюдён: успешных резерваций ровно 10 (= остаток), `reserved_quantity` не превысил `total_quantity`.

**Прогон 2 — остаток 100 (`SKU-001`), 200 параллельных клиентов по 1 шт. (больше конкуренции за версии):**

```
=== Load test report ===
SKU:                        SKU-001
Concurrent clients:         200
Successful (reserved):      100
Failed (business reject):   100
Errors (transport/HTTP):    0
Avg Inventory attempts/success: 3.74
Total wall-clock time:      138.572ms
Failure reasons:
  insufficient_stock             3
  version_conflict_exhausted     97

=== CSV ===
sku,clients,reserved,failed,errors,avg_attempts_per_success,total_duration_ms
SKU-001,200,100,100,0,3.74,138
```

Здесь хорошо видна цена жёсткого лимита в 5 попыток под сильной конкуренцией: 97 из 200 запросов исчерпали ретраи (`version_conflict_exhausted`), хотя товара физически хватало бы, если бы им повезло с порядком выполнения. Это ожидаемый и показательный эффект optimistic locking с ограниченным числом попыток — хороший материал для главы с анализом результатов.

**Прогон 3 — остаток 0 (`SKU-003`), 20 параллельных клиентов (контроль):**

```
=== Load test report ===
SKU:                        SKU-003
Concurrent clients:         20
Successful (reserved):      0
Failed (business reject):   20
Errors (transport/HTTP):    0
Avg Inventory attempts/success: 0.00
Total wall-clock time:      13.101ms
Failure reasons:
  insufficient_stock             20

=== CSV ===
sku,clients,reserved,failed,errors,avg_attempts_per_success,total_duration_ms
SKU-003,20,0,20,0,0.00,13
```

**Пример логов ретраев** (структурированные, `log/slog`, из Прогона 1):

```json
{"time":"2026-07-07T03:16:00.9145122+03:00","level":"WARN","msg":"optimistic lock conflict, retrying","op":"reserve","sku":"SKU-002","quantity":1,"attempt":1,"max_attempts":5}
```

## Принятые архитектурные решения и допущения

- **Интерфейсы репозиториев — в `domain/`, а не в `repository/`.** Классический DDD dependency inversion: домен владеет портом (`StockRepository`, `OrderRepository`), инфраструктура (`repository/postgres.go`) его реализует. Так домен не зависит от Postgres/pgx.
- **Ordering получил свой `.proto` (`ordering.proto`, метод `CreateOrder`).** В ТЗ явно описан только контракт Inventory, но при этом зарезервирован gRPC-порт 50051 для Ordering. Чтобы не оставлять порт пустым и показать архитектурную симметрию (HTTP и gRPC — два адаптера над одним `domain.OrderService`), добавлен минимальный gRPC-метод, зеркалирующий HTTP-сценарий.
- **`inventory_attempts` в HTTP-ответе Ordering — не часть персистентной модели Order.** Агрегат `Order` в БД хранит ровно то, что описано в ТЗ (id/sku/quantity/status/created_at). Дополнительное поле в JSON-ответе — это число попыток CAS, которое сделал Inventory при резервировании; оно нужно исключительно для метрики "среднее количество retry на успешный запрос" из нагрузочного теста и прокидывается по цепочке `ReserveResponse.attempts → OrderService.PlaceOrder → HTTP DTO`, не затрагивая схему БД.
- **Транспортная ошибка при вызове Inventory (например, сервис недоступен) не отдаёт HTTP 500.** Заказ помечается `failed` с причиной `inventory_unavailable`. Так клиент всегда получает предсказуемый JSON, а не иногда 500/иногда 200 — это осознанный компромисс для MVP без Saga/ретраев на уровне Ordering (в дипломе здесь появится компенсация через Release/Saga).
- **Миграции — через одноразовые `migrate/migrate` контейнеры**, а не через код сервиса при старте. Это явно демонстрирует требуемый порядок запуска (`depends_on: condition: service_completed_successfully`) и не заставляет само приложение знать про CLI-инструмент миграций.
- **Proto-код генерируется через `buf`**, а не `protoc` напрямую — `buf` написан на Go и ставится через `go install`, не требует системного бинаря `protoc`. Сгенерированный код лежит в общем модуле `proto/` (свой `go.mod`), оба сервиса подключают его через `replace` в своих `go.mod` — типичный подход для монорепозитория без публикации приватного модуля.
- **gRPC reflection + стандартный `grpc_health_v1` включены в обоих сервисах** — чтобы grpcurl работал без файла `.proto`, а docker-compose healthcheck и будущий API-gateway могли использовать стандартный health-check.
- **Ретраи при конфликте версии:** до 5 попыток, между попытками — `attempt * 5ms` + случайный джиттер `0–9ms`. Каждая попытка логируется через `log/slog` (уровень WARN), финальный отказ — уровень ERROR.
- **Порты Postgres наружу** (`5433` для inventory_db, `5434` для ordering_db) — для удобной отладки через `psql`/DBeaver с хоста, не только изнутри docker-сети.

## Ограничения окружения разработки

Машина, на которой выполнялась разработка, не имеет установленных Docker, `protoc`/`buf` и локального PostgreSQL. Это не повлияло на сам поставляемый код:

- Кодогенерация из `.proto` выполнена через `buf` (поставлен через `go install`, не требует системного `protoc`).
- Реальный прогон (юнит-тесты, concurrency-тест, нагрузочный тест) выполнен напрямую против собранных бинарников сервисов и временного embedded-PostgreSQL (без установки в систему, без Docker) — то есть тот же код, реальные вызовы, реальная БД, просто не через `docker-compose`.
- `docker-compose.yml` не тестировался запуском `docker-compose up` в этой среде — рекомендуется проверить его на машине с установленным Docker перед защитой практики (структура, healthcheck'и и порядок `depends_on` соответствуют документированному поведению golang-migrate/postgres/grpc-health, но реальный `docker-compose up` стоит прогнать хотя бы раз).
