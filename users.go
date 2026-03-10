package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

var token_validity_time time.Duration = time.Hour * 24 * 14

// On success returns the user_id, on failure writes out the status and error message and returns 0
func getUserId(w http.ResponseWriter, r *http.Request) uint {
	token_b64 := r.Header.Get("token")
	if token_b64 == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "No token header provided")
		return 0
	}
	token, err := base64.StdEncoding.DecodeString(token_b64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Bad encoding for token")
		return 0
	}

	row := db.QueryRow("SELECT user_id, creation_date FROM tokens WHERE token=?", token)
	var id uint
	var creation_date time.Time
	if err := row.Scan(&id, &creation_date); err != nil {
		if err != sql.ErrNoRows {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(os.Stderr, "Error while fetching password hash %v\n", err)
			fmt.Fprintf(w, "Error while fetching password hash %v", err)

		} else {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "Bad token")
		}
		return 0
	}

	if time.Since(creation_date) > token_validity_time {
		db.Exec("DELETE FROM tokens WHERE token=?", token)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "Token expired, please request a new one")
		return 0
	}
	return id
}

func register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	//These are required for registration, the rest can be added in the profile settings
	email := r.PostForm.Get("email")
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")
	firstname := r.PostForm.Get("firstname")
	lastname := r.PostForm.Get("lastname")

	if email == "" || username == "" || password == "" || firstname == "" || lastname == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s := sha256.New()
	s.Write([]byte(password))
	computed_hash := s.Sum([]byte{})
	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "Error creating transaction %v\n", err)
		fmt.Fprintf(w, "Error creating transaction %v", err)
		return
	}
	row := tx.QueryRow("INSERT INTO users (email, username, password_hash, firstname, lastname) VALUES (?, ?, ?, ?, ?) RETURNING id", email, username, computed_hash, firstname, lastname)
	var id uint
	if err := row.Scan(&id); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "Error while creating user %v\n", err)
		fmt.Fprintf(w, "Error while fetching password hash %v", err)
		return
	}

	token, err := createTokenForUser(id, tx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "Error while creating token %v\n", err)
		fmt.Fprintf(w, "Error while creating token %v", err)
		return
	}
	tx.Commit()
	// has to be string, because it will be in headers
	w.Write([]byte(base64.StdEncoding.EncodeToString(token[:])))
}

func login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	email := r.PostForm.Get("email")
	password := r.PostForm.Get("password")

	if email == "" || password == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s := sha256.New()
	s.Write([]byte(password))
	computed_hash := s.Sum([]byte{})

	row := db.QueryRow("SELECT id,password_hash FROM users WHERE email=?", email)
	var stored_hash [sha256.Size]byte
	var user_id uint
	if err := row.Scan(&user_id, &stored_hash); err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "Bad email or password")
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(os.Stderr, "Error while fetching password hash %v\n", err)
			fmt.Fprintf(w, "Error while fetching password hash %v", err)
		}
		return
	}

	if !slices.Equal(computed_hash, stored_hash[:]) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "Bad email or password")
		return
	}
	token, err := createTokenForUser(user_id, db)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "Error while creating token %v\n", err)
		fmt.Fprintf(w, "Error while creating token %v", err)
		return
	}
	// has to be string, because it will be in headers
	w.Write([]byte(base64.StdEncoding.EncodeToString(token[:])))
}

type dbQuerier interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func createTokenForUser(user_id uint, db dbQuerier) ([32]byte, error) {
	for {
		var token [32]byte
		rand.Read(token[:])
		_, err := db.Exec("INSERT INTO tokens (token, user_id) VALUES (?, ?)", token, user_id)
		if strings.Contains(err.Error(), "unique") {
			fmt.Fprintf(os.Stderr, "Casino time\n")
			continue
		}
		if err != nil {
			return [32]byte{}, err
		}

		return token, err
	}
}
