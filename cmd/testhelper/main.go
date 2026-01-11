package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

// ClientInterface defines the client operations used by testhelper.
// This allows for easy mocking in tests.
type ClientInterface interface {
	CreateInbox(ctx context.Context, opts ...vaultsandbox.InboxOption) (*vaultsandbox.Inbox, error)
	ImportInbox(ctx context.Context, data *vaultsandbox.ExportedInbox) (*vaultsandbox.Inbox, error)
	DeleteInbox(ctx context.Context, emailAddress string) error
}

// Config holds the I/O configuration for the testhelper commands.
type Config struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// DefaultConfig returns a Config using standard I/O.
func DefaultConfig() *Config {
	return &Config{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// clientFactory creates a vaultsandbox client. Can be replaced in tests.
var clientFactory = func() (ClientInterface, error) {
	return vaultsandbox.New(
		os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	)
}

func run(args []string, cfg *Config) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: testhelper <command> [args]")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := clientFactory()
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	switch args[1] {
	case "create-inbox":
		return runCreateInbox(ctx, client, cfg)
	case "import-inbox":
		return runImportInbox(ctx, client, cfg)
	case "read-emails":
		return runReadEmails(ctx, client, cfg)
	case "cleanup":
		if len(args) < 3 {
			return fmt.Errorf("usage: testhelper cleanup <address>")
		}
		return runCleanup(ctx, client, cfg, args[2])
	default:
		return fmt.Errorf("unknown command: %s", args[1])
	}
}

func runCreateInbox(ctx context.Context, client ClientInterface, cfg *Config) error {
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		return fmt.Errorf("create inbox: %w", err)
	}

	exported := inbox.Export()
	if err := json.NewEncoder(cfg.Stdout).Encode(exported); err != nil {
		return fmt.Errorf("encode export: %w", err)
	}
	return nil
}

func runImportInbox(ctx context.Context, client ClientInterface, cfg *Config) error {
	data, err := io.ReadAll(cfg.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var exportData vaultsandbox.ExportedInbox
	if err := json.Unmarshal(data, &exportData); err != nil {
		return fmt.Errorf("parse export: %w", err)
	}

	_, err = client.ImportInbox(ctx, &exportData)
	if err != nil {
		return fmt.Errorf("import inbox: %w", err)
	}

	json.NewEncoder(cfg.Stdout).Encode(map[string]bool{"success": true})
	return nil
}

type EmailOutput struct {
	ID          string             `json:"id"`
	Subject     string             `json:"subject"`
	From        string             `json:"from"`
	To          []string           `json:"to"`
	Text        string             `json:"text"`
	HTML        string             `json:"html,omitempty"`
	Attachments []AttachmentOutput `json:"attachments,omitempty"`
	ReceivedAt  string             `json:"receivedAt"`
}

type AttachmentOutput struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
}

// convertEmails converts vaultsandbox.Email slice to EmailOutput slice.
func convertEmails(emails []*vaultsandbox.Email) []EmailOutput {
	output := make([]EmailOutput, 0, len(emails))
	for _, email := range emails {
		e := EmailOutput{
			ID:         email.ID,
			Subject:    email.Subject,
			From:       email.From,
			To:         email.To,
			Text:       email.Text,
			HTML:       email.HTML,
			ReceivedAt: email.ReceivedAt.Format(time.RFC3339),
		}
		for _, att := range email.Attachments {
			e.Attachments = append(e.Attachments, AttachmentOutput{
				Filename:    att.Filename,
				ContentType: att.ContentType,
				Size:        len(att.Content),
			})
		}
		output = append(output, e)
	}
	return output
}

func runReadEmails(ctx context.Context, client ClientInterface, cfg *Config) error {
	data, err := io.ReadAll(cfg.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var exportData vaultsandbox.ExportedInbox
	if err := json.Unmarshal(data, &exportData); err != nil {
		return fmt.Errorf("parse export: %w", err)
	}

	inbox, err := client.ImportInbox(ctx, &exportData)
	if err != nil {
		return fmt.Errorf("import inbox: %w", err)
	}

	emails, err := inbox.GetEmails(ctx)
	if err != nil {
		return fmt.Errorf("list emails: %w", err)
	}

	output := struct {
		Emails []EmailOutput `json:"emails"`
	}{
		Emails: convertEmails(emails),
	}

	if err := json.NewEncoder(cfg.Stdout).Encode(output); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return nil
}

func runCleanup(ctx context.Context, client ClientInterface, cfg *Config, address string) error {
	if err := client.DeleteInbox(ctx, address); err != nil {
		return fmt.Errorf("delete inbox: %w", err)
	}
	json.NewEncoder(cfg.Stdout).Encode(map[string]bool{"success": true})
	return nil
}

// exitFunc is the function called to exit the program. Can be replaced in tests.
var exitFunc = os.Exit

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	exitFunc(1)
}
