package main

import (
	"log"

	"helvilette/pkg/playbook"
)

func main() {
	// Try to load playbooks from data directory
	loader, err := playbook.NewLoader("helvillette/othela/data/playbooks")
	if err != nil {
		log.Printf("[WARN] Could not initialize playbook loader: %v", err)
		log.Printf("[WARN] Falling back to default server (no playbook loading)")
		server := NewServer()
		server.ListenAndServe(":8080")
		return
	}

	server := NewServerWithLoader(loader)
	server.ListenAndServe(":8080")
}

