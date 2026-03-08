package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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
			log.Printf(err.Error())

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

	handlers := logsMiddleware(corsMiddleware(allowedOrigin, mux))

	if err := http.ListenAndServe(":"+port, handlers); err != nil {
		log.Fatal(err)
	}
}
