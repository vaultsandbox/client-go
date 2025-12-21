package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
	apiKey := os.Getenv("VAULTSANDBOX_API_KEY")
	if apiKey == "" {
		log.Fatal("VAULTSANDBOX_API_KEY environment variable is required")
	}

	client, err := vaultsandbox.New(apiKey)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Create inbox
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		log.Fatalf("Failed to create inbox: %v", err)
	}
	defer inbox.Delete(ctx)

	fmt.Printf("Waiting for email at: %s\n", inbox.EmailAddress())

	// Wait for an email with a specific subject pattern
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)welcome`)),
		vaultsandbox.WithWaitTimeout(2*time.Minute),
	)
	if err != nil {
		log.Fatalf("Failed to wait for email: %v", err)
	}

	fmt.Printf("\nReceived email:\n")
	fmt.Printf("  Subject: %s\n", email.Subject)
	fmt.Printf("  From: %s\n", email.From)
	fmt.Printf("  Text: %s\n", email.Text)

	// Check authentication results
	if email.AuthResults != nil && email.AuthResults.IsPassing() {
		fmt.Println("\n  Email passed authentication checks!")
	}

	// Get links from the email
	if len(email.Links) > 0 {
		fmt.Printf("\n  Links found:\n")
		for _, link := range email.Links {
			fmt.Printf("    - %s\n", link)
		}
	}
}
