package main

import (
	"encoding/json"
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
	users, err := GetOrders()
	if err != nil {
		jsonError(w, "Failed to fetch orders", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, *users)
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

	orderId := fmt.Sprintf("%d", req.OrderId)
	order, err := GetOrder(orderId)
	if err != nil {
		jsonError(w, "Order not found", http.StatusNotFound)
		return
	}

	if order.Status == "cancelled" {
		jsonError(w, "Cannot scan a cancelled order", http.StatusConflict)
		return
	}
	if order.Status == "delivered" {
		jsonError(w, "Cannot scan a delivered order", http.StatusConflict)
		return
	}

	err = CreateOrderScan(&req, courier)
	if err != nil {
		log.Printf("create order scan error: %v", err)
		jsonError(w, "Could not create order scan", http.StatusInternalServerError)
		return
	}

	if order.Status == "pending" {
		_, err = UpdateOrderStatus(orderId, "in-transit")
		if err != nil {
			log.Printf("auto status update error: %v", err)
		}
	}

	jsonResponse(w, "all good")
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

		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			jsonError(w, "Email already exists", http.StatusConflict)
			return
		}

		jsonError(w, "Could not create user", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, *user)
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
