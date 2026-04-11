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
		err := server.ListenAndServe(":8080")
		if err != nil {
			return
		}
		return
	}

	server := NewServerWithLoader(loader)
	err = server.ListenAndServe(":8080")
	if err != nil {
		return
	}
}
