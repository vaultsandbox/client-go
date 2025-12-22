---
title: Working with Attachments
description: Access, decode, and validate email attachments
---

VaultSandbox automatically decrypts email attachments and provides them as `[]byte` slices ready to process.

## Accessing Attachments

### Basic Access

```go
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Invoice`)),
	vaultsandbox.WithWaitTimeout(10*time.Second),
)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("Email has %d attachments\n", len(email.Attachments))

for _, att := range email.Attachments {
	fmt.Printf("- %s (%s, %d bytes)\n", att.Filename, att.ContentType, att.Size)
}
```

## Attachment Structure

### Attachment Fields

```go
attachment := email.Attachments[0]

fmt.Println(attachment.Filename)           // "invoice.pdf"
fmt.Println(attachment.ContentType)        // "application/pdf"
fmt.Println(attachment.Size)               // 15234 (bytes)
fmt.Println(len(attachment.Content))       // []byte length
fmt.Println(attachment.ContentID)          // "part123@example.com" (optional)
fmt.Println(attachment.ContentDisposition) // "attachment" or "inline"
fmt.Println(attachment.Checksum)           // SHA-256 hash for integrity
```

## Working with Different File Types

### Text Files

```go
var txtAttachment *vaultsandbox.Attachment
for i := range email.Attachments {
	if strings.Contains(email.Attachments[i].ContentType, "text") {
		txtAttachment = &email.Attachments[i]
		break
	}
}

if txtAttachment != nil && txtAttachment.Content != nil {
	text := string(txtAttachment.Content)
	fmt.Println("Text content:", text)

	// Validate content
	if !strings.Contains(text, "expected text") {
		t.Error("expected text not found")
	}
}
```

### JSON Files

```go
var jsonAttachment *vaultsandbox.Attachment
for i := range email.Attachments {
	att := &email.Attachments[i]
	if strings.Contains(att.ContentType, "json") || strings.HasSuffix(att.Filename, ".json") {
		jsonAttachment = att
		break
	}
}

if jsonAttachment != nil && jsonAttachment.Content != nil {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonAttachment.Content, &data); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Parsed JSON:", data)

	if data["status"] != "success" {
		t.Error("expected status to be success")
	}
}
```

### CSV Files

```go
var csvAttachment *vaultsandbox.Attachment
for i := range email.Attachments {
	if strings.HasSuffix(email.Attachments[i].Filename, ".csv") {
		csvAttachment = &email.Attachments[i]
		break
	}
}

if csvAttachment != nil && csvAttachment.Content != nil {
	csvText := string(csvAttachment.Content)
	lines := strings.Split(csvText, "\n")

	fmt.Printf("CSV has %d lines\n", len(lines))

	if !strings.Contains(lines[0], "Name,Email,Status") {
		t.Error("expected CSV headers not found")
	}
}
```

### PDF Files

```go
var pdfAttachment *vaultsandbox.Attachment
for i := range email.Attachments {
	if email.Attachments[i].ContentType == "application/pdf" {
		pdfAttachment = &email.Attachments[i]
		break
	}
}

if pdfAttachment != nil && pdfAttachment.Content != nil {
	// Save to disk
	if err := os.WriteFile("invoice.pdf", pdfAttachment.Content, 0644); err != nil {
		log.Fatal(err)
	}

	// Verify PDF signature
	if len(pdfAttachment.Content) >= 5 {
		header := string(pdfAttachment.Content[:5])
		if header != "%PDF-" {
			t.Error("invalid PDF header")
		}
	}

	// Check size
	if pdfAttachment.Size <= 1000 {
		t.Error("PDF too small")
	}
}
```

### Images

```go
var imageAttachment *vaultsandbox.Attachment
for i := range email.Attachments {
	if strings.HasPrefix(email.Attachments[i].ContentType, "image/") {
		imageAttachment = &email.Attachments[i]
		break
	}
}

