---
title: Working with Attachments
description: Access, decode, and validate email attachments
---

VaultSandbox automatically decrypts email attachments and provides them as `Uint8Array` buffers ready to process.

## Accessing Attachments

### Basic Access

```javascript
const email = await inbox.waitForEmail({
	subject: /Invoice/,
	timeout: 10000,
});

console.log(`Email has ${email.attachments.length} attachments`);

email.attachments.forEach((att) => {
	console.log(`- ${att.filename} (${att.contentType}, ${att.size} bytes)`);
});
```

## Attachment Structure

### AttachmentData Properties

```javascript
const attachment = email.attachments[0];

console.log(attachment.filename); // "invoice.pdf"
console.log(attachment.contentType); // "application/pdf"
console.log(attachment.size); // 15234 (bytes)
console.log(attachment.content); // Uint8Array
console.log(attachment.contentId); // "part123@example.com" (optional)
console.log(attachment.contentDisposition); // "attachment" or "inline"
```

## Working with Different File Types

### Text Files

```javascript
const txtAttachment = email.attachments.find((att) => att.contentType.includes('text'));

if (txtAttachment?.content) {
	const text = new TextDecoder().decode(txtAttachment.content);
	console.log('Text content:', text);

	// Validate content
	expect(text).toContain('expected text');
}
```

### JSON Files

```javascript
const jsonAttachment = email.attachments.find(
	(att) => att.contentType.includes('json') || att.filename.endsWith('.json')
);

if (jsonAttachment?.content) {
	const jsonText = new TextDecoder().decode(jsonAttachment.content);
	const data = JSON.parse(jsonText);

	console.log('Parsed JSON:', data);
	expect(data.status).toBe('success');
}
```

### CSV Files

```javascript
const csvAttachment = email.attachments.find((att) => att.filename.endsWith('.csv'));

if (csvAttachment?.content) {
	const csvText = new TextDecoder().decode(csvAttachment.content);
	const lines = csvText.split('\n');

	console.log(`CSV has ${lines.length} lines`);
	expect(lines[0]).toContain('Name,Email,Status');
}
```

### PDF Files

```javascript
import fs from 'fs';

const pdfAttachment = email.attachments.find((att) => att.contentType === 'application/pdf');

if (pdfAttachment?.content) {
	// Save to disk
	fs.writeFileSync('invoice.pdf', pdfAttachment.content);

	// Verify PDF signature
	const header = new TextDecoder().decode(pdfAttachment.content.slice(0, 5));
	expect(header).toBe('%PDF-');

	// Check size
	expect(pdfAttachment.size).toBeGreaterThan(1000);
}
```

### Images

```javascript
import fs from 'fs';

const imageAttachment = email.attachments.find((att) => att.contentType.startsWith('image/'));

if (imageAttachment?.content) {
	// Save image
	fs.writeFileSync(`logo.${getExtension(imageAttachment.contentType)}`, imageAttachment.content);

	// Verify it's a valid image (check magic bytes)
	if (imageAttachment.contentType === 'image/png') {
		const pngSignature = Array.from(imageAttachment.content.slice(0, 8));
		expect(pngSignature).toEqual([137, 80, 78, 71, 13, 10, 26, 10]);
	}
}

function getExtension(contentType) {
	const map = {
		'image/png': 'png',
		'image/jpeg': 'jpg',
		'image/gif': 'gif',
		'image/webp': 'webp',
	};
	return map[contentType] || 'bin';
}
```

### ZIP Archives

```javascript
import AdmZip from 'adm-zip';

const zipAttachment = email.attachments.find((att) => att.contentType === 'application/zip');

if (zipAttachment?.content) {
	const zip = new AdmZip(Buffer.from(zipAttachment.content));
	const entries = zip.getEntries();

	console.log(`ZIP contains ${entries.length} files:`);
	entries.forEach((entry) => {
		console.log(`  - ${entry.entryName}`);
	});

	// Extract specific file
	const readme = zip.getEntry('README.md');
	if (readme) {
		const content = readme.getData().toString('utf8');
		console.log('README:', content);
	}
}
```

## Finding Attachments

### By Filename

