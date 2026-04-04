package agentic

import (
	"fmt"
	"strings"
)

// MIME type to file extension mapping and large output instruction generator.
//
// Inspired by Claude Code's toolResultStorage.ts — maps MIME types to
// file extensions for tool result persistence, and generates structured
// instructions for the LLM when tool results exceed the context budget.
// Prevents silent truncation by telling the LLM how to read persisted
// output in chunks.

// ExtensionForMIMEType returns the file extension (without dot) for a
// MIME type. Unknown types return "bin". Strips charset/boundary parameters.
func ExtensionForMIMEType(mimeType string) string {
	if mimeType == "" {
		return "bin"
	}
	// Strip any charset/boundary parameter
	mt := strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])
	mt = strings.ToLower(mt)

	switch mt {
	case "application/pdf":
		return "pdf"
	case "application/json":
		return "json"
	case "text/csv":
		return "csv"
	case "text/plain":
		return "txt"
	case "text/html":
		return "html"
	case "text/markdown":
		return "md"
	case "application/zip":
		return "zip"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return "xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return "pptx"
	case "application/msword":
		return "doc"
	case "application/vnd.ms-excel":
		return "xls"
	case "audio/mpeg":
		return "mp3"
	case "audio/wav":
		return "wav"
	case "audio/ogg":
		return "ogg"
	case "video/mp4":
		return "mp4"
	case "video/webm":
		return "webm"
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/svg+xml":
		return "svg"
	case "application/xml", "text/xml":
		return "xml"
	case "application/yaml", "text/yaml":
		return "yaml"
	default:
		return "bin"
	}
}

// MIMETypeForExtension returns the MIME type for a file extension (without dot).
func MIMETypeForExtension(ext string) string {
	switch strings.ToLower(ext) {
	case "pdf":
		return "application/pdf"
	case "json":
		return "application/json"
	case "csv":
		return "text/csv"
	case "txt":
		return "text/plain"
	case "html":
		return "text/html"
	case "md":
		return "text/markdown"
	case "zip":
		return "application/zip"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "svg":
		return "image/svg+xml"
	case "xml":
		return "application/xml"
	case "yaml", "yml":
		return "application/yaml"
	default:
		return "application/octet-stream"
	}
}

// LargeOutputInstructions generates instruction text for the LLM when
// a tool result exceeds the token budget. The instructions tell the
// LLM how to read the persisted output file in chunks.
func LargeOutputInstructions(outputPath string, contentLength int, formatDesc string, maxReadLength int) string {
	base := fmt.Sprintf(
		"Error: result (%d characters) exceeds maximum allowed tokens. Output has been saved to %s.\n"+
			"Format: %s\n"+
			"Use offset and limit parameters to read specific portions of the file, search within it for specific content.\n"+
			"REQUIREMENTS FOR SUMMARIZATION/ANALYSIS/REVIEW:\n"+
			"- You MUST read the content from the file at %s in sequential chunks until 100%% of the content has been read.\n",
		contentLength, outputPath, formatDesc, outputPath,
	)

	var truncationWarning string
	if maxReadLength > 0 {
		truncationWarning = fmt.Sprintf(
			"- If you receive truncation warnings when reading the file, reduce the chunk size until you have read 100%% of the content without truncation. Output is limited to %d chars.\n",
			maxReadLength,
		)
	} else {
		truncationWarning = "- If you receive truncation warnings when reading the file, reduce the chunk size until you have read 100% of the content without truncation.\n"
	}

	completion := "- Before producing ANY summary or analysis, you MUST explicitly describe what portion of the content you have read. If you did not read the entire content, you MUST explicitly state this.\n"

	return base + truncationWarning + completion
}
