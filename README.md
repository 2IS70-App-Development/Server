# App Development Server Go

Server implementation using Go for course App Development (2IS70).

## Tech Stack

- Go with `net/http`
- SQLite via `mattn/go-sqlite3`
- JWT via `golang-jwt/jwt/v5`
- Password hashing via `golang.org/x/crypto/bcrypt`

## Getting Started

### Prerequisites

For local development:

- [Go](https://go.dev/dl/) 1.24+

### Environment Variables

| Variable      | Default                                                      | Description            |
| ------------- | ------------------------------------------------------------ | ---------------------- |
| `PORT`        | `8080`                                                       | Server port            |
| `DB_PATH`     | `./database.db?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL` | SQLite database path   |
| `SCHEMA_PATH` | `./schema.sql`                                               | Path to SQL schema     |
| `JWT_SECRET`  | `change-me-in-production`                                    | Secret key for signing JWTs |

### Run locally

```
go run .
```

The dev server by default starts on port `8080`.

### API Endpoints

Routes under `/auth/` require a valid JWT token in the `Authorization: Bearer <token>` header.

#### Public

| Method | Path         | Description                              |
| ------ | ------------ | ---------------------------------------- |
| GET    | /api/health  | Health check                             |
| POST   | /signup      | Register a new user (email + password)   |
| POST   | /jwt/create  | Login and receive a JWT access token     |

#### Authenticated (`/auth/`)

| Method | Path                | Description                              |
| ------ | ------------------- | ---------------------------------------- |
| GET    | /auth/users         | List all users                           |
| GET    | /auth/users/details | Get user details (query: `id`)           |
| GET    | /auth/orders        | List orders for the authenticated user   |
| GET    | /auth/orders/details| Get order details (query: `id`)          |
| POST   | /auth/orders        | Create a new order                       |
| PUT    | /auth/orders/status | Update order status (`pending`, `in-transit`, `delivered`, `cancelled`) |
| GET    | /auth/orders/scans  | List scans for an order (query: `order_id`) |
| POST   | /auth/orders/scan   | Create a scan for an order               |

### Data Models

#### User

| Field        | Type     |
| ------------ | -------- |
| id           | INTEGER  |
| email        | TEXT     |
| password_hash| TEXT     |
| created_at   | DATETIME |

#### Order

| Field       | Type     |
| ----------- | -------- |
| id          | INTEGER  |
| sender_id   | INTEGER  |
| receiver_id | INTEGER  |
| name        | VARCHAR  |
| status      | VARCHAR  |
| meta        | TEXT     |
| comment     | TEXT     |
| created_at  | DATETIME |

#### Scan

| Field     | Type     |
| --------- | -------- |
| id        | INTEGER  |
| order_id  | INTEGER  |
| courier_id| INTEGER  |
| photo     | BLOB     |
| condition | VARCHAR  |
| longitude | DECIMAL  |
| latitude  | DECIMAL  |
| comment   | TEXT     |
| created_at| DATETIME |

### Project Structure

```bash
├── main.go                     # HTTP server, routes, logging + auth middleware
├── views.go                    # Request handlers (users, orders, scans, auth)
├── models.go                   # Data models (User, Order, Scan)
├── services.go                 # Business logic & database queries
├── db.go                       # SQLite database connection & schema init
├── schema.sql                  # Database schema (users, orders, scans)
├── go.mod
├── go.sum
└── README.md
```
