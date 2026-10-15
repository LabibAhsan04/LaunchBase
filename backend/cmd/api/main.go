package main

import (
	"log"
	"net/http"
	"os"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/httpapi"
)

func main() {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("api listening on %s", addr)
	err := http.ListenAndServe(addr, httpapi.NewMux())
	if err != nil {
		log.Fatal(err)
	}
}
