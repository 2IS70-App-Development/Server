package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	b64 "encoding/base64"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrPasswordTooShort = errors.New("password must be at least 8 characters")

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
	if len(password) < 8 {
		return nil, ErrPasswordTooShort
	}
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
	rows, err := Db.Query("SELECT id, sender_id, receiver_id, name, status, meta, comment, photo, created_at FROM orders WHERE sender_id = ? OR receiver_id = ?", userId, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.ID, &order.SenderId, &order.ReceiverId, &order.Name, &order.Status, &order.Meta, &order.Comment, &order.Photo, &order.CreatedAt); err != nil {
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
	err := Db.QueryRow("SELECT id, sender_id, receiver_id, name, status, meta, comment, photo, created_at FROM orders WHERE id = ?", id).Scan(&order.ID, &order.SenderId, &order.ReceiverId, &order.Name, &order.Status, &order.Meta, &order.Comment, &order.Photo, &order.CreatedAt)
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

	var photo []byte
	if data.PhotoBase64 != "" {
		decoded, err := b64.RawStdEncoding.DecodeString(data.PhotoBase64)
		if err != nil {
			return nil, err
		}
		photo = decoded
	}

	err := Db.QueryRow("INSERT INTO orders (sender_id, receiver_id, name, meta, comment, photo) VALUES(?, ?, ?, ?, ?, ?) RETURNING id, status, photo, created_at", sender.ID, data.ReceiverId, data.Name, data.Meta, data.Comment, photo).Scan(&newOrder.ID, &newOrder.Status, &newOrder.Photo, &newOrder.CreatedAt)
	if err != nil {
		return nil, err
	}

	// create denormalized activity entries for sender and receiver
	go func() {
		// best-effort async logging
		_ = CreateActivity(sender.ID, sender.ID, "order_created", fmt.Sprintf("Created order %d to %d", newOrder.ID, data.ReceiverId))
		_ = CreateActivity(sender.ID, data.ReceiverId, "order_received", fmt.Sprintf("Order %d created for you by %d", newOrder.ID, sender.ID))
	}()

	return &newOrder, nil
}

func UpdateOrderStatus(id string, status string) (*Order, error) {
	var order Order
	err := Db.QueryRow("UPDATE orders SET status = ? WHERE id = ? RETURNING id, sender_id, receiver_id, name, status, meta, comment, photo, created_at", status, id).Scan(&order.ID, &order.SenderId, &order.ReceiverId, &order.Name, &order.Status, &order.Meta, &order.Comment, &order.Photo, &order.CreatedAt)
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

func GetAllScans() ([]Scan, error) {
	var scans []Scan
	rows, err := Db.Query("SELECT id, order_id, courier_id, photo, condition, longitude, latitude, comment, created_at FROM scans")
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
	// log input summary (avoid logging raw base64 content)
	log.Printf("CreateOrderScan: start - order_id=%d courier_id=%d condition=%s lon=%f lat=%f comment_len=%d photo_b64_len=%d", data.OrderId, courier.ID, data.Condition, data.Longitude, data.Latitude, len(data.Comment), len(data.PhotoBase64))

	photo, err := decodeBase64(data.PhotoBase64, data.OrderId, courier.ID)
	if err != nil {
		return err
	}

	log.Printf("CreateOrderScan: decoded photo bytes=%d for order_id=%d", len(photo), data.OrderId)

	res, err := Db.Exec("INSERT INTO scans (order_id, courier_id, photo, condition, longitude, latitude, comment) VALUES(?, ?, ?, ?, ?, ?, ?)", data.OrderId, courier.ID, photo, data.Condition, data.Longitude, data.Latitude, data.Comment)
	if err != nil {
		log.Printf("CreateOrderScan: DB insert error for order_id=%d courier_id=%d: %v", data.OrderId, courier.ID, err)
		return err
	}

	if res != nil {
		if id, err2 := res.LastInsertId(); err2 == nil {
			log.Printf("CreateOrderScan: DB insert succeeded id=%d order_id=%d", id, data.OrderId)
		}
	}

	return nil
}

// decodeBase64 accepts common base64 payload formats: data URLs, padded and unpadded variants,
// and strips whitespace/newlines. It tries StdEncoding first, then RawStdEncoding, then URL encodings.
func decodeBase64(s string, orderId int, courierId int) ([]byte, error) {
	if s == "" {
		return nil, errors.New("empty photo base64")
	}

	// strip possible data URL prefix: data:<mediatype>;base64,<data>
	if idx := strings.Index(s, ","); idx != -1 && strings.HasPrefix(s, "data:") {
		s = s[idx+1:]
	}

	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, " ", "")

	// try standard padded base64 first
	if decoded, err := b64.StdEncoding.DecodeString(s); err == nil {
		return decoded, nil
	} else {
		log.Printf("decodeBase64: StdEncoding failed for order_id=%d courier_id=%d: %v", orderId, courierId, err)
	}

	// try raw (unpadded) standard encoding
	if decoded, err := b64.RawStdEncoding.DecodeString(s); err == nil {
		return decoded, nil
	} else {
		log.Printf("decodeBase64: RawStdEncoding failed for order_id=%d courier_id=%d: %v", orderId, courierId, err)
	}

	// try URL-safe encodings
	if decoded, err := b64.URLEncoding.DecodeString(s); err == nil {
		return decoded, nil
	} else {
		log.Printf("decodeBase64: URLEncoding failed for order_id=%d courier_id=%d: %v", orderId, courierId, err)
	}
	if decoded, err := b64.RawURLEncoding.DecodeString(s); err == nil {
		return decoded, nil
	} else {
		log.Printf("decodeBase64: RawURLEncoding failed for order_id=%d courier_id=%d: %v", orderId, courierId, err)
	}

	return nil, fmt.Errorf("base64 decode failed for order %d courier %d", orderId, courierId)
}

// CreateActivity inserts a denormalized activity row.
func CreateActivity(actorId int, userId int, typ string, summary string) error {
	_, err := Db.Exec("INSERT INTO activities (actor_id, user_id, type, summary) VALUES(?, ?, ?, ?)", actorId, userId, typ, summary)
	return err
}

// GetActivitiesForUser returns activities for a given user ordered newest first.
func GetActivitiesForUser(userId int) ([]Activity, error) {
	var acts []Activity
	rows, err := Db.Query("SELECT id, actor_id, user_id, type, summary, created_at FROM activities WHERE user_id = ? ORDER BY created_at DESC", userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.ActorId, &a.UserId, &a.Type, &a.Summary, &a.CreatedAt); err != nil {
			return nil, err
		}
		acts = append(acts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return acts, nil
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
	if len(password) < 8 {
		return "", ErrPasswordTooShort
	}
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
