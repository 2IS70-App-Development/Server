# App Development Server Go

Server implementation using Go for course App Development (2IS70).

## Tech Stack

- Go with `net/http`
- SQLite via `mattn/go-sqlite3`
- JWT via `golang-jwt/jwt/v5`

## Getting Started

### Prerequisites

For local development:

- [Go](https://go.dev/dl/) 1.24+

### Run locally

```
go run .
```

The dev server by default starts on port `8080`.

### API Endpoints

| Methods | Path               | Description                                    |
| ------- | ------------------ | ---------------------------------------------- |
| GET     | /api/health        | Check server health                            |
| GET     | /api/users         | Get users                                      |
| GET     | /api/users/details | Get user details                               |
| POST    | /api/signup        | Signup via email                               |
| POST    | /api/jwt/create    | Login via email and password receive JWT token |

### Project Structure

```bash
├── main.go                     # HTTP server, routes, CORS + Logs + Auth middleware
├── views.go                    # User, JWT, handlers
├── models.go                   # User model
├── services.go                 # User, JWT services
├── db.go                       # SQLite database connection
├── schema.sql                  # Database schema
├── .env.example                # .env example file
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```
