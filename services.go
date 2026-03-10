package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
	err := a.db.QueryRow("SELECT id, email, password_hash, created_at FROM users WHERE email = ?", email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
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

	var newUser User
	err = a.db.QueryRow("INSERT INTO users (email, password_hash) VALUES(?, ?) RETURNING id, email, created_at", email, string(hash)).Scan(&newUser.ID, &newUser.Email, &newUser.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &newUser, nil
}

func (a *App) jwtCreateService(user *User, password string) (string, error) {
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
		Issuer:    "oath",
		Subject:   strconv.Itoa(user.ID),
	})
	ss, err := token.SignedString(a.jwtSecret)

	if err != nil {
		return "", err
	}

	return ss, nil
}