if imageAttachment != nil && imageAttachment.Content != nil {
	// Save image
	ext := getExtension(imageAttachment.ContentType)
	filename := "logo." + ext
	if err := os.WriteFile(filename, imageAttachment.Content, 0644); err != nil {
		log.Fatal(err)
	}

	// Verify PNG signature (magic bytes)
	if imageAttachment.ContentType == "image/png" && len(imageAttachment.Content) >= 8 {
		pngSignature := []byte{137, 80, 78, 71, 13, 10, 26, 10}
		if !bytes.Equal(imageAttachment.Content[:8], pngSignature) {
			t.Error("invalid PNG signature")
		}
	}
}

func getExtension(contentType string) string {
	extensions := map[string]string{
		"image/png":  "png",
		"image/jpeg": "jpg",
		"image/gif":  "gif",
		"image/webp": "webp",
	}
	if ext, ok := extensions[contentType]; ok {
		return ext
	}
	return "bin"
}
```

### ZIP Archives

```go
import "archive/zip"

var zipAttachment *vaultsandbox.Attachment
for i := range email.Attachments {
	if email.Attachments[i].ContentType == "application/zip" {
		zipAttachment = &email.Attachments[i]
		break
	}
}

if zipAttachment != nil && zipAttachment.Content != nil {
	reader := bytes.NewReader(zipAttachment.Content)
	zipReader, err := zip.NewReader(reader, int64(len(zipAttachment.Content)))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("ZIP contains %d files:\n", len(zipReader.File))
	for _, f := range zipReader.File {
		fmt.Printf("  - %s\n", f.Name)
	}

	// Extract specific file
	for _, f := range zipReader.File {
		if f.Name == "README.md" {
			rc, err := f.Open()
			if err != nil {
				log.Fatal(err)
			}
			defer rc.Close()

			content, err := io.ReadAll(rc)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("README:", string(content))
			break
		}
	}
}
```

## Finding Attachments

### By Filename

```go
var invoice *vaultsandbox.Attachment
for i := range email.Attachments {
	if email.Attachments[i].Filename == "invoice.pdf" {
		invoice = &email.Attachments[i]
		break
	}
}

if invoice == nil {
	t.Error("invoice attachment not found")
}
```

### By Extension

```go
var pdfs []vaultsandbox.Attachment
for _, att := range email.Attachments {
	if strings.HasSuffix(att.Filename, ".pdf") {
		pdfs = append(pdfs, att)
	}
}

fmt.Printf("Found %d PDF attachments\n", len(pdfs))
```

### By Content Type

```go
var images []vaultsandbox.Attachment
for _, att := range email.Attachments {
	if strings.HasPrefix(att.ContentType, "image/") {
		images = append(images, att)
	}
}

var documents []vaultsandbox.Attachment
docTypes := map[string]bool{
	"application/pdf":   true,
	"application/msword": true,
}
for _, att := range email.Attachments {
	if docTypes[att.ContentType] {
		documents = append(documents, att)
	}
}
```

### By Size

```go
var largeAttachments []vaultsandbox.Attachment
for _, att := range email.Attachments {
	if att.Size > 1024*1024 { // Larger than 1MB
		largeAttachments = append(largeAttachments, att)
	}
}

var smallAttachments []vaultsandbox.Attachment
for _, att := range email.Attachments {
	if att.Size < 10*1024 { // Smaller than 10KB
		smallAttachments = append(smallAttachments, att)
	}
}
```

## Saving Attachments

### Save to Disk

```go
func saveAttachment(attachment vaultsandbox.Attachment, directory string) (string, error) {
	if directory == "" {
		directory = "./downloads"
	}

	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", err
	}

	filePath := filepath.Join(directory, attachment.Filename)
	if err := os.WriteFile(filePath, attachment.Content, 0644); err != nil {
		return "", err
	}

	fmt.Println("Saved:", filePath)
	return filePath, nil
}

