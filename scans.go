package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
)

func scan(w http.ResponseWriter, r *http.Request) {
	courier_id := getUserId(w, r)
	if courier_id == 0 {
		return
	}

	if err:=r.ParseMultipartForm(5*1024*1024); err!=nil{
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error while parsing form data id: %v", err)
		return
	}
	
	order := r.MultipartForm.Value["order"][0]
	condition := r.MultipartForm.Value["condition"][0]
	longitude := r.MultipartForm.Value["longitude"][0]
	latitude := r.MultipartForm.Value["latitude"][0]
	comment := r.MultipartForm.Value["comment"][0]

	if longitude == "" || latitude == "" || order == "" || condition == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	order_id, err := strconv.ParseUint(order, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error while parsing order id: %v", err)
		return
	}
	condition_i, err := strconv.ParseUint(condition, 10, 64)
	if err != nil || condition_i > 4 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error while parsing condition: %v", err)
		return
	}
	longitude_f, err := strconv.ParseFloat(longitude, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error while parsing longitude: %v", err)
		return
	}
	latitude_f, err := strconv.ParseFloat(latitude, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error while parsing latitude: %v", err)
		return
	}
	row := db.QueryRow("INSERT INTO scans (order_id, courier_id, condition, longitude, latitude) VALUES (?, ?, ?, ?, ?, ?) RETURNING id", order_id, courier_id, condition_i, longitude_f, latitude_f, comment)
	var ret uint
	err = row.Scan(&ret)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "Error while logging scan: %v\n", err)
		fmt.Fprintf(w, "Error while logging scan: %v", err)
		return
	}

	fmt.Fprint(w, ret)
}
