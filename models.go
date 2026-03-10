package main

import (
	"time"
)

type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Order struct {
	ID         int       `json:"id"`
	SenderId   int       `json:"sender_id"`
	ReceiverId int       `json:"receiver_id"`
	Name       string    `json:"name"`
	Meta       string    `json:"meta"`
	Comment    string    `json:"comment"`
	CreatedAt  time.Time `json:"created_at"`
}

type Scan struct {
	ID        int       `json:"id"`
	OrderId   int       `json:"order_id"`
	CourierId int       `json:"courier_id"`
	Photo     byte      `json:"photo"`
	Condition string    `json:"condition"`
	Longitude float32   `json:"longitude"`
	Latitude  float32   `json:"latitude"`
	Comment   string    `json:"string"`
	CreatedAt time.Time `json:"created_at"`
}
