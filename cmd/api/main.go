package main

import (
	"log"
	"net/http"
	"os"

	"github.com/AleksKAG/RoomScan-AI/internal/api"
)

func main() {
addr := ":8080"
if v := os.Getenv("PORT"); v != "" {
    addr = ":" + v
}

	r := api.NewRouter()

	log.Printf("starting RoomScan-AI API on %s\n", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
