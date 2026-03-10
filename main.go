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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// corsMiddleware wraps a handler and applies CORS headers to every response,
// including preflight OPTIONS requests.
func corsMiddleware(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
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
		accessToken := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		log.Printf("%s", accessToken)

		token, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
			return []byte(jwtSecret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

		if err != nil {
			log.Printf("%v", err.Error())

			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if _, ok := token.Claims.(jwt.MapClaims); ok {
			next.ServeHTTP(w, r)
		} else {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
		}
	})
}

func main() {
	port := envOr("PORT", "8080")
	allowedOrigin := envOr("ALLOWED_ORIGIN", "http://localhost:3000")
	dbPath := envOr("DB_PATH", "./database.db")
	schemaPath := envOr("SCHEMA_PATH", "./schema.sql")
	jwtSecret := envOr("JWT_SECRET", "change-me-in-production")

	app, err := NewApp(dbPath, schemaPath, jwtSecret)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/api/users", authMiddleware(http.HandlerFunc(app.getUsersList), jwtSecret))
	mux.Handle("/api/users/details", authMiddleware(http.HandlerFunc(app.getUserDetails), jwtSecret))
	mux.HandleFunc("/api/signup", app.signup)
	mux.HandleFunc("/api/jwt/create", app.jwtCreate)

	fmt.Printf("Server starting on port %s...\n", port)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: logsMiddleware(corsMiddleware(allowedOrigin, mux)),
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
