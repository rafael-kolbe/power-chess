package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"power-chess/internal/server"
)

// main boots the HTTP/WebSocket server process.
func main() {
	_ = godotenv.Load()
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	s := server.NewServer()
	log.Printf("power-chess server listening on %s", addr)
	if err := http.ListenAndServe(addr, s.Routes()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
