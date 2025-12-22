package main

import (
	"log"
	"net/http"

	"git.sr.ht/~jakintosh/todo/internal/store"
	"git.sr.ht/~jakintosh/todo/internal/web"
)

func main() {
	// Initialize Store
	s, err := store.NewSQLiteStore("todo.db")
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Initialize Web Server
	srv, err := web.NewServer(s)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Start Server
	log.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", srv); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
