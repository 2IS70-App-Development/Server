package main

import (
	"fmt"
	"strconv"
	"time"

	b64 "encoding/base64"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func GetUsers() (*[]User, error) {
	var users []User
	rows, err := Db.Query("SELECT id, email, created_at FROM users")
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

func GetUser(id string) (*User, error) {
	var user User
	err := Db.QueryRow("SELECT id, email, password_hash, created_at FROM users WHERE id = ?", id).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByEmail(email string) (*User, error) {
	var user User
	err := Db.QueryRow("SELECT id, email, password_hash, created_at FROM users WHERE email = ?", email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func CreateUser(email string, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var newUser User
	err = Db.QueryRow("INSERT INTO users (email, password_hash) VALUES(?, ?) RETURNING id, email, created_at", email, string(hash)).Scan(&newUser.ID, &newUser.Email, &newUser.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &newUser, nil
}

func GetOrders(userId int) (*[]Order, error) {
	var orders []Order
	rows, err := Db.Query("SELECT id, sender_id, receiver_id, name, status, meta, comment, created_at FROM orders WHERE sender_id = ? OR receiver_id = ?", userId, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.ID, &order.SenderId, &order.ReceiverId, &order.Name, &order.Status, &order.Meta, &order.Comment, &order.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &orders, nil
}

func GetOrder(id string) (*Order, error) {
	var order Order
	err := Db.QueryRow("SELECT id, sender_id, receiver_id, name, status, meta, comment, created_at FROM orders WHERE id = ?", id).Scan(&order.ID, &order.SenderId, &order.ReceiverId, &order.Name, &order.Status, &order.Meta, &order.Comment, &order.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func CreateOrderService(data *CreateOrder, sender *User) (*Order, error) {
	var newOrder = Order{
		SenderId:   sender.ID,
		ReceiverId: data.ReceiverId,
		Name:       data.Name,
		Meta:       data.Meta,
		Comment:    data.Comment,
	}

	err := Db.QueryRow("INSERT INTO orders (sender_id, receiver_id, name, meta, comment) VALUES(?, ?, ?, ?, ?) RETURNING id, status, created_at", sender.ID, data.ReceiverId, data.Name, data.Meta, data.Comment).Scan(&newOrder.ID, &newOrder.Status, &newOrder.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &newOrder, nil
}

func UpdateOrderStatus(id string, status string) (*Order, error) {
	var order Order
	err := Db.QueryRow("UPDATE orders SET status = ? WHERE id = ? RETURNING id, sender_id, receiver_id, name, status, meta, comment, created_at", status, id).Scan(&order.ID, &order.SenderId, &order.ReceiverId, &order.Name, &order.Status, &order.Meta, &order.Comment, &order.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func GetOrderScans(orderId string) ([]Scan, error) {
	var scans []Scan
	rows, err := Db.Query("SELECT id, order_id, courier_id, photo, condition, longitude, latitude, comment, created_at FROM scans WHERE order_id = ?", orderId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var scan Scan
		if err := rows.Scan(&scan.ID, &scan.OrderId, &scan.CourierId, &scan.Photo, &scan.Condition, &scan.Longitude, &scan.Latitude, &scan.Comment, &scan.CreatedAt); err != nil {
			return nil, err
		}
		scans = append(scans, scan)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return scans, nil
}

func CreateOrderScan(data *CreateOrderScanRequest, courier *User) error {
	photo, err := b64.RawStdEncoding.DecodeString(data.PhotoBase64)
	if err != nil {
		return err
	}

	_, err = Db.Exec("INSERT INTO scans (order_id, courier_id, photo, condition, longitude, latitude, comment) VALUES(?, ?, ?, ?, ?, ?, ?)", data.OrderId, courier.ID, photo, data.Condition, data.Longitude, data.Latitude, data.Comment)
	if err != nil {
		return err
	}

	return nil
}

func GetContacts(ownerId int) (*[]Contact, error) {
	var contacts []Contact
	rows, err := Db.Query("SELECT owner_id, contact_id FROM contacts WHERE owner_id = ?", ownerId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var contact Contact
		if err := rows.Scan(&contact.OwnerId, &contact.ContactId); err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &contacts, nil
}

func AddContact(ownerId int, contactUserId int) (*Contact, error) {
	_, err := Db.Exec("INSERT INTO contacts (owner_id, contact_id) VALUES (?, ?)", ownerId, contactUserId)
	if err != nil {
		return nil, err
	}

	return &Contact{OwnerId: ownerId, ContactId: contactUserId}, nil
}

func RemoveContact(ownerId int, contactUserId int) error {
	_, err := Db.Exec("DELETE FROM contacts WHERE owner_id = ? AND contact_id = ?", ownerId, contactUserId)
	return err
}

func JwtCreateService(user *User, password string) (string, error) {
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
	ss, err := token.SignedString(jwtSecretKey)

	if err != nil {
		return "", err
	}

	return ss, nil
}
