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

FROM alpine:3.18
RUN apk add --no-cache ca-certificates sqlite-libs

RUN addgroup -S app && adduser -S app -G app
COPY --from=builder /usr/local/bin/server /usr/local/bin/server
COPY --from=builder /src/schema.sql /app/schema.sql
COPY --from=builder /src/database.db /app/database.db
WORKDIR /app
USER app

ENV PORT=8080
EXPOSE 8080

CMD ["/usr/local/bin/server"]
