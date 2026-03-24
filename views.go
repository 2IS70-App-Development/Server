package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func GetUsersList(w http.ResponseWriter, r *http.Request) {
	users, err := GetUsers()
	if err != nil {
		jsonError(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, *users)
}

func GetUserDetails(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	user, err := GetUser(id)
	if err != nil {
		jsonError(w, "User not found", http.StatusBadRequest)
		return
	}

	jsonResponse(w, *user)
}

func GetOrdersList(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(contextKeyUser).(*User)
	if !ok || user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	orders, err := GetOrders(user.ID)
	if err != nil {
		jsonError(w, "Failed to fetch orders", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, *orders)
}

func GetOrderDetails(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	order, err := GetOrder(id)
	if err != nil {
		jsonError(w, "Order not found", http.StatusBadRequest)
		return
	}

	jsonResponse(w, *order)
}

type CreateOrder struct {
	ReceiverId int    `json:"receiver_id"`
	Name       string `json:"name"`
	Meta       string `json:"meta"`
	Comment    string `json:"comment"`
	PhotoBase64 string `json:"photo_base64,omitempty"`
}

func CreateOrderEndpoint(w http.ResponseWriter, r *http.Request) {
	sender, ok := r.Context().Value(contextKeyUser).(*User)
	if !ok || sender == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateOrder
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		jsonError(w, "Name is required", http.StatusBadRequest)
		return
	}

	receiverId := fmt.Sprintf("%d", req.ReceiverId)
	_, err := GetUser(receiverId)
	if err != nil {
		jsonError(w, "Receiver not found", http.StatusBadRequest)
		return
	}

	if req.ReceiverId == sender.ID {
		jsonError(w, "Cannot send an order to yourself", http.StatusBadRequest)
		return
	}

	order, err := CreateOrderService(&req, sender)
	if err != nil {
		log.Printf("create order error: %v", err)
		jsonError(w, "Could not create order", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, *order)
}

func UpdateOrderStatusEndpoint(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(contextKeyUser).(*User)
	if !ok || user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		OrderId int    `json:"order_id"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	allowedStatuses := map[string]bool{
		"pending": true, "in-transit": true, "delivered": true, "cancelled": true,
	}
	if !allowedStatuses[req.Status] {
		jsonError(w, "Invalid status. Allowed: pending, in-transit, delivered, cancelled", http.StatusBadRequest)
		return
	}

	orderId := fmt.Sprintf("%d", req.OrderId)
	order, err := GetOrder(orderId)
	if err != nil {
		jsonError(w, "Order not found", http.StatusNotFound)
		return
	}

	if order.SenderId != user.ID && order.ReceiverId != user.ID {
		jsonError(w, "Forbidden", http.StatusForbidden)
		return
	}

	if order.Status == "cancelled" {
		jsonError(w, "Cannot update a cancelled order", http.StatusConflict)
		return
	}
	if order.Status == "delivered" {
		jsonError(w, "Cannot update a delivered order", http.StatusConflict)
		return
	}

	updated, err := UpdateOrderStatus(orderId, req.Status)
	if err != nil {
		log.Printf("update order status error: %v", err)
		jsonError(w, "Could not update order status", http.StatusInternalServerError)
		return
	}

	// create activity entries for sender and receiver (best-effort async)
	go func() {
		_ = CreateActivity(user.ID, updated.SenderId, "status_changed", fmt.Sprintf("Order %d status changed to %s by %s", updated.ID, updated.Status, user.Email))
		_ = CreateActivity(user.ID, updated.ReceiverId, "status_changed", fmt.Sprintf("Order %d status changed to %s by %s", updated.ID, updated.Status, user.Email))
	}()

	jsonResponse(w, *updated)
}

func GetOrderScansEndpoint(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(contextKeyUser).(*User)
	if !ok || user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	orderId := r.URL.Query().Get("order_id")
	if orderId == "" {
		jsonError(w, "Missing order_id parameter", http.StatusBadRequest)
		return
	}

	order, err := GetOrder(orderId)
	if err != nil {
		jsonError(w, "Order not found", http.StatusNotFound)
		return
	}

	if order.SenderId != user.ID && order.ReceiverId != user.ID {
		jsonError(w, "Forbidden", http.StatusForbidden)
		return
	}

	scans, err := GetOrderScans(orderId)
	if err != nil {
		log.Printf("get order scans error: %v", err)
		jsonError(w, "Could not fetch scans", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, scans)
}

func GetAllScansEndpoint(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(contextKeyUser).(*User)
	if !ok || user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	scans, err := GetAllScans()
	if err != nil {
		log.Printf("get all scans error: %v", err)
		jsonError(w, "Could not fetch scans", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, scans)
}

type CreateOrderScanRequest struct {
	OrderId     int     `json:"order_id"`
	PhotoBase64 string  `json:"photo_base64"`
	Condition   string  `json:"condition"`
	Longitude   float32 `json:"longitude"`
	Latitude    float32 `json:"latitude"`
	Comment     string  `json:"comment"`
}

func CreateOrderScanEndpoint(w http.ResponseWriter, r *http.Request) {
	courier, ok := r.Context().Value(contextKeyUser).(*User)
	if !ok || courier == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateOrderScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("CreateOrderScanEndpoint: start - courier_id=%d, order_id=%d, condition=%s, photo_len=%d", courier.ID, req.OrderId, req.Condition, len(req.PhotoBase64))

	allowedConditions := map[string]bool{
		"Good": true, "Damaged": true, "Missing": true,
	}
	if !allowedConditions[req.Condition] {
		log.Printf("CreateOrderScanEndpoint: invalid condition %s for courier %d", req.Condition, courier.ID)
		jsonError(w, "Invalid condition. Allowed: Good, Damaged, Missing", http.StatusBadRequest)
		return
	}

	orderId := fmt.Sprintf("%d", req.OrderId)
	order, err := GetOrder(orderId)
	if err != nil {
		log.Printf("CreateOrderScanEndpoint: order not found id=%s courier=%d err=%v", orderId, courier.ID, err)
		jsonError(w, "Order not found", http.StatusNotFound)
		return
	}

	if order.Status == "cancelled" {
		log.Printf("CreateOrderScanEndpoint: cannot scan cancelled order id=%d courier=%d", order.ID, courier.ID)
		jsonError(w, "Cannot scan a cancelled order", http.StatusConflict)
		return
	}
	if order.Status == "delivered" {
		log.Printf("CreateOrderScanEndpoint: cannot scan delivered order id=%d courier=%d", order.ID, courier.ID)
		jsonError(w, "Cannot scan a delivered order", http.StatusConflict)
		return
	}

	log.Printf("CreateOrderScanEndpoint: calling CreateOrderScan for order_id=%d courier=%d", req.OrderId, courier.ID)
	err = CreateOrderScan(&req, courier)

	if err != nil {
		log.Printf("CreateOrderScanEndpoint: create order scan error: %v", err)
		jsonError(w, "Could not create order scan", http.StatusInternalServerError)
		return
	}

	log.Printf("CreateOrderScanEndpoint: CreateOrderScan succeeded for order_id=%d courier=%d", req.OrderId, courier.ID)

	if order.Status == "pending" {
		_, err = UpdateOrderStatus(orderId, "in-transit")
		if err != nil {
			log.Printf("CreateOrderScanEndpoint: failed to update order status to in-transit for order_id=%d courier=%d: %v", order.ID, courier.ID, err)
		} else {
			log.Printf("CreateOrderScanEndpoint: order status updated to in-transit for order_id=%d courier=%d", order.ID, courier.ID)
		}
	}

	// create activity entries: for courier (self), and notify sender & receiver
	go func() {
		_ = CreateActivity(courier.ID, courier.ID, "scan_created", fmt.Sprintf("Scan created for order %d", req.OrderId))
		_ = CreateActivity(courier.ID, order.SenderId, "scan_added", fmt.Sprintf("Order %d scanned by %s", req.OrderId, courier.Email))
		_ = CreateActivity(courier.ID, order.ReceiverId, "scan_added", fmt.Sprintf("Order %d scanned by %s", req.OrderId, courier.Email))
	}()

	jsonResponse(w, "all good")
}

func GetActivitiesEndpoint(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(contextKeyUser).(*User)
	if !ok || user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	acts, err := GetActivitiesForUser(user.ID)
	if err != nil {
		log.Printf("get activities error: %v", err)
		jsonError(w, "Could not fetch activities", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, acts)
}

func Signup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := CreateUser(req.Email, req.Password)
	if err != nil {
		log.Printf("signup error: %v", err)

		if errors.Is(err, ErrPasswordTooShort) {
			jsonError(w, "Password must be at least 8 characters long", http.StatusBadRequest)
			return
		}

		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			jsonError(w, "Username already exists", http.StatusConflict)
			return
		}

		jsonError(w, "Could not create user", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, *user)
}

func GetContactsEndpoint(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(contextKeyUser).(*User)
	if !ok || user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	contacts, err := GetContacts(user.ID)
	if err != nil {
		log.Printf("get contacts error: %v", err)
		jsonError(w, "Failed to fetch contacts", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, *contacts)
}

type AddContactRequest struct {
	ContactId int `json:"contact_id"`
}

func AddContactEndpoint(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(contextKeyUser).(*User)
	if !ok || user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req AddContactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ContactId == user.ID {
		jsonError(w, "Cannot add yourself as a contact", http.StatusBadRequest)
		return
	}

	contact, err := AddContact(user.ID, req.ContactId)
	if err != nil {
		log.Printf("add contact error: %v", err)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			jsonError(w, "Contact already exists", http.StatusConflict)
			return
		}
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			jsonError(w, "Contact user not found", http.StatusBadRequest)
			return
		}
		jsonError(w, "Could not add contact", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, *contact)
}

type RemoveContactRequest struct {
	ContactId int `json:"contact_id"`
}

func RemoveContactEndpoint(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(contextKeyUser).(*User)
	if !ok || user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req RemoveContactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := RemoveContact(user.ID, req.ContactId)
	if err != nil {
		log.Printf("remove contact error: %v", err)
		jsonError(w, "Could not remove contact", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func JwtCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := GetUserByEmail(req.Email)
	if err != nil {
		jsonError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	accessToken, err := JwtCreateService(user, req.Password)
	if err != nil {
		jsonError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	jsonResponse(w, map[string]string{
		"access_token": accessToken,
	})
}
