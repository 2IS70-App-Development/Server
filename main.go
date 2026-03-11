package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const contextKeyUser contextKey = "user"

var jwtSecretKey []byte

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// logsMiddleware wraps a handler and logs incoming requests
func logsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s", r.Method, r.URL.Path)
	})
}

// authMiddleware wraps a handler and verifies jwt
func authMiddleware(next http.Handler, jwtSecret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(strings.TrimPrefix(r.URL.Path, "/"), "auth/") {
			next.ServeHTTP(w, r)
			return
		}

		accessToken := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		token, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
			return []byte(jwtSecret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if _, ok := token.Claims.(jwt.MapClaims); !ok {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userId, err := token.Claims.GetSubject()
		if err != nil {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := GetUser(userId)
		if err != nil {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func main() {
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "./database.db?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL")
	schemaPath := envOr("SCHEMA_PATH", "./schema.sql")
	jwtSecret := envOr("JWT_SECRET", "change-me-in-production")

	jwtSecretKey = []byte(jwtSecret)

	err := DbStart(dbPath, schemaPath)
	if err != nil {
		log.Fatal(err)
	}
	defer Db.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /auth/users", http.HandlerFunc(GetUsersList))
	mux.Handle("GET /auth/users/details", http.HandlerFunc(GetUserDetails))

	mux.Handle("GET /auth/orders", http.HandlerFunc(GetOrdersList))
	mux.Handle("GET /auth/orders/details", http.HandlerFunc(GetOrderDetails))
	mux.Handle("POST /auth/orders", http.HandlerFunc(CreateOrderEndpoint))
	mux.Handle("PUT /auth/orders/status", http.HandlerFunc(UpdateOrderStatusEndpoint))

	mux.Handle("GET /auth/orders/scans", http.HandlerFunc(GetOrderScansEndpoint))
	mux.Handle("POST /auth/orders/scan", http.HandlerFunc(CreateOrderScanEndpoint))

	mux.HandleFunc("POST /signup", Signup)
	mux.HandleFunc("POST /jwt/create", JwtCreate)

	fmt.Printf("Server starting on port %s...\n", port)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: authMiddleware(logsMiddleware(mux), jwtSecret),
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v", err)
			os.Exit(1)
		}

	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	fmt.Printf("Shutdown signal received. Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "HTTP server shutdown error: %v", err)
	}
}
