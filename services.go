package main

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (a *App) getUsers() (*[]User, error) {
	var users []User
	rows, err := a.db.Query("SELECT id, email, created_at FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &users, nil
}

func (a *App) getUser(email string) (*User, error) {
	var user User
	err := a.db.QueryRow("SELECT id, email, created_at FROM users WHERE username = ?", email).Scan(&user.ID, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (a *App) createUser(email string, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	res, err := a.db.Exec("INSERT INTO users (email, password_hash) VALUES(?, ?)", email, string(hash))
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	createdAt := time.Now()

	return &User{
		ID:        int(id),
		Email:     email,
		CreatedAt: createdAt,
	}, nil
}
