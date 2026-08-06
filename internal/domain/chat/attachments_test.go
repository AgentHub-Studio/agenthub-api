package chat_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

func TestBuildUploadedChatAttachment_Text(t *testing.T) {
	att, err := chat.BuildUploadedChatAttachment("notes.md", "text/markdown", "", "", []byte("# Title\nbody"))
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, att.ID)
	assert.Equal(t, "notes.md", att.Name)
	assert.Equal(t, chat.AttachmentKindText, att.Kind)
	assert.Equal(t, "# Title\nbody", att.Text)
}

func TestBuildUploadedChatAttachment_RejectsUnsupportedType(t *testing.T) {
	_, err := chat.BuildUploadedChatAttachment("archive.zip", "application/zip", "", "", []byte("zip"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported attachment type")
}

func TestNormalizeChatAttachments_RejectsTooMany(t *testing.T) {
	attachments := make([]chat.ChatAttachment, chat.MaxAttachmentsPerMessage+1)
	for i := range attachments {
		attachments[i] = chat.ChatAttachment{
			ID:   uuid.New(),
			Name: "file.txt",
			Type: "text/plain",
			Size: 1,
			Kind: chat.AttachmentKindText,
		}
	}
	raw, err := json.Marshal(attachments)
	require.NoError(t, err)

	_, err = chat.NormalizeChatAttachments(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum of 5 attachments")
}

func TestUserMessageWithAttachmentContext(t *testing.T) {
	raw, err := json.Marshal([]chat.ChatAttachment{{
		ID:   uuid.New(),
		Name: "data.csv",
		Type: "text/csv",
		Size: 7,
		Kind: chat.AttachmentKindText,
		Text: "a,b\n1,2",
	}})
	require.NoError(t, err)

	content := chat.UserMessageWithAttachmentContext("Analyze this", raw)
	assert.Contains(t, content, "Analyze this")
	assert.Contains(t, content, "[Attached files]")
	assert.Contains(t, content, "a,b\n1,2")
}
