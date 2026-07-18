package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/confuzeus/replyforge/internal/auth"
)

func main() {
	fmt.Print("Enter password: ")
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading password:", err)
		os.Exit(1)
	}
	password = strings.TrimSuffix(password, "\n")
	password = strings.TrimSuffix(password, "\r")

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
