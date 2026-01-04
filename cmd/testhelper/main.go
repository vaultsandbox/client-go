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

func main() {
	if len(os.Args) < 2 {
		fatal("usage: testhelper <command> [args]")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := vaultsandbox.New(
		os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	)
	if err != nil {
		fatal("create client: %v", err)
	}

	switch os.Args[1] {
	case "create-inbox":
		createInbox(ctx, client)
	case "import-inbox":
		importInbox(ctx, client)
	case "read-emails":
		readEmails(ctx, client)
	case "cleanup":
		if len(os.Args) < 3 {
			fatal("usage: testhelper cleanup <address>")
		}
		cleanup(ctx, client, os.Args[2])
	default:
		fatal("unknown command: %s", os.Args[1])
	}
}

func createInbox(ctx context.Context, client *vaultsandbox.Client) {
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		fatal("create inbox: %v", err)
	}

	exported := inbox.Export()
	if err := json.NewEncoder(os.Stdout).Encode(exported); err != nil {
		fatal("encode export: %v", err)
	}
}

func importInbox(ctx context.Context, client *vaultsandbox.Client) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal("read stdin: %v", err)
	}

	var exportData vaultsandbox.ExportedInbox
	if err := json.Unmarshal(data, &exportData); err != nil {
		fatal("parse export: %v", err)
	}

	_, err = client.ImportInbox(ctx, &exportData)
	if err != nil {
		fatal("import inbox: %v", err)
	}

	json.NewEncoder(os.Stdout).Encode(map[string]bool{"success": true})
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

func readEmails(ctx context.Context, client *vaultsandbox.Client) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal("read stdin: %v", err)
	}

	var exportData vaultsandbox.ExportedInbox
	if err := json.Unmarshal(data, &exportData); err != nil {
		fatal("parse export: %v", err)
	}

	inbox, err := client.ImportInbox(ctx, &exportData)
	if err != nil {
		fatal("import inbox: %v", err)
	}

	emails, err := inbox.GetEmails(ctx)
	if err != nil {
		fatal("list emails: %v", err)
	}

	output := struct {
		Emails []EmailOutput `json:"emails"`
	}{
		Emails: make([]EmailOutput, 0, len(emails)),
	}

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
		output.Emails = append(output.Emails, e)
	}

	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fatal("encode output: %v", err)
	}
}

func cleanup(ctx context.Context, client *vaultsandbox.Client, address string) {
	if err := client.DeleteInbox(ctx, address); err != nil {
		fatal("delete inbox: %v", err)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]bool{"success": true})
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
