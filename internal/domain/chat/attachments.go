package chat

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaxAttachmentSizeBytes   int64 = 10 * 1024 * 1024
	MaxAttachmentsPerMessage       = 5
	maxAttachmentTextBytes         = 64 * 1024
)

type AttachmentKind string

const (
	AttachmentKindImage AttachmentKind = "image"
	AttachmentKindPDF   AttachmentKind = "pdf"
	AttachmentKindText  AttachmentKind = "text"
	AttachmentKindCode  AttachmentKind = "code"
)

type ChatAttachment struct {
	ID            uuid.UUID      `json:"id"`
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	Size          int64          `json:"size"`
	URL           string         `json:"url"`
	Kind          AttachmentKind `json:"kind"`
	StorageKey    string         `json:"storageKey,omitempty"`
	Text          string         `json:"text,omitempty"`
	ExtractedText string         `json:"extractedText,omitempty"`
	Truncated     bool           `json:"truncated,omitempty"`
}

func NormalizeChatAttachments(raw json.RawMessage) (json.RawMessage, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, nil
	}

	attachments, err := DecodeChatAttachments(raw)
	if err != nil {
		return nil, err
	}
	if len(attachments) == 0 {
		return json.RawMessage("[]"), nil
	}
	if len(attachments) > MaxAttachmentsPerMessage {
		return nil, fmt.Errorf("chat attachment: maximum of %d attachments per message exceeded", MaxAttachmentsPerMessage)
	}
	for i, a := range attachments {
		if a.ID == uuid.Nil {
			return nil, fmt.Errorf("chat attachment: attachments[%d].id is required", i)
		}
		if strings.TrimSpace(a.Name) == "" {
			return nil, fmt.Errorf("chat attachment: attachments[%d].name is required", i)
		}
		if a.Size < 0 || a.Size > MaxAttachmentSizeBytes {
			return nil, fmt.Errorf("chat attachment: attachments[%d].size exceeds maximum of %d bytes", i, MaxAttachmentSizeBytes)
		}
		if a.Kind == "" {
			kind, err := ClassifyAttachment(a.Type, a.Name)
			if err != nil {
				return nil, fmt.Errorf("chat attachment: attachments[%d]: %w", i, err)
			}
			attachments[i].Kind = kind
		} else if !isSupportedAttachmentKind(a.Kind) {
			return nil, fmt.Errorf("chat attachment: attachments[%d].kind is unsupported", i)
		}
	}

	normalized, err := json.Marshal(attachments)
	if err != nil {
		return nil, fmt.Errorf("chat attachment: marshal: %w", err)
	}
	return normalized, nil
}

func DecodeChatAttachments(raw json.RawMessage) ([]ChatAttachment, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, nil
	}
	var attachments []ChatAttachment
	if err := json.Unmarshal(raw, &attachments); err != nil {
		return nil, fmt.Errorf("chat attachment: invalid attachments payload")
	}
	return attachments, nil
}

func HasChatAttachments(raw json.RawMessage) bool {
	attachments, err := DecodeChatAttachments(raw)
	return err == nil && len(attachments) > 0
}