```javascript
const invoice = email.attachments.find((att) => att.filename === 'invoice.pdf');

expect(invoice).toBeDefined();
```

### By Extension

```javascript
const pdfs = email.attachments.filter((att) => att.filename.endsWith('.pdf'));

console.log(`Found ${pdfs.length} PDF attachments`);
```

### By Content Type

```javascript
const images = email.attachments.filter((att) => att.contentType.startsWith('image/'));

const documents = email.attachments.filter((att) =>
	['application/pdf', 'application/msword'].includes(att.contentType)
);
```

### By Size

```javascript
const largeAttachments = email.attachments.filter(
	(att) => att.size > 1024 * 1024 // Larger than 1MB
);

const smallAttachments = email.attachments.filter(
	(att) => att.size < 10 * 1024 // Smaller than 10KB
);
```

## Saving Attachments

### Save to Disk

```javascript
import fs from 'fs';
import path from 'path';

function saveAttachment(attachment, directory = './downloads') {
	if (!fs.existsSync(directory)) {
		fs.mkdirSync(directory, { recursive: true });
	}

	const filePath = path.join(directory, attachment.filename);
	fs.writeFileSync(filePath, attachment.content);

	console.log(`Saved: ${filePath}`);
	return filePath;
}

// Usage
email.attachments.forEach((att) => {
	saveAttachment(att);
});
```

### Save with Validation

```javascript
function saveAttachmentSafely(attachment, directory = './downloads') {
	// Validate filename (prevent directory traversal)
	const safeName = path.basename(attachment.filename);
	if (safeName !== attachment.filename) {
		throw new Error('Invalid filename');
	}

	// Check size
	if (attachment.size > 10 * 1024 * 1024) {
		// 10MB limit
		throw new Error('Attachment too large');
	}

	// Save
	const filePath = path.join(directory, safeName);
	fs.writeFileSync(filePath, attachment.content);

	return filePath;
}
```

## Validating Attachments

### Check Presence

```javascript
test('email includes invoice PDF', async () => {
	const email = await inbox.waitForEmail({
		subject: /Invoice/,
		timeout: 10000,
	});

	expect(email.attachments.length).toBeGreaterThan(0);

	const pdf = email.attachments.find((att) => att.filename.match(/invoice.*\.pdf/i));

	expect(pdf).toBeDefined();
});
```

### Validate Content Type

```javascript
const attachment = email.attachments[0];

expect(attachment.contentType).toBe('application/pdf');

// Or more flexible
expect(['application/pdf', 'application/x-pdf']).toContain(attachment.contentType);
```

### Validate Size

```javascript
const attachment = email.attachments[0];

// Not empty
expect(attachment.size).toBeGreaterThan(0);

// Within expected range
expect(attachment.size).toBeGreaterThan(1000); // At least 1KB
expect(attachment.size).toBeLessThan(5 * 1024 * 1024); // Less than 5MB

// Exact size (if known)
expect(attachment.size).toBe(expectedSize);
```

### Validate Content

```javascript
const attachment = email.attachments[0];

if (attachment.content) {
	// Check content exists
	expect(attachment.content.length).toBe(attachment.size);

	// For text files
	const text = new TextDecoder().decode(attachment.content);
	expect(text).toContain('expected content');

	// For binary files (check signature)
	const signature = Array.from(attachment.content.slice(0, 4));
	expect(signature).toEqual([0x25, 0x50, 0x44, 0x46]); // PDF signature
}
```

## Testing Patterns

### Test with Attachments

