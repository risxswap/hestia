package main

import (
	"log"
	"net/http"

	userapp "hestia/server/internal/app/user"
)

func main() {
	router := userapp.NewRouter()
	log.Println("user-server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