func BuildUploadedChatAttachment(fileName, contentType, url, storageKey string, data []byte) (ChatAttachment, error) {
	name, err := SafeAttachmentName(fileName)
	if err != nil {
		return ChatAttachment{}, err
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	kind, err := ClassifyAttachment(contentType, name)
	if err != nil {
		return ChatAttachment{}, err
	}

	attachment := ChatAttachment{
		ID:         uuid.New(),
		Name:       name,
		Type:       normalizeContentType(contentType),
		Size:       int64(len(data)),
		URL:        url,
		Kind:       kind,
		StorageKey: storageKey,
	}
	if kind == AttachmentKindText || kind == AttachmentKindCode {
		attachment.Text, attachment.Truncated = attachmentText(data)
	}
	return attachment, nil
}

func SafeAttachmentName(fileName string) (string, error) {
	name := filepath.Base(filepath.ToSlash(fileName))
	if name == "." || name == "/" || name == ".." || name == "" || strings.ContainsAny(name, "\x00/\\") {
		return "", fmt.Errorf("chat attachment: invalid file name")
	}
	if len(name) > 255 {
		return "", fmt.Errorf("chat attachment: file name exceeds 255 chars")
	}
	return name, nil
}

func ClassifyAttachment(contentType, fileName string) (AttachmentKind, error) {
	ct := normalizeContentType(contentType)
	ext := strings.ToLower(filepath.Ext(fileName))

	switch {
	case ct == "image/png" || ct == "image/jpeg" || ct == "image/gif" || ct == "image/webp":
		return AttachmentKindImage, nil
	case ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp":
		return AttachmentKindImage, nil
	case ct == "application/pdf" || ext == ".pdf":
		return AttachmentKindPDF, nil
	case isCodeExtension(ext) || isCodeContentType(ct):
		return AttachmentKindCode, nil
	case strings.HasPrefix(ct, "text/") || isTextExtension(ext) || isTextContentType(ct):
		return AttachmentKindText, nil
	default:
		return "", fmt.Errorf("unsupported attachment type %q for %q", contentType, fileName)
	}
}

func UserMessageWithAttachmentContext(content string, raw json.RawMessage) string {
	context := AttachmentContext(raw)
	if context == "" {
		return content
	}
	if strings.TrimSpace(content) == "" {
		return context
	}
	return content + "\n\n" + context
}

func AttachmentContext(raw json.RawMessage) string {
	attachments, err := DecodeChatAttachments(raw)
	if err != nil || len(attachments) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("[Attached files]\n")
	for i, a := range attachments {
		fmt.Fprintf(&b, "%d. %s (%s, %d bytes", i+1, a.Name, a.Type, a.Size)
		if a.URL != "" {
			fmt.Fprintf(&b, ", url: %s", a.URL)
		}
		b.WriteString(")\n")

		text := strings.TrimSpace(a.Text)
		if text == "" {
			text = strings.TrimSpace(a.ExtractedText)
		}
		switch {
		case text != "":
			if a.Truncated {
				b.WriteString("Content preview (truncated):\n")
			} else {
				b.WriteString("Content:\n")
			}
			b.WriteString(text)
			b.WriteString("\n")
		case a.Kind == AttachmentKindImage:
			b.WriteString("Image attachment available from object storage. Use the file metadata as context if multimodal input is unavailable.\n")
		case a.Kind == AttachmentKindPDF:
			b.WriteString("PDF attachment available from object storage. Extracted text was not provided in this message payload.\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func normalizeContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	mt, _, err := mime.ParseMediaType(strings.ToLower(strings.TrimSpace(contentType)))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(contentType))
	}
	return mt
}

func attachmentText(data []byte) (string, bool) {
	truncated := len(data) > maxAttachmentTextBytes
	if truncated {
		data = data[:maxAttachmentTextBytes]
	}
	text := string(data)
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "")
	}
	return text, truncated
}

func isTextContentType(contentType string) bool {
	switch contentType {
	case "application/json", "application/xml", "application/yaml", "application/x-yaml", "text/xml":
		return true
	default:
		return false
	}
}

func isCodeContentType(contentType string) bool {
	switch contentType {
	case "application/javascript", "application/typescript", "application/x-sh", "application/x-shellscript":
		return true
	default:
		return false
	}
}

func isTextExtension(ext string) bool {
	switch ext {
	case ".txt", ".md", ".csv", ".json", ".xml", ".yaml", ".yml", ".toml", ".ini", ".log":
		return true
	default:
		return false
	}
}

func isCodeExtension(ext string) bool {
	switch ext {
	case ".go", ".js", ".ts", ".jsx", ".tsx", ".py", ".java", ".kt", ".rs", ".rb", ".php", ".cs",
		".c", ".cc", ".cpp", ".h", ".hpp", ".sh", ".bash", ".zsh", ".fish", ".sql", ".html",
		".css", ".scss", ".vue", ".svelte", ".swift", ".scala", ".lua", ".r", ".pl", ".ex",
		".exs", ".clj", ".dart":
		return true
	default:
		return false
	}
}

func isSupportedAttachmentKind(kind AttachmentKind) bool {
	switch kind {
	case AttachmentKindImage, AttachmentKindPDF, AttachmentKindText, AttachmentKindCode:
		return true
	default:
		return false
	}
}
