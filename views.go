package main

import (
	"encoding/json"
	"net/http"
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

func (a *App) getUserDetail(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")

	user, err := a.getUser(username)
	if err != nil {
		jsonError(w, "User not found", http.StatusBadRequest)
		return
	}

	jsonResponse(w, *user)
}
