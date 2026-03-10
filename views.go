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

func (a *App) getUsersList(w http.ResponseWriter, r *http.Request) {
	users, err := a.getUsers()
	if err != nil {
		jsonError(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, *users)
}

func (a *App) getUserDetails(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")

	user, err := a.getUser(username)
	if err != nil {
		jsonError(w, "User not found", http.StatusBadRequest)
		return
	}

	jsonResponse(w, *user)
}

func (a *App) signup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		jsonError(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	user, err := a.createUser(req.Email, req.Password)
	if err != nil {
		log.Printf(err.Error())

		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			jsonError(w, "Email already exists", http.StatusConflict)
			return
		}

		jsonError(w, "Could not create user", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, *user)
}

func (a *App) jwtCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := a.getUser(req.Email)
	if err != nil {
		log.Printf(err.Error())
		jsonError(w, err.Error(), http.StatusOK)
		return
	}

	accessToken, err := a.jwtCreateService(user, req.Password)
	if err != nil {
		log.Printf(err.Error())
		jsonError(w, err.Error(), http.StatusOK)
		return
	}

	log.Printf(accessToken)

	jsonResponse(w, map[string]string{
		"access_token": accessToken,
	})
}
