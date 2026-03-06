package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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

func main() {
	port := envOr("PORT", "8080")
	allowedOrigin := envOr("ALLOWED_ORIGIN", "http://localhost:3000")
	dbPath := envOr("DB_PATH", "./database.db")
	schemaPath := envOr("SCHEMA_PATH", "./schema.sql")

	_, err := NewApp(dbPath, schemaPath)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	fmt.Printf("Server starting on port %s...\n", port)

	handlers := logsMiddleware(corsMiddleware(allowedOrigin, mux))

	if err := http.ListenAndServe(":"+port, handlers); err != nil {
		log.Fatal(err)
	}
}
