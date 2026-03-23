FROM golang:1.24-alpine AS builder

WORKDIR /src

# Download modules first (caching)
COPY go.mod go.sum ./
# install build dependencies needed for cgo + sqlite
RUN apk add --no-cache git build-base sqlite-dev && \
    go mod download

# Copy source and build with CGO enabled for go-sqlite3
COPY . .
# enable cgo for sqlite; link against system sqlite3
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /usr/local/bin/server .

# Create a pre-populated SQLite database using the schema so the final image
# contains a ready-to-use DB file. Install sqlite CLI and create the file.
RUN apk add --no-cache sqlite

FROM golang:1.24-alpine
RUN apk add --no-cache ca-certificates sqlite-dev build-base git

RUN addgroup -S app && adduser -S app -G app
WORKDIR /app

# Copy source and pre-created DB from the builder stage so `go run .` can use them.
COPY --from=builder /src /app

ENV PORT=8080
EXPOSE 8080

CMD ["go","run","."]