```javascript
describe('Email with Attachments', () => {
	let inbox;

	beforeEach(async () => {
		inbox = await client.createInbox();
	});

	afterEach(async () => {
		await inbox.delete();
	});

	test('receives invoice with PDF', async () => {
		await app.sendInvoice({
			to: inbox.emailAddress,
			orderId: '12345',
		});

		const email = await inbox.waitForEmail({
			subject: /Invoice/,
			predicate: (e) => e.attachments.length > 0,
			timeout: 10000,
		});

		// Validate attachment
		const pdf = email.attachments.find((att) => att.contentType === 'application/pdf');

		expect(pdf).toBeDefined();
		expect(pdf.filename).toMatch(/invoice.*\.pdf/i);
		expect(pdf.size).toBeGreaterThan(1000);

		// Validate PDF content
		if (pdf.content) {
			const header = new TextDecoder().decode(pdf.content.slice(0, 5));
			expect(header).toBe('%PDF-');
		}
	});

	test('receives report with multiple attachments', async () => {
		await app.sendReport(inbox.emailAddress);

		const email = await inbox.waitForEmail({ timeout: 10000 });

		expect(email.attachments.length).toBe(3);

		// Verify each attachment
		const csv = email.attachments.find((att) => att.filename.endsWith('.csv'));
		const pdf = email.attachments.find((att) => att.filename.endsWith('.pdf'));
		const json = email.attachments.find((att) => att.filename.endsWith('.json'));

		expect(csv).toBeDefined();
		expect(pdf).toBeDefined();
		expect(json).toBeDefined();
	});
});
```

### Process Attachment Content

```javascript
test('processes CSV attachment', async () => {
	await app.sendReport(inbox.emailAddress);

	const email = await inbox.waitForEmail({ timeout: 10000 });
	const csv = email.attachments.find((att) => att.filename.endsWith('.csv'));

	expect(csv).toBeDefined();

	if (csv?.content) {
		const text = new TextDecoder().decode(csv.content);
		const lines = text.trim().split('\n');
		const headers = lines[0].split(',');

		expect(headers).toContain('Name');
		expect(headers).toContain('Email');
		expect(lines.length).toBeGreaterThan(1); // Has data rows
	}
});
```

## Advanced Patterns

### Extract and Process All Attachments

```javascript
async function processAttachments(email) {
	const results = await Promise.all(
		email.attachments.map(async (att) => {
			if (att.contentType.startsWith('image/')) {
				return processImage(att);
			} else if (att.contentType === 'application/pdf') {
				return processPDF(att);
			} else if (att.contentType.includes('text')) {
				return processText(att);
			} else {
				return { type: 'unknown', filename: att.filename };
			}
		})
	);

	return results;
}

async function processImage(att) {
	// Image processing logic
	return {
		type: 'image',
		filename: att.filename,
		size: att.size,
		dimensions: await getImageDimensions(att.content),
	};
}
```

### Attachment Metadata Collection

```javascript
function collectAttachmentMetadata(email) {
	return email.attachments.map((att) => ({
		filename: att.filename,
		contentType: att.contentType,
		size: att.size,
		sizeHuman: formatBytes(att.size),
		hasContent: att.content !== undefined,
		isInline: att.contentDisposition === 'inline',
	}));
}

function formatBytes(bytes) {
	if (bytes < 1024) return bytes + ' B';
	if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
	return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

// Usage
const metadata = collectAttachmentMetadata(email);
console.table(metadata);
```

## Troubleshooting

### No Attachments Found

```javascript
if (email.attachments.length === 0) {
	console.log('No attachments in email');
	console.log('Subject:', email.subject);
	console.log('From:', email.from);

	// Check if attachments mentioned in body
	if (email.text?.includes('attach')) {
		console.warn('Email mentions attachments but none found');
	}
}
```

### Attachment Content Missing

```javascript
const attachment = email.attachments[0];

if (!attachment.content) {
	console.error('Attachment content is undefined');
	console.log('Filename:', attachment.filename);
	console.log('Size:', attachment.size);
	console.log('This may indicate:');
	console.log('- Attachment failed to download');
	console.log('- Attachment was too large');
	console.log('- Decryption error');
}
```

### Invalid File Format

```javascript
try {
	const json = JSON.parse(new TextDecoder().decode(attachment.content));
} catch (error) {
	console.error('Failed to parse JSON attachment');
	console.log('Filename:', attachment.filename);
	console.log('Content preview:', new TextDecoder().decode(attachment.content.slice(0, 100)));
}
```

## Next Steps

- **[Email Objects](/client-node/concepts/emails/)** - Understanding email structure
- **[Managing Inboxes](/client-node/guides/managing-inboxes/)** - Inbox operations
- **[Testing Patterns](/client-node/testing/password-reset/)** - Real-world examples
- **[API Reference: Email](/client-node/api/email/)** - Complete API documentation
