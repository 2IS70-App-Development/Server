package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var Db *sql.DB

func DbStart(dbPath string, schemaPath string) error {
	var err error
	Db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	// If the database file already exists on disk, skip creating tables
	// This allows using a pre-populated (possibly read-only) DB from the image.
	filePath := strings.Split(dbPath, "?")[0]
	if filePath != "" {
		if _, statErr := os.Stat(filePath); statErr == nil {
			return nil
		}
	}

	if err := createTablesFromSchema(Db, schemaPath); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	return nil
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
