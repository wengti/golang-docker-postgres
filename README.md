# Go + Gin + Dockerized PostgreSQL — Backend API

A demo backend API built with **Go (Gin)** that talks to a **PostgreSQL** database
running in **Docker**. This README is a personal refresher covering how to run the
project and the key concepts behind it.

**Stack:** Go + Gin · PostgreSQL 17 (Docker) · pgx/pgxpool · Air (hot reload) · Docker Compose

---

## Table of contents

1. [Getting started](#1-getting-started)
2. [Project structure & architecture](#2-project-structure--architecture)
3. [Core concepts](#3-core-concepts)
   - [PostgreSQL is server-based (not file-based like SQLite)](#postgresql-is-server-based)
   - [What a connection pool is](#what-a-connection-pool-is)
   - [pgx / pgxpool](#pgx--pgxpool)
   - [TLS / `sslmode` in the connection string](#tls--sslmode)
   - [`//go:embed`](#goembed)
4. [Hot reloading with Air](#4-hot-reloading-with-air)
5. [Docker reference](#5-docker-reference)
   - [Container networking: `localhost` vs service name](#container-networking)
   - [`docker ps` vs `docker compose ps`](#docker-ps-vs-docker-compose-ps)
   - [Common commands](#common-commands)
6. [Inspecting the database with a GUI](#6-inspecting-the-database-with-a-gui)
7. [API reference](#7-api-reference)
8. [Further reading](#8-further-reading)

---

## 1. Getting started

### Prerequisites
- [Go](https://go.dev/dl/) (1.26+)
- [Docker](https://www.docker.com/) (with Compose)
- [Air](https://github.com/air-verse/air) for hot reload — `go install github.com/air-verse/air@latest`

### First-time setup
```bash
# 1. Create your local env file from the template, then edit values if needed
cp .env.example .env

# 2. Download Go dependencies
go mod download
```

### There are two ways to run this project

#### Mode 1 — DB in Docker, app on your machine with hot reload (day-to-day dev)
Run only the database as a container, and run the Go app locally via Air so code
changes reload instantly.

```bash
docker compose up -d db   # start ONLY the postgres container (detached)
air                       # run the Go app on your machine with hot reload
```
- App connects to Postgres at `localhost:5432` (`DB_HOST=localhost` in `.env`).
- Edit any `.go` file → Air rebuilds & restarts automatically.

#### Mode 2 — DB and backend both as containers (deployable build)
Build and run the whole stack with one command.

```bash
docker compose up --build   # builds the API image, starts db + api
```
- `--build` rebuilds the API image — use it after changing Go code or the Dockerfile.
- The API container connects to Postgres by the **service name `db`** (see
  [container networking](#container-networking)).
- No hot reload here — it runs the compiled binary.

> ⚠️ **One mode at a time.** Both `air` and the `api` container bind host port
> `8080`. Running them together gives `bind: address already in use`. Stop one
> before starting the other.

| | Mode 1 (dev) | Mode 2 (full) |
|---|---|---|
| Command | `docker compose up -d db` + `air` | `docker compose up --build` |
| App runs | On your machine | In a container |
| `DB_HOST` | `localhost` | `db` (overridden in compose) |
| Hot reload | ✅ | ❌ |
| Use when | Writing code | Testing the deployable build |

Stop everything: `docker compose down` (add `-v` to also delete the data volume).

---

## 2. Project structure & architecture

```
.
├── main.go                 # entry point: load env → connect DB → migrate → wire layers → serve
├── db/
│   ├── db.go               # Connect(): build DSN, open pool, ping
│   ├── migrate.go          # Migrate(): runs the embedded schema
│   └── schema.sql          # CREATE TABLE IF NOT EXISTS ... (embedded via //go:embed)
├── store/
│   ├── store.go            # Store struct wrapping the pgx pool
│   └── user.go             # User model + SQL (Create/List/Delete) + domain errors
├── handlers/
│   ├── handlers.go         # Handler struct holding the store (dependency injection)
│   ├── hello.go            # GET /
│   └── users.go            # POST/GET/DELETE /users handlers
├── router/router.go        # builds the Gin engine, registers routes
├── docker-compose.yml      # db + api services, named volume, healthcheck
├── Dockerfile              # multi-stage build for the Go binary
├── .env / .env.example     # configuration (credentials, host, port, sslmode)
└── .air.toml               # Air hot-reload config
```

### The request flow: `pool → store → handler → router`
Each layer has one job, and dependencies are passed *inward* (dependency injection):

```
HTTP request
  → router      (maps URL+method to a handler)
    → handler   (parse/validate JSON, choose HTTP status codes)
      → store   (owns ALL SQL; the only layer that touches the pool)
        → pool  (pgxpool: reusable DB connections)
          → PostgreSQL
```

Why split it this way?
- **Separation of concerns** — handlers speak HTTP; the store speaks SQL.
- **Reuse** — the same query lives in one place, callable from anywhere.
- **Testability** — the store can be swapped for a fake (via an interface) so
  handlers are unit-testable without a real database.
- **Change isolation** — swapping drivers, adding caching, or editing SQL stays
  inside `store/`; handlers don't change.

Rule of thumb: **the `store` package is the only place allowed to write SQL.**

---

## 3. Core concepts

### PostgreSQL is server-based
Coming from SQLite (a single `.db`/`.sqlite` **file** your app reads directly),
the big shift is that **PostgreSQL is a client/server database**:

```
SQLite:    your app ──► mydb.sqlite            (a file on disk)
Postgres:  your app ──network──► Postgres server process ──► data on disk
```

- It runs as a long-lived **server process** listening on a port (`5432`).
- Your app never touches files — it **connects over the network** and sends queries.
- The server stores data in an internal **data directory** you never edit by hand.
  In Docker, that directory is kept in a **named volume** (`pgdata`) so data
  survives container restarts/removal.
- A `.sql` file here is **not a database** — it's just a text script of SQL commands
  you run *against* the server (e.g. our `schema.sql`).

> Only `docker compose down -v` deletes the volume (and your data). `down` alone
> keeps it.

### What a connection pool is
Opening a brand-new Postgres connection is expensive (TCP + auth + session setup).
A **pool** opens a set of reusable, warm connections once and lets requests share them:

- Each request **borrows** a connection, runs its query, and **returns** it — the
  connection is reused, not destroyed.
- The pool grows/shrinks between a min and max size on demand.
- One connection handles one query at a time, so the pool's max size is roughly
  your DB concurrency ceiling.
- **Backpressure:** if all connections are busy, new requests **wait** instead of
  opening unlimited connections — this protects Postgres from overload.

We create the pool **once** in `main` and pass it down (never connect per request).

### pgx / pgxpool
**pgx** is the PostgreSQL driver + toolkit for Go; **pgxpool** is its connection
pool. The methods used in this project:

| Method | Use |
|---|---|
| `pgxpool.New(ctx, dsn)` | Create the pool (lazy — doesn't dial yet) |
| `pool.Ping(ctx)` | Force a real round-trip to **fail fast** at startup |
| `pool.Exec(ctx, sql, args...)` | Statements with no rows (DDL, INSERT/UPDATE/DELETE) |
| `pool.QueryRow(...).Scan(...)` | Exactly one row (e.g. `INSERT ... RETURNING`) |
| `pool.Query(...)` + `rows.Next/Scan/Err/Close` | Many rows |

Notes learned along the way:
- **Always use placeholders** (`$1`, `$2`) for values — never string-concatenate
  user input. This prevents SQL injection.
- `RETURNING` gets back DB-generated columns (`id`, `created_at`) in the same query.
- When iterating rows, **`defer rows.Close()`** (returns the connection to the pool)
  and **check `rows.Err()`** after the loop (the loop also ends on error).
- Detect specific DB errors via `errors.As` on `*pgconn.PgError` and its SQLSTATE
  `.Code` (e.g. `23505` = unique violation → map to HTTP 409).

📖 **Docs:** https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool
(Locally: `go doc github.com/jackc/pgx/v5/pgxpool` — always matches your version.)

### TLS / `sslmode`
The connection string ends with `?sslmode=...`, controlled by `DB_SSLMODE` in `.env`.
It decides whether traffic between the app and the DB is encrypted.

| Mode | Meaning | When |
|---|---|---|
| `disable` | No encryption | **Local dev** — app and DB on the same machine (loopback) |
| `require` | Encrypt, don't verify the cert | Remote DB, minimum bar |
| `verify-full` | Encrypt **and** verify the server cert | **Production** (e.g. managed RDS) |

**Rule of thumb — does the traffic leave the machine?**
- App + DB on the **same host** (localhost / same container task) → traffic stays
  on the box → `disable` is fine.
- DB is a **managed service (RDS)** or on a **separate host/container** → traffic
  crosses a network → encrypt (`require` / `verify-full`).

A private VPC limits *who can reach* the DB but doesn't encrypt the data; TLS is
defense-in-depth. Because production usually means a remote DB, assume you'll need
encryption there. Keeping `sslmode` configurable means flipping environments is a
config change, not a code change. (`verify-full` additionally needs the CA cert
available to the app.)

### `//go:embed`
`//go:embed` is a **compiler directive** (a special comment, not a normal one) that
inlines a file's contents into the binary at **build time**:

```go
import _ "embed"          // blank import enables the feature

//go:embed schema.sql     // directive — must sit directly above the var
var schema string         // gets the file contents (string, []byte, or embed.FS)
```

- Must be written exactly `//go:embed` (no space after `//`).
- The file is read **at compile time** and baked into the binary, so there's no
  file lookup at runtime → a self-contained executable (great for Docker).
- Missing file at build time = compile error (fail fast).
- We use it so `schema.sql` ships inside the binary and `db.Migrate` can run it on
  startup. The script uses `CREATE TABLE IF NOT EXISTS`, so it's **idempotent**
  (safe to run on every boot).

---

## 4. Hot reloading with Air

[Air](https://github.com/air-verse/air) watches your Go source and automatically
rebuilds + restarts the app on save.

```bash
# Install once per machine (not per project)
go install github.com/air-verse/air@latest

# Generate a config in the project root (optional — Air has built-in defaults)
air init        # creates .air.toml

# Run (replaces `go run .` for dev)
air
```

Common usage:
- `air` — run with hot reload.
- `air -d` — debug logging.
- Build artifacts go to `tmp/` and `build-errors.log` (both gitignored).

For each **new** project: `go install` is *not* repeated (it's global); just run
`air init` (or even just `air`) in the project root. If your entrypoint isn't at the
repo root, edit the `cmd` in `.air.toml` (e.g. `go build -o ./tmp/main ./cmd/server`).

---

## 5. Docker reference

### Container networking
**Where the app runs decides how it reaches Postgres:**

- App **on your machine** (Mode 1 / Air) → Postgres is at **`localhost:5432`**.
- App **inside a container** (Mode 2) → `localhost` means *the app's own container*,
  not the DB. Containers reach each other by **Compose service name** → **`db:5432`**.

That's why the `api` service overrides `DB_HOST: db` in `docker-compose.yml`. Compose
env precedence: values in `environment:` **override** values from `env_file:`, so the
same `.env` works for both modes — only `DB_HOST` is swapped for the container case.

### `docker ps` vs `docker compose ps`
| | `docker ps` | `docker compose ps` |
|---|---|---|
| Scope | **Every** container on the machine | Only **this project's** services |
| Needs a compose file? | No | Yes (run from project dir) |
| Identifies by | Container name/ID | **Service name** (`db`, `api`) |

`docker compose ps` is essentially `docker ps` filtered to the current Compose
project — use it to answer "is *my* stack up?".

### Common commands
```bash
docker compose up -d            # create + start all services (detached)
docker compose up -d db         # start only the db service (+ its deps)
docker compose up --build       # rebuild images, then start
docker compose ps               # status of this project's services
docker compose logs -f db       # follow a service's logs
docker compose stop / start     # halt / resume containers (kept around)
docker compose down             # stop AND remove containers + network (volume kept)
docker compose down -v          # ...and DELETE the data volume (wipes the DB)
```

Mental model: `stop`/`start` = light switch · `down`/`up` = demolish/rebuild the
container · only `-v` destroys the `pgdata` volume.

---

## 6. Inspecting the database with a GUI

Unlike SQLite's file-based browsers, a Postgres GUI **connects to the running server**.

**Native app — [TablePlus](https://tableplus.com/)** (DB-Browser-like). Connect with:

| Field | Value |
|---|---|
| Host | `127.0.0.1` |
| Port | `5432` |
| User | `appuser` |
| Password | `secret` |
| Database | `appdb` |

Other options: DBeaver, pgAdmin (same connection details).

**Zero-install alternative — Adminer in Docker** (web-based, no admin rights needed):
add an `adminer` service to compose, open `http://localhost:8081`, and connect with
**Server = `db`** (service name, because Adminer runs *inside* the Docker network).

> After inserting via the API, hit **refresh** in the GUI — clients cache the result
> set; the data is in Postgres immediately.

---

## 7. API reference

| Method | Path | Body | Success | Errors |
|---|---|---|---|---|
| `GET` | `/` | – | `200` hello message | – |
| `POST` | `/users` | `{"name","email"}` | `201` created user | `400` invalid body · `409` email exists |
| `GET` | `/users` | – | `200` array of users | `500` |
| `DELETE` | `/users/:id` | – | `204` no content | `400` bad id · `404` not found |

```bash
# Create
curl -i -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'

# List
curl -s http://localhost:8080/users

# Delete
curl -i -X DELETE http://localhost:8080/users/1
```

**Error-handling pattern:** the `store` catches DB-specific errors and re-expresses
them as domain sentinels (`ErrEmailExists`, `ErrUserNotFound`); the handler maps
those to HTTP status codes with `errors.Is`. Raw driver errors never leak to the
HTTP layer.

---

## 8. Further reading

- **Docker tutorial:** https://github.com/wengti/docker-tutorial
- **pgx / pgxpool docs:** https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool
- **Air (hot reload):** https://github.com/air-verse/air
- **Gin web framework:** https://gin-gonic.com/docs/
- **PostgreSQL error codes:** https://www.postgresql.org/docs/current/errcodes-appendix.html
- **PostgreSQL connection strings / `sslmode`:** https://www.postgresql.org/docs/current/libpq-connect.html
```
