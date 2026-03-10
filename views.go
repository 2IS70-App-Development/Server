package main

import (
	"encoding/json"
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
