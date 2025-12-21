package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
	// Load .env file
	if err := loadEnv(".env"); err != nil {
		log.Printf("Warning: could not load .env file: %v", err)
	}

	// Get configuration from environment
	apiKey := os.Getenv("VAULTSANDBOX_API_KEY")
	if apiKey == "" {
		log.Fatal("VAULTSANDBOX_API_KEY environment variable is required")
	}

	baseURL := os.Getenv("VAULTSANDBOX_URL")
	if baseURL == "" {
		log.Fatal("VAULTSANDBOX_URL environment variable is required")
	}

	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		log.Fatal("SMTP_HOST environment variable is required")
	}

	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "25"
	}

	// Create a new client
	client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(baseURL))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create a temporary inbox
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		log.Fatalf("Failed to create inbox: %v", err)
	}
	defer inbox.Delete(ctx)

	fmt.Printf("Created inbox: %s\n", inbox.EmailAddress())
	fmt.Printf("Expires at: %s\n", inbox.ExpiresAt().Format(time.RFC3339))

	// Send a test email via SMTP
	from := "test@example.com"
	to := inbox.EmailAddress()
	subject := "Test Email from VaultSandbox"
	body := "Hello from the VaultSandbox SMTP example!\n\nThis email was sent programmatically to test the library."

	fmt.Printf("\nSending email via SMTP to %s...\n", to)

	if err := sendEmail(smtpHost, smtpPort, from, to, subject, body); err != nil {
		log.Fatalf("Failed to send email: %v", err)
	}

	fmt.Println("Email sent successfully!")

	// Wait for the email to arrive
	fmt.Println("\nWaiting for email to arrive...")

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithSubject(subject),
		vaultsandbox.WithWaitTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to receive email: %v", err)
	}

	// Display the received email
	fmt.Println("\nReceived email:")
	fmt.Printf("  ID:       %s\n", email.ID)
	fmt.Printf("  From:     %s\n", email.From)
	fmt.Printf("  To:       %v\n", email.To)
	fmt.Printf("  Subject:  %s\n", email.Subject)
	fmt.Printf("  Received: %s\n", email.ReceivedAt.Format(time.RFC3339))
	fmt.Printf("  Body:\n%s\n", indent(email.Text, "    "))

	fmt.Println("\nTest completed successfully!")
}

// sendEmail sends an email via SMTP
func sendEmail(host, port, from, to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		from, to, subject, body)

	addr := fmt.Sprintf("%s:%s", host, port)
	return smtp.SendMail(addr, nil, from, []string{to}, []byte(msg))
}

// loadEnv loads environment variables from a file
func loadEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Only set if not already set in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// indent adds a prefix to each line of text
func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
