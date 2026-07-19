package main

import (
	"fmt"
	"os"

	"github.com/confuzeus/replyforge/internal/auth"
	"golang.org/x/term"
)

func main() {
	fmt.Print("Enter password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading password:", err)
		os.Exit(1)
	}

	password := string(passwordBytes)
	if password == "" {
		fmt.Fprintln(os.Stderr, "password cannot be empty")
		os.Exit(1)
	}

	hash, err := auth.GenerateHash(password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error generating hash:", err)
		os.Exit(1)
	}

	fmt.Println("\nAdd this to your .env file:")
	fmt.Printf("ADMIN_PASSWORD_HASH=%s\n", hash)
}
