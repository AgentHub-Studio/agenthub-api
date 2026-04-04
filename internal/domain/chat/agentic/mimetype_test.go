package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- ExtensionForMIMEType ---

func TestExtensionForMIMEType_Empty(t *testing.T) {
	assert.Equal(t, "bin", agentic.ExtensionForMIMEType(""))
}

func TestExtensionForMIMEType_PDF(t *testing.T) {
	assert.Equal(t, "pdf", agentic.ExtensionForMIMEType("application/pdf"))
}

func TestExtensionForMIMEType_JSON(t *testing.T) {
	assert.Equal(t, "json", agentic.ExtensionForMIMEType("application/json"))
}

func TestExtensionForMIMEType_PlainText(t *testing.T) {
	assert.Equal(t, "txt", agentic.ExtensionForMIMEType("text/plain"))
}

func TestExtensionForMIMEType_HTML(t *testing.T) {
	assert.Equal(t, "html", agentic.ExtensionForMIMEType("text/html"))
}

func TestExtensionForMIMEType_CSV(t *testing.T) {
	assert.Equal(t, "csv", agentic.ExtensionForMIMEType("text/csv"))
}

func TestExtensionForMIMEType_WithCharset(t *testing.T) {
	assert.Equal(t, "json", agentic.ExtensionForMIMEType("application/json; charset=utf-8"))
}

func TestExtensionForMIMEType_CaseInsensitive(t *testing.T) {
	assert.Equal(t, "png", agentic.ExtensionForMIMEType("Image/PNG"))
}

func TestExtensionForMIMEType_Unknown(t *testing.T) {
	assert.Equal(t, "bin", agentic.ExtensionForMIMEType("application/x-custom"))
}

func TestExtensionForMIMEType_Images(t *testing.T) {
	assert.Equal(t, "png", agentic.ExtensionForMIMEType("image/png"))
	assert.Equal(t, "jpg", agentic.ExtensionForMIMEType("image/jpeg"))
	assert.Equal(t, "gif", agentic.ExtensionForMIMEType("image/gif"))
	assert.Equal(t, "webp", agentic.ExtensionForMIMEType("image/webp"))
	assert.Equal(t, "svg", agentic.ExtensionForMIMEType("image/svg+xml"))
}

func TestExtensionForMIMEType_Office(t *testing.T) {
	assert.Equal(t, "docx", agentic.ExtensionForMIMEType("application/vnd.openxmlformats-officedocument.wordprocessingml.document"))
	assert.Equal(t, "xlsx", agentic.ExtensionForMIMEType("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"))
	assert.Equal(t, "doc", agentic.ExtensionForMIMEType("application/msword"))
}

func TestExtensionForMIMEType_Audio(t *testing.T) {
	assert.Equal(t, "mp3", agentic.ExtensionForMIMEType("audio/mpeg"))
	assert.Equal(t, "wav", agentic.ExtensionForMIMEType("audio/wav"))
	assert.Equal(t, "ogg", agentic.ExtensionForMIMEType("audio/ogg"))
}

func TestExtensionForMIMEType_Video(t *testing.T) {
	assert.Equal(t, "mp4", agentic.ExtensionForMIMEType("video/mp4"))
	assert.Equal(t, "webm", agentic.ExtensionForMIMEType("video/webm"))
}

func TestExtensionForMIMEType_XML(t *testing.T) {
	assert.Equal(t, "xml", agentic.ExtensionForMIMEType("application/xml"))
	assert.Equal(t, "xml", agentic.ExtensionForMIMEType("text/xml"))
}

// --- MIMETypeForExtension ---

func TestMIMETypeForExtension_Known(t *testing.T) {
	assert.Equal(t, "application/pdf", agentic.MIMETypeForExtension("pdf"))
	assert.Equal(t, "application/json", agentic.MIMETypeForExtension("json"))
	assert.Equal(t, "text/plain", agentic.MIMETypeForExtension("txt"))
	assert.Equal(t, "image/png", agentic.MIMETypeForExtension("png"))
}

func TestMIMETypeForExtension_JpegVariants(t *testing.T) {
	assert.Equal(t, "image/jpeg", agentic.MIMETypeForExtension("jpg"))
	assert.Equal(t, "image/jpeg", agentic.MIMETypeForExtension("jpeg"))
}

func TestMIMETypeForExtension_CaseInsensitive(t *testing.T) {
	assert.Equal(t, "application/pdf", agentic.MIMETypeForExtension("PDF"))
}

func TestMIMETypeForExtension_Unknown(t *testing.T) {
	assert.Equal(t, "application/octet-stream", agentic.MIMETypeForExtension("xyz"))
}

func TestMIMETypeForExtension_YAML(t *testing.T) {
	assert.Equal(t, "application/yaml", agentic.MIMETypeForExtension("yaml"))
	assert.Equal(t, "application/yaml", agentic.MIMETypeForExtension("yml"))
}

// --- LargeOutputInstructions ---

func TestLargeOutputInstructions_Basic(t *testing.T) {
	result := agentic.LargeOutputInstructions("/tmp/output.json", 50000, "JSON", 0)
	assert.Contains(t, result, "50000 characters")
	assert.Contains(t, result, "/tmp/output.json")
	assert.Contains(t, result, "JSON")
	assert.Contains(t, result, "sequential chunks")
}

func TestLargeOutputInstructions_WithMaxReadLength(t *testing.T) {
	result := agentic.LargeOutputInstructions("/tmp/out.txt", 100000, "plain text", 10000)
	assert.Contains(t, result, "limited to 10000 chars")
}

func TestLargeOutputInstructions_WithoutMaxReadLength(t *testing.T) {
	result := agentic.LargeOutputInstructions("/tmp/out.txt", 100000, "plain text", 0)
	assert.NotContains(t, result, "limited to")
	assert.Contains(t, result, "reduce the chunk size")
}

func TestLargeOutputInstructions_CompletionRequirement(t *testing.T) {
	result := agentic.LargeOutputInstructions("/tmp/out.txt", 1000, "text", 0)
	assert.Contains(t, result, "MUST explicitly describe")
}

// --- Roundtrip ---

func TestMIME_Roundtrip(t *testing.T) {
	types := map[string]string{
		"pdf":  "application/pdf",
		"json": "application/json",
		"txt":  "text/plain",
		"html": "text/html",
		"png":  "image/png",
		"svg":  "image/svg+xml",
	}
	for ext, mime := range types {
		assert.Equal(t, ext, agentic.ExtensionForMIMEType(mime), "MIME→ext for %s", mime)
		assert.Equal(t, mime, agentic.MIMETypeForExtension(ext), "ext→MIME for %s", ext)
	}
}
