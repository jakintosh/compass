package main

import (
	"flag"
	"log"
	"net/http"

	"git.sr.ht/~jakintosh/todo/internal/store"
	"git.sr.ht/~jakintosh/todo/internal/web"
)

func main() {
	// Parse CLI flags
	publicMode := flag.Bool("public", false, "Run in public read-only mode (no authentication)")
	flag.Parse()

	// Initialize Store
	store, err := store.NewSQLiteStore("todo.db", true)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Initialize Web Server
	opts := web.ServerOptions{
		PublicMode: *publicMode,
	}
	srv, err := web.NewServer(store, opts)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Start Server
	if *publicMode {
		log.Println("Starting server in PUBLIC (read-only) mode on :8080...")
	} else {
		log.Println("Starting server in AUTHENTICATED mode on :8080...")
	}
	if err := http.ListenAndServe(":8080", srv); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
