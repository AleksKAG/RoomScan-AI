package main

import (
	"log"
	"net/http"
	"os"

	"github.com/yourname/roomscan/internal/api"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("ROOMSCAN_ADDR"); v != "" {
		addr = v
	}

	r := api.NewRouter()

	log.Printf("starting RoomScan API on %s\n", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
