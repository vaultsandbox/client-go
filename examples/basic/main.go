package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
	apiKey := os.Getenv("VAULTSANDBOX_API_KEY")
	if apiKey == "" {
		log.Fatal("VAULTSANDBOX_API_KEY environment variable is required")
	}

	// Create a new client
	client, err := vaultsandbox.New(apiKey)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create a temporary inbox
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		log.Fatalf("Failed to create inbox: %v", err)
	}

	fmt.Printf("Created inbox: %s\n", inbox.EmailAddress())
	fmt.Printf("Expires at: %s\n", inbox.ExpiresAt().Format(time.RFC3339))

	// At this point, you can send emails to the inbox address
	fmt.Println("\nSend an email to the address above, then press Enter...")
	fmt.Scanln()

	// Fetch emails
	emails, err := inbox.GetEmails(ctx)
	if err != nil {
		log.Fatalf("Failed to get emails: %v", err)
	}

	fmt.Printf("\nFound %d email(s):\n", len(emails))
	for _, email := range emails {
		fmt.Printf("  - Subject: %s\n", email.Subject)
		fmt.Printf("    From: %s\n", email.From)
		fmt.Printf("    Received: %s\n", email.ReceivedAt.Format(time.RFC3339))
	}

	// Clean up
	if err := inbox.Delete(ctx); err != nil {
		log.Printf("Failed to delete inbox: %v", err)
	}
}
