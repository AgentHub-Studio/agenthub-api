package chat_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatHandler_ResumeElicitationAliasesMustAgree(t *testing.T) {
	requestID := "approval-1"

	t.Run("equivalent aliases preserve compatibility", func(t *testing.T) {
		r, svc := setupChat()
		svc.elicitationOK = true
		body := `{"requestId":"` + requestID + `","resume_data":{"choice":"approve"},"resumeData":{"choice":"approve"},"content":{"choice":"approve"},"values":{"choice":"approve"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+uuid.NewString()+"/resume", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
		require.Equal(t, 1, svc.elicitationCalls)
		assert.Equal(t, map[string]interface{}{"choice": "approve"}, svc.lastElicitationResult.Content)
	})

	for _, tc := range []struct {
		name string
		body string
	}{
		{
			name: "snake and camel resume data",
			body: `{"requestId":"` + requestID + `","resume_data":{"choice":"approve"},"resumeData":{"choice":"decline"}}`,
		},
		{
			name: "content and values",
			body: `{"requestId":"` + requestID + `","content":{"choice":"approve"},"values":{"choice":"decline"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, svc := setupChat()
			svc.elicitationOK = true
			req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+uuid.NewString()+"/resume", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Zero(t, svc.elicitationCalls)
		})
	}
}

func TestChatHandler_RespondElicitationAliasesMustAgree(t *testing.T) {
	requestID := "approval-1"

	t.Run("equivalent aliases preserve compatibility", func(t *testing.T) {
		r, svc := setupChat()
		svc.elicitationOK = true
		body := `{"action":"accept","content":{"choice":"approve"},"values":{"choice":"approve"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+uuid.NewString()+"/elicitation/"+requestID+"/respond", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
		require.Equal(t, 1, svc.elicitationCalls)
		assert.Equal(t, map[string]interface{}{"choice": "approve"}, svc.lastElicitationResult.Content)
	})

	t.Run("conflicting aliases do not resolve the elicitation", func(t *testing.T) {
		r, svc := setupChat()
		svc.elicitationOK = true
		body := `{"action":"accept","content":{"choice":"approve"},"values":{"choice":"decline"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+uuid.NewString()+"/elicitation/"+requestID+"/respond", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Zero(t, svc.elicitationCalls)
	})
}
