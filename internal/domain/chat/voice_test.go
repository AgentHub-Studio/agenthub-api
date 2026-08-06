package chat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveVoiceConfigFromModelConfig_UsesNestedVoiceBlock(t *testing.T) {
	cfg := resolveVoiceConfigFromModelConfig(json.RawMessage(`{
		"ttsModel": "legacy-tts",
		"ttsVoice": "legacy-voice",
		"voiceEnabled": false,
		"language": "pt-BR",
		"voice": {
			"enabled": true,
			"ttsModel": "custom-tts",
			"ttsVoice": "nova",
			"language": "en-US"
		}
	}`))

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "custom-tts", cfg.Model)
	assert.Equal(t, "nova", cfg.Voice)
	assert.Equal(t, "en-US", cfg.Language)
}

func TestResolveVoiceConfigFromModelConfig_DefaultsToEnabledForInvalidInput(t *testing.T) {
	cfg := resolveVoiceConfigFromModelConfig(json.RawMessage(`{invalid-json`))

	assert.True(t, cfg.Enabled)
	assert.Empty(t, cfg.Model)
	assert.Empty(t, cfg.Voice)
	assert.Empty(t, cfg.Language)
}

func TestResolveVoiceConfigFromModelConfig_UsesDefaultsWhenEmpty(t *testing.T) {
	cfg := resolveVoiceConfigFromModelConfig(nil)

	assert.True(t, cfg.Enabled)
	assert.Empty(t, cfg.Model)
	assert.Empty(t, cfg.Voice)
	assert.Empty(t, cfg.Language)
}

func TestOpenAIVoiceService_TranscribePreservesEscapedBasePath(t *testing.T) {
	var seenPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":" hello ","language":"pt"}`))
	}))
	defer server.Close()

	service := &OpenAIVoiceService{
		apiKey:   "test-key",
		baseURL:  server.URL + "/tenant%2Fone/v1/",
		sttModel: "whisper-1",
		client:   server.Client(),
	}

	out, err := service.Transcribe(context.Background(), VoiceTranscriptionInput{
		Filename: "hello.wav",
		Audio:    []byte("fake wav"),
	})

	require.NoError(t, err)
	assert.Equal(t, "hello", out.Text)
	assert.Equal(t, "/tenant%2Fone/v1/audio/transcriptions", seenPath)
}

func TestOpenAIVoiceService_RejectsUnsafeBaseURLBeforeRequest(t *testing.T) {
	cases := map[string]func(*OpenAIVoiceService) error{
		"transcribe": func(service *OpenAIVoiceService) error {
			_, err := service.Transcribe(context.Background(), VoiceTranscriptionInput{
				Filename: "hello.wav",
				Audio:    []byte("fake wav"),
			})
			return err
		},
		"synthesize": func(service *OpenAIVoiceService) error {
			_, err := service.Synthesize(context.Background(), VoiceSynthesisInput{
				Text: "hello",
			})
			return err
		},
	}

	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			requests := 0
			service := &OpenAIVoiceService{
				apiKey:   "test-key",
				baseURL:  "https://api.openai.com/v1?next=http://169.254.169.254",
				sttModel: "whisper-1",
				ttsModel: "gpt-4o-mini-tts",
				ttsVoice: "alloy",
				client: &http.Client{Transport: voiceRoundTripFunc(func(*http.Request) (*http.Response, error) {
					requests++
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"text":"ok"}`)),
					}, nil
				})},
			}

			err := run(service)

			require.Error(t, err)
			assert.Zero(t, requests)
		})
	}
}

func TestOpenAIVoiceService_BlocksRedirectToPrivateEndpoint(t *testing.T) {
	cases := map[string]func(*OpenAIVoiceService) error{
		"transcribe": func(service *OpenAIVoiceService) error {
			_, err := service.Transcribe(context.Background(), VoiceTranscriptionInput{
				Filename: "hello.wav",
				Audio:    []byte("fake wav"),
			})
			return err
		},
		"synthesize": func(service *OpenAIVoiceService) error {
			_, err := service.Synthesize(context.Background(), VoiceSynthesisInput{Text: "hello"})
			return err
		},
	}

	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			requests := 0
			service := &OpenAIVoiceService{
				apiKey:   "test-key",
				baseURL:  "https://1.1.1.1/v1",
				sttModel: "whisper-1",
				ttsModel: "gpt-4o-mini-tts",
				ttsVoice: "alloy",
				client: &http.Client{Transport: voiceRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					if requests == 1 {
						return &http.Response{
							StatusCode: http.StatusFound,
							Header:     http.Header{"Location": []string{"http://169.254.169.254/latest/meta-data"}},
							Body:       io.NopCloser(strings.NewReader("redirect")),
							Request:    req,
						}, nil
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"text":"ok"}`)),
						Request:    req,
					}, nil
				})},
			}

			err := run(service)

			require.Error(t, err)
			assert.Equal(t, 1, requests, "a redirect to metadata must not make a second request")
		})
	}
}

type voiceRoundTripFunc func(*http.Request) (*http.Response, error)

func (f voiceRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
