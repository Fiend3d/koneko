package main

import (
	"fmt"
	"os"
	"strings"
)

// User represents a user in the system.
type User struct {
	ID       int
	Name     string
	Email    string
	IsActive bool
}

const maxUsers = 100

func main() {
	users := []User{
		{ID: 1, Name: "Alice", Email: "alice@example.com", IsActive: true},
		{ID: 2, Name: "Bob", Email: "bob@example.com", IsActive: false},
		{ID: 3, Name: "Charlie", Email: "charlie@example.com", IsActive: true},
	}

	for _, u := range users {
		if u.IsActive {
			fmt.Printf("Hello, %s! (%s)\n", u.Name, u.Email)
		}
	}

	data, err := os.ReadFile("input.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	lines := strings.Split(string(data), "\n")
	fmt.Printf("Read %d lines\n", len(lines))
}
