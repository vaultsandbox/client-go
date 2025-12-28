// Package vaultsandbox provides a Go client SDK for VaultSandbox,
// a secure receive-only SMTP server for QA/testing environments.
//
// The SDK enables creating temporary email inboxes with quantum-safe encryption
// using ML-KEM-768 for key encapsulation and ML-DSA-65 for signatures.
//
// Basic usage:
//
//	client, err := vaultsandbox.New("your-api-key")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Create a temporary inbox
//	inbox, err := client.CreateInbox(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Wait for an email
//	email, err := inbox.WaitForEmail(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Println("Subject:", email.Subject)
package vaultsandbox
