package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

type App struct {
	db        *sql.DB
	jwtSecret []byte
}

func NewApp(dbPath string, schemaPath string, jwtSecret string) (*App, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := createTablesFromSchema(db, schemaPath); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return &App{db: db, jwtSecret: []byte(jwtSecret)}, nil
}

func createTablesFromSchema(db *sql.DB, schemaPath string) error {
	byteSchema, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	schema := string(byteSchema)

	if _, err := db.Exec(schema); err != nil {
		return err
	}

	return nil
}