// Usage
for _, att := range email.Attachments {
	if _, err := saveAttachment(att, ""); err != nil {
		log.Printf("failed to save %s: %v", att.Filename, err)
	}
}
```

### Save with Validation

```go
func saveAttachmentSafely(attachment vaultsandbox.Attachment, directory string) (string, error) {
	if directory == "" {
		directory = "./downloads"
	}

	// Validate filename (prevent directory traversal)
	safeName := filepath.Base(attachment.Filename)
	if safeName != attachment.Filename {
		return "", fmt.Errorf("invalid filename: %s", attachment.Filename)
	}

	// Check size
	if attachment.Size > 10*1024*1024 { // 10MB limit
		return "", fmt.Errorf("attachment too large: %d bytes", attachment.Size)
	}

	// Create directory if needed
	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", err
	}

	// Save
	filePath := filepath.Join(directory, safeName)
	if err := os.WriteFile(filePath, attachment.Content, 0644); err != nil {
		return "", err
	}

	return filePath, nil
}
```

## Validating Attachments

### Check Presence

```go
func TestEmailIncludesInvoicePDF(t *testing.T) {
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Invoice`)),
		vaultsandbox.WithWaitTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(email.Attachments) == 0 {
		t.Fatal("expected at least one attachment")
	}

	var pdf *vaultsandbox.Attachment
	invoicePattern := regexp.MustCompile(`(?i)invoice.*\.pdf`)
	for i := range email.Attachments {
		if invoicePattern.MatchString(email.Attachments[i].Filename) {
			pdf = &email.Attachments[i]
			break
		}
	}

	if pdf == nil {
		t.Fatal("invoice PDF not found")
	}
}
```

### Validate Content Type

```go
attachment := email.Attachments[0]

if attachment.ContentType != "application/pdf" {
	t.Errorf("expected application/pdf, got %s", attachment.ContentType)
}

// Or more flexible
validTypes := []string{"application/pdf", "application/x-pdf"}
valid := false
for _, vt := range validTypes {
	if attachment.ContentType == vt {
		valid = true
		break
	}
}
if !valid {
	t.Errorf("unexpected content type: %s", attachment.ContentType)
}
```

### Validate Size

```go
attachment := email.Attachments[0]

// Not empty
if attachment.Size == 0 {
	t.Error("attachment is empty")
}

// Within expected range
if attachment.Size < 1000 {
	t.Error("attachment too small (less than 1KB)")
}
if attachment.Size > 5*1024*1024 {
	t.Error("attachment too large (more than 5MB)")
}

// Exact size (if known)
if attachment.Size != expectedSize {
	t.Errorf("expected size %d, got %d", expectedSize, attachment.Size)
}
```

### Validate Content

```go
attachment := email.Attachments[0]

if attachment.Content != nil {
	// Check content length matches size
	if len(attachment.Content) != attachment.Size {
		t.Errorf("content length %d doesn't match size %d",
			len(attachment.Content), attachment.Size)
	}

	// For text files
	text := string(attachment.Content)
	if !strings.Contains(text, "expected content") {
		t.Error("expected content not found")
	}

	// For binary files (check PDF signature)
	if len(attachment.Content) >= 4 {
		signature := attachment.Content[:4]
		pdfSig := []byte{0x25, 0x50, 0x44, 0x46} // %PDF
		if !bytes.Equal(signature, pdfSig) {
			t.Error("invalid PDF signature")
		}
	}
}
```

## Testing Patterns

### Test with Attachments

```go
func TestEmailWithAttachments(t *testing.T) {
	ctx := context.Background()

	client, err := vaultsandbox.New(apiKey)
	if err != nil {
		t.Fatal(err)
	}

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	t.Run("receives invoice with PDF", func(t *testing.T) {
		// Trigger your application to send an invoice
		err := app.SendInvoice(SendInvoiceRequest{
			To:      inbox.EmailAddress(),
			OrderID: "12345",
		})
		if err != nil {
			t.Fatal(err)
		}

		email, err := inbox.WaitForEmail(ctx,
			vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Invoice`)),
			vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
				return len(e.Attachments) > 0
			}),
			vaultsandbox.WithWaitTimeout(10*time.Second),
		)
		if err != nil {
			t.Fatal(err)
		}

		// Find PDF attachment
		var pdf *vaultsandbox.Attachment
		for i := range email.Attachments {
			if email.Attachments[i].ContentType == "application/pdf" {
				pdf = &email.Attachments[i]
				break
			}
		}

		if pdf == nil {
			t.Fatal("PDF attachment not found")
		}

		invoicePattern := regexp.MustCompile(`(?i)invoice.*\.pdf`)
		if !invoicePattern.MatchString(pdf.Filename) {
			t.Errorf("unexpected filename: %s", pdf.Filename)
		}

		if pdf.Size < 1000 {
			t.Error("PDF too small")
		}

		// Validate PDF content
		if pdf.Content != nil && len(pdf.Content) >= 5 {
			header := string(pdf.Content[:5])
			if header != "%PDF-" {
				t.Error("invalid PDF header")
			}
		}
	})

	t.Run("receives report with multiple attachments", func(t *testing.T) {
		err := app.SendReport(inbox.EmailAddress())
		if err != nil {
			t.Fatal(err)
		}

		email, err := inbox.WaitForEmail(ctx,
			vaultsandbox.WithWaitTimeout(10*time.Second),
		)
		if err != nil {
			t.Fatal(err)
		}

		if len(email.Attachments) != 3 {
			t.Errorf("expected 3 attachments, got %d", len(email.Attachments))
		}

		// Verify each attachment type
		var csv, pdf, jsonAtt *vaultsandbox.Attachment
		for i := range email.Attachments {
			att := &email.Attachments[i]
			switch {
			case strings.HasSuffix(att.Filename, ".csv"):
				csv = att
			case strings.HasSuffix(att.Filename, ".pdf"):
				pdf = att
			case strings.HasSuffix(att.Filename, ".json"):
				jsonAtt = att
			}
		}

		if csv == nil {
			t.Error("CSV attachment not found")
		}
		if pdf == nil {
			t.Error("PDF attachment not found")
		}
		if jsonAtt == nil {
			t.Error("JSON attachment not found")
		}
	})
}
```

### Process Attachment Content

```go
func TestProcessCSVAttachment(t *testing.T) {
	err := app.SendReport(inbox.EmailAddress())
	if err != nil {
		t.Fatal(err)
	}

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}

	var csv *vaultsandbox.Attachment
	for i := range email.Attachments {
		if strings.HasSuffix(email.Attachments[i].Filename, ".csv") {
			csv = &email.Attachments[i]
			break
		}
	}

	if csv == nil {
		t.Fatal("CSV attachment not found")
	}

	if csv.Content != nil {
		text := string(csv.Content)
		lines := strings.Split(strings.TrimSpace(text), "\n")
		headers := strings.Split(lines[0], ",")

		found := false
		for _, h := range headers {
			if h == "Name" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Name header not found")
		}

		found = false
		for _, h := range headers {
			if h == "Email" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Email header not found")
		}

		if len(lines) <= 1 {
			t.Error("CSV has no data rows")
		}
	}
}
```

## Advanced Patterns

### Extract and Process All Attachments

```go
type ProcessedAttachment struct {
	Type     string
	Filename string
	Size     int
	Data     interface{}
}

