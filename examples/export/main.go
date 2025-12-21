package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

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

	fmt.Printf("Created inbox: %s\n", inbox.EmailAddress())

	// Export the inbox
	exported := inbox.Export()

	// Serialize to JSON
	data, err := json.MarshalIndent(exported, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal exported inbox: %v", err)
	}

	fmt.Printf("\nExported inbox:\n%s\n", string(data))

	// Simulate restoring the inbox later
	fmt.Println("\n--- Simulating import ---")

	// Parse the exported data
	var importData vaultsandbox.ExportedInbox
	if err := json.Unmarshal(data, &importData); err != nil {
		log.Fatalf("Failed to unmarshal exported inbox: %v", err)
	}

	// Import the inbox
	restoredInbox, err := client.ImportInbox(ctx, &importData)
	if err != nil {
		log.Fatalf("Failed to import inbox: %v", err)
	}

	fmt.Printf("Restored inbox: %s\n", restoredInbox.EmailAddress())
	fmt.Printf("Expires at: %s\n", restoredInbox.ExpiresAt())

	// The restored inbox can now receive and decrypt emails
	emails, err := restoredInbox.GetEmails(ctx)
	if err != nil {
		log.Fatalf("Failed to get emails: %v", err)
	}

	fmt.Printf("Found %d email(s) in restored inbox\n", len(emails))
}
