# App Development Server Go

Server implementation using Go for course App Development (2IS70).

## Tech Stack

- Go with `net/http`

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

| Methods | Path               | Description         |
| ------- | ------------------ | ------------------- |
| GET     | /api/health        | Check server health |
| GET     | /api/users         | Get users           |
| GET     | /api/users/details | Get user details    |

### Project Structure

```bash
├── main.go                     # HTTP server, routes, CORS + Logs middleware
├── views.go                    # User handlers
├── models.go                   # User model
├── services.go                 # User services
├── db.go                       # SQLite database connection
├── schema.sql                  # Database schema
├── .env.example                # .env example file
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```
