# Task Service

A task-tracking REST API for a medical information system. Built with Go 1.23, gorilla/mux, pgx/v5, and clean architecture. Includes a recurring-task engine that materialises schedule occurrences into concrete task rows.

- [Quickstart](#quickstart)
- [Architecture](#architecture)
- [Recurrence feature](#recurrence-feature)
- [API](#api)
- [Configuration](#configuration)
- [Assumptions and edge cases](#assumptions-and-edge-cases)
- [Testing](#testing)
- [Out of scope](#out-of-scope)

---

## Quickstart

```bash
docker compose up --build
```

Service is available at `http://localhost:8080`.

If the postgres container was started before with the old schema, recreate the volume:

```bash
docker compose down -v
docker compose up --build
```

Both migration files in `migrations/` are mounted into `docker-entrypoint-initdb.d` and applied automatically on the first volume initialisation.

Swagger UI: `http://localhost:8080/swagger/`

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│  cmd/api/main.go  (wiring, graceful shutdown, env config) │
└──────────┬───────────────────────────────────────────────┘
           │
    ┌──────▼──────────────────────┐
    │  transport/http             │  gorilla/mux, JSON DTOs
    │  handlers/{task,schedule}   │
    └──────┬──────────────────────┘
           │  usecase interfaces
    ┌──────▼──────────────────────┐
    │  usecase/{task,schedule}    │  validation, business rules
    └──────┬──────────────────────┘
           │  repository interfaces
    ┌──────▼──────────────────────┐
    │  repository/postgres        │  pgx/v5, raw SQL
    └──────┬──────────────────────┘
           │
    ┌──────▼──────────────────────┐
    │  domain/{task,schedule}     │  entities, pure functions
    └─────────────────────────────┘

    ┌────────────────────────────────────────────────────────┐
    │  worker/generator  (background goroutine)              │
    │  ListActive → Occurrences → CreateFromSchedule         │
    │  fires once on startup, then every GENERATOR_INTERVAL  │
    └────────────────────────────────────────────────────────┘
```

---

## Recurrence feature

### Problem

The clinic needs tasks that repeat on a schedule: daily ward rounds, monthly medication checks, audits on specific dates, procedures on even or odd days. Creating them by hand every day is error-prone.

### Domain model

A **Schedule** is a recurrence template. It holds the `kind`, its typed `params`, a `start_date`, an optional `end_date`, and a `timezone`. The existing **Task** remains unchanged in its role — it is the materialized instance with its own `status`, `created_at`, and `updated_at`. One-off tasks created directly via the API continue to work exactly as before; `schedule_id` and `due_date` are simply `null` for them.

### Supported kinds

| `kind` | `params` shape | Description |
|---|---|---|
| `daily_every_n` | `{"n": 1}` | Every N days, anchored on `start_date` |
| `monthly_days` | `{"days": [1, 15]}` | Specific days of every month (1–30) |
| `specific_dates` | `{"dates": ["2026-04-10"]}` | Explicit calendar dates |
| `even_odd` | `{"parity": "even"}` | Even or odd days of each month |

### Materialisation strategy

The generator runs as a background goroutine with a configurable tick cadence (`GENERATOR_INTERVAL`, default `1h`). On **startup** it runs immediately to catch up the look-ahead window from today forward (no back-fill of dates before today).

**Why materialise-ahead instead of compute-on-read?**

Compute-on-read would require a separate table to store per-instance statuses (since a recurring task can be independently marked `done`), and every existing `/tasks` endpoint would need to merge two sources. Materialise-ahead keeps `Task` as the single source of truth and requires zero changes to how the client reads tasks.

### Horizon and idempotency

The generator looks ahead `GENERATOR_HORIZON_DAYS` (default 30) from today. A database `UNIQUE INDEX ON tasks (schedule_id, due_date) WHERE schedule_id IS NOT NULL` combined with `INSERT … ON CONFLICT DO NOTHING` makes every cycle idempotent. Restarting the service, expanding the horizon, or running a second generator instance cannot create duplicate tasks.

### Generator lifecycle

1. Process starts → `generator.Run(ctx)` fires immediately.
2. `ListActive` → for each schedule: `Occurrences(schedule, today, today+horizon)` → `CreateFromSchedule` per date.
3. Tick (default 1 h) → repeat step 2.
4. SIGINT/SIGTERM → context cancelled → generator finishes its current cycle → process exits (`sync.WaitGroup`).

---

## API

Base prefix: `/api/v1`

| Method | Path | Description |
|---|---|---|
| `POST` | `/tasks` | Create one-off task |
| `GET` | `/tasks` | List tasks (with optional filters) |
| `GET` | `/tasks/{id}` | Get task |
| `PUT` | `/tasks/{id}` | Update task |
| `DELETE` | `/tasks/{id}` | Delete task |
| `POST` | `/schedules` | Create schedule |
| `GET` | `/schedules` | List schedules |
| `GET` | `/schedules/{id}` | Get schedule |
| `PUT` | `/schedules/{id}` | Replace schedule |
| `DELETE` | `/schedules/{id}` | Delete schedule |
| `POST` | `/schedules/{id}/activate` | Activate schedule |
| `POST` | `/schedules/{id}/deactivate` | Deactivate schedule |

### Create schedule — one example per kind

**daily_every_n**
```bash
curl -s -X POST http://localhost:8080/api/v1/schedules \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Daily patient round",
    "description": "Morning round of all wards",
    "default_status": "new",
    "kind": "daily_every_n",
    "params": {"n": 1},
    "start_date": "2026-04-10",
    "timezone": "Europe/Moscow"
  }' | jq .
```

**monthly_days**
```bash
curl -s -X POST http://localhost:8080/api/v1/schedules \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Monthly medication audit",
    "default_status": "new",
    "kind": "monthly_days",
    "params": {"days": [1, 15]},
    "start_date": "2026-04-01",
    "timezone": "Europe/Moscow"
  }' | jq .
```

**specific_dates**
```bash
curl -s -X POST http://localhost:8080/api/v1/schedules \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Annual equipment inspection",
    "default_status": "new",
    "kind": "specific_dates",
    "params": {"dates": ["2026-05-01", "2026-09-01", "2027-01-15"]},
    "start_date": "2026-05-01",
    "timezone": "Europe/Moscow"
  }' | jq .
```

**even_odd**
```bash
curl -s -X POST http://localhost:8080/api/v1/schedules \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Physiotherapy (even days)",
    "default_status": "new",
    "kind": "even_odd",
    "params": {"parity": "even"},
    "start_date": "2026-04-10",
    "timezone": "Europe/Moscow"
  }' | jq .
```

### Filtered task list

```bash
# Tasks from schedule 1 in April 2026
curl -s "http://localhost:8080/api/v1/tasks?schedule_id=1&from=2026-04-01&to=2026-04-30" | jq .

# All new tasks
curl -s "http://localhost:8080/api/v1/tasks?status=new" | jq .
```

### Deactivate a schedule

```bash
curl -s -X POST http://localhost:8080/api/v1/schedules/1/deactivate
# 204 No Content — generator stops producing new tasks for this schedule
```

---

## Configuration

| Variable | Default | Description |
|---|---|---|
| `HTTP_ADDR` | `:8080` | HTTP listen address |
| `DATABASE_DSN` | `postgres://postgres:postgres@localhost:5432/taskservice?sslmode=disable` | PostgreSQL connection string |
| `TASKSERVICE_TIMEZONE` | `Europe/Moscow` | IANA timezone for computing day boundaries in the generator |
| `GENERATOR_HORIZON_DAYS` | `30` | How many days ahead to materialise tasks |
| `GENERATOR_INTERVAL` | `1h` | Generator tick cadence (`time.ParseDuration` format) |
| `GENERATOR_DISABLED` | `false` | Set to `true` to disable the generator (useful for local debugging) |

---

## Assumptions and edge cases

1. **Schedule deletion** leaves existing materialised tasks intact with `schedule_id = NULL`. Audit history outweighs relational cleanliness; a cascade delete would silently remove completed tasks from history.

2. **Schedule update** does not rewrite already-materialised tasks. The generator adds only new dates inside the current horizon. If a change shrinks the schedule, future tasks that were already created remain; delete them manually if needed.

3. **Deactivate** stops the generator from creating new tasks for that schedule. Already-materialised future tasks remain and keep their statuses. Delete them manually if the schedule will never resume.

4. **`monthly_days` with 31** is rejected by the validator (spec says 1..30). Day 31 is not a universal guarantee even in the standard calendar, making it a source of confusion.

5. **`monthly_days` with 29/30** silently skips months that do not have those days (February in non-leap years). There is no snap-to-last-day behaviour; the spec does not ask for it.

6. **`specific_dates` in the past** are not rejected on creation — a schedule can be registered after some of its dates have already passed. The generator's horizon starts from today, so past dates are simply never materialised.

7. **Expanding the horizon** (e.g. 30 → 60 days) causes the next generator tick to fill in the additional 30 days. Shrinking it leaves already-created tasks untouched.

8. **Single clinic timezone** — `Europe/Moscow` is the default. A per-schedule override is supported via the `timezone` field (any valid IANA identifier).

9. **DST** — Moscow has been fixed at UTC+3 since 2014, so DST is not a practical concern here. The code uses `*time.Location` throughout, so DST-aware zones (e.g. `America/New_York`) work correctly if needed.

10. **At-least-once delivery + DB UNIQUE** — the generator may call `CreateFromSchedule` for the same `(schedule_id, due_date)` pair more than once across restarts; the database constraint makes it a no-op. Zero duplicates are guaranteed.

11. **Catch-up on startup** materialises from today forward, not a back-fill of missed past dates. A task that should have been created last week while the service was down is treated as intentionally absent, not as lost work.

12. **Narrowing `end_date` after creation** stops the generator from adding dates beyond the new bound; tasks already created past the old bound are not deleted.

13. **Horizontal scaling** is explicitly out of scope (see below). The DB UNIQUE constraint prevents duplicates from concurrent generators, but there is no leader election.

14. **Time of day** — `due_date` is a calendar date (`DATE` in PostgreSQL). If the spec grows to require "daily at 09:00", a `time_of_day` column can be added to the schedule without changing `due_date`.

15. **`daily_every_n` anchor** — the sequence is always computed relative to `start_date`. "Every 3 days from April 10" always yields April 10, 13, 16, … regardless of when the generator runs or when the query window starts.

---

## Testing

```bash
make test-race
# or: go test -race ./...
```

Coverage by package:

| Package | Tests | Notes |
|---|---|---|
| `domain/schedule` | 28 table-driven | Occurrences logic, all four kinds, TZ boundary, validation |
| `usecase/schedule` | 15 table-driven | CRUD, activate/deactivate, error mapping, fake repo |
| `worker/generator` | 4 parallel | Materialisation, idempotency, inactive-skip, graceful shutdown |

Not covered (explicit out-of-scope): postgres integration tests (no testcontainers), HTTP handler integration tests (no httptest suite beyond what the compiler verifies).

---

## Out of scope

| Topic | Reason |
|---|---|
| RRULE / iCal parsing | The spec defines exactly 4 kinds; a full RFC 5545 engine would be scope creep |
| Message queues / Redis | The generator is ~120 lines with `time.Ticker`; a broker adds operational complexity without benefit |
| Auth, users, roles | Not in the spec |
| Prometheus / OpenTelemetry | `log/slog` structured logging is sufficient for the scope |
| testcontainers integration tests | Slow for CI; the boundary is tested at the usecase layer with fakes |
| golang-migrate / custom migration runner | The existing `docker-entrypoint-initdb.d` approach already works; replacing it is a separate concern |
| Generics in repositories | Not idiomatic for this codebase style |
| Preview-next-N endpoint | Not requested |
| Eager materialisation to `end_date` | Could generate thousands of rows for long-running schedules; the rolling horizon keeps it bounded |
| Per-user timezone | Clinic operates in a single timezone; per-user override is a future concern |

---

## Future work

- Per-user timezone overrides when the clinic goes multi-site.
- `time_of_day` on schedule for "fire at 09:00" semantics.
- Webhook / event on task materialisation for downstream integrations.
- Leader election (e.g. via advisory lock) for safe horizontal scaling of the generator.