func processAttachments(email *vaultsandbox.Email) ([]ProcessedAttachment, error) {
	results := make([]ProcessedAttachment, 0, len(email.Attachments))

	for _, att := range email.Attachments {
		var result ProcessedAttachment
		var err error

		switch {
		case strings.HasPrefix(att.ContentType, "image/"):
			result, err = processImage(att)
		case att.ContentType == "application/pdf":
			result, err = processPDF(att)
		case strings.Contains(att.ContentType, "text"):
			result, err = processText(att)
		default:
			result = ProcessedAttachment{
				Type:     "unknown",
				Filename: att.Filename,
				Size:     att.Size,
			}
		}

		if err != nil {
			return nil, fmt.Errorf("processing %s: %w", att.Filename, err)
		}
		results = append(results, result)
	}

	return results, nil
}

func processImage(att vaultsandbox.Attachment) (ProcessedAttachment, error) {
	return ProcessedAttachment{
		Type:     "image",
		Filename: att.Filename,
		Size:     att.Size,
	}, nil
}

func processPDF(att vaultsandbox.Attachment) (ProcessedAttachment, error) {
	return ProcessedAttachment{
		Type:     "pdf",
		Filename: att.Filename,
		Size:     att.Size,
	}, nil
}

func processText(att vaultsandbox.Attachment) (ProcessedAttachment, error) {
	return ProcessedAttachment{
		Type:     "text",
		Filename: att.Filename,
		Size:     att.Size,
		Data:     string(att.Content),
	}, nil
}
```

### Attachment Metadata Collection

```go
type AttachmentMetadata struct {
	Filename    string
	ContentType string
	Size        int
	SizeHuman   string
	HasContent  bool
	IsInline    bool
}

