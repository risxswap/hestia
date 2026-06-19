package main

import (
	"log"
	"net/http"

	adminapp "hestia/server/internal/app/admin"
)

func main() {
	router := adminapp.NewRouter()
	log.Println("admin-server listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", router))
}
