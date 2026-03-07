package main

import (
	"fmt"
)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

func (a *App) getUsers() (*[]User, error) {
	var users []User
	rows, err := a.db.Query("SELECT id, username FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username); err != nil {
			return nil, err
		}
		fmt.Printf("id: %d username: %s\n", user.ID, user.Username)
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &users, nil
}

func (a *App) getUser(username string) (*User, error) {
	var user User
	err := a.db.QueryRow("SELECT id, username FROM users WHERE username = ?", username).Scan(&user.ID, &user.Username)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
