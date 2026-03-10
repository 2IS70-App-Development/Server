package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func main() {
	var err error
	dsn := "the.db?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL"
	db, err = sql.Open("sqlite3", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %s", err)
		os.Exit(1)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", login)
	mux.HandleFunc("POST /register", register)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	go func() {
		srv.ListenAndServe()
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