func collectAttachmentMetadata(email *vaultsandbox.Email) []AttachmentMetadata {
	metadata := make([]AttachmentMetadata, len(email.Attachments))

	for i, att := range email.Attachments {
		metadata[i] = AttachmentMetadata{
			Filename:    att.Filename,
			ContentType: att.ContentType,
			Size:        att.Size,
			SizeHuman:   formatBytes(att.Size),
			HasContent:  att.Content != nil,
			IsInline:    att.ContentDisposition == "inline",
		}
	}

	return metadata
}

func formatBytes(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}

// Usage
metadata := collectAttachmentMetadata(email)
for _, m := range metadata {
	fmt.Printf("%s: %s (%s)\n", m.Filename, m.ContentType, m.SizeHuman)
}
```

## Troubleshooting

### No Attachments Found

```go
if len(email.Attachments) == 0 {
	fmt.Println("No attachments in email")
	fmt.Println("Subject:", email.Subject)
	fmt.Println("From:", email.From)

	// Check if attachments mentioned in body
	if strings.Contains(email.Text, "attach") {
		fmt.Println("Warning: Email mentions attachments but none found")
	}
}
```

### Attachment Content Missing

```go
attachment := email.Attachments[0]

if attachment.Content == nil {
	fmt.Println("Attachment content is nil")
	fmt.Println("Filename:", attachment.Filename)
	fmt.Println("Size:", attachment.Size)
	fmt.Println("This may indicate:")
	fmt.Println("- Attachment failed to download")
	fmt.Println("- Attachment was too large")
	fmt.Println("- Decryption error")
}
```

### Invalid File Format

```go
var data map[string]interface{}
if err := json.Unmarshal(attachment.Content, &data); err != nil {
	fmt.Println("Failed to parse JSON attachment")
	fmt.Println("Filename:", attachment.Filename)
	if len(attachment.Content) > 100 {
		fmt.Println("Content preview:", string(attachment.Content[:100]))
	} else {
		fmt.Println("Content preview:", string(attachment.Content))
	}
}
```

### Verify Checksum

```go
import "crypto/sha256"

func verifyChecksum(attachment vaultsandbox.Attachment) bool {
	if attachment.Checksum == "" || attachment.Content == nil {
		return false
	}

	hash := sha256.Sum256(attachment.Content)
	computed := fmt.Sprintf("%x", hash)

	return computed == attachment.Checksum
}

// Usage
if !verifyChecksum(attachment) {
	fmt.Println("Warning: Attachment checksum mismatch")
}
```

## Next Steps

- **[Email Objects](/client-go/concepts/emails/)** - Understanding email structure
- **[Managing Inboxes](/client-go/guides/managing-inboxes/)** - Inbox operations
- **[Testing Patterns](/client-go/testing/password-reset/)** - Real-world examples
- **[API Reference: Email](/client-go/api/email/)** - Complete API documentation
