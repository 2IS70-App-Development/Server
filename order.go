package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
)

func createOrder(w http.ResponseWriter, r *http.Request) {
	user_id := getUserId(w, r)
	if user_id == 0 {
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	name := r.PostForm.Get("name")
	reciever := r.PostForm.Get("reciever")
	meta := r.PostForm.Get("meta") //TODO what is this?
	comment := r.PostForm.Get("comment")
	if name == "" || reciever == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	recv_id, err := strconv.ParseUint(reciever, 10, 64)
	if err!=nil{
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error while parsing reciever id: %v", err)
		return
	}

	row := db.QueryRow("INSERT INTO orders (sender_id, reciever_id, name, meta, comment) VALUES (?, ?, ?, ?, ?) RETURNING id", user_id, recv_id, name, meta, comment)
	var order_id uint
	err = row.Scan(&order_id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "Error while creating order: %v\n", err)
		fmt.Fprintf(w, "Error while creating order: %v", err)
		return
	}

	fmt.Fprint(w, order_id)
}
