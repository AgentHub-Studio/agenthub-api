package chat

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	EventTranscription = "transcription"
	EventAudioDelta    = "audio_delta"
)

// VoiceService transcribes inbound audio and optionally synthesizes speech.
type VoiceService interface {
	Transcribe(ctx context.Context, in VoiceTranscriptionInput) (VoiceTranscription, error)
	Synthesize(ctx context.Context, in VoiceSynthesisInput) (VoiceAudio, error)
}

type VoiceTranscriptionInput struct {
	Filename    string
	ContentType string
	Audio       []byte
	Language    string
}

type VoiceTranscription struct {
	Text       string  `json:"text"`
	Language   string  `json:"language,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type VoiceSynthesisInput struct {
	Text  string
	Voice string
}

type VoiceAudio struct {
	Format string `json:"format"`
	Base64 string `json:"base64"`
}

type VoiceAudioDelta struct {
	Chunk  string `json:"chunk"`
	Format string `json:"format"`
}

type VoiceInputResponse struct {
	SessionID     string             `json:"sessionId"`
	RunID         *string            `json:"runId,omitempty"`
	Status        string             `json:"status"`
	Transcription VoiceTranscription `json:"transcription"`
	Audio         *VoiceAudio        `json:"audio,omitempty"`
}

type OpenAIVoiceService struct {
	apiKey   string
	baseURL  string
	sttModel string
	ttsModel string
	ttsVoice string
	client   *http.Client
}

func NewOpenAIVoiceServiceFromEnv() *OpenAIVoiceService {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("VOICE_OPENAI_API_KEY"))
	}
	if apiKey == "" {
		return nil
	}
	baseURL := strings.TrimRight(os.Getenv("OPENAI_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	sttModel := os.Getenv("VOICE_STT_MODEL")
	if sttModel == "" {
		sttModel = "whisper-1"
	}
	ttsModel := os.Getenv("VOICE_TTS_MODEL")
	if ttsModel == "" {
		ttsModel = "gpt-4o-mini-tts"
	}
	ttsVoice := os.Getenv("VOICE_TTS_VOICE")
	if ttsVoice == "" {
		ttsVoice = "alloy"
	}
	return &OpenAIVoiceService{
		apiKey:   apiKey,
		baseURL:  baseURL,
		sttModel: sttModel,
		ttsModel: ttsModel,
		ttsVoice: ttsVoice,
		client:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (s *OpenAIVoiceService) Transcribe(ctx context.Context, in VoiceTranscriptionInput) (VoiceTranscription, error) {
	if s == nil || s.apiKey == "" {
		return VoiceTranscription{}, fmt.Errorf("voice: provider is not configured")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", safeVoiceFilename(in.Filename))
	if err != nil {
		return VoiceTranscription{}, err
	}
	if _, err := part.Write(in.Audio); err != nil {
		return VoiceTranscription{}, err
	}
	_ = writer.WriteField("model", s.sttModel)
	if in.Language != "" {
		_ = writer.WriteField("language", in.Language)
	}
	if err := writer.Close(); err != nil {
		return VoiceTranscription{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/audio/transcriptions", &body)
	if err != nil {
		return VoiceTranscription{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := s.client.Do(req)
	if err != nil {
		return VoiceTranscription{}, fmt.Errorf("voice: transcribe request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return VoiceTranscription{}, fmt.Errorf("voice: transcribe status %d: %s", resp.StatusCode, string(respBody))
	}
	var decoded struct {
		Text     string `json:"text"`
		Language string `json:"language,omitempty"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return VoiceTranscription{}, fmt.Errorf("voice: transcribe decode: %w", err)
	}
	return VoiceTranscription{Text: strings.TrimSpace(decoded.Text), Language: decoded.Language}, nil
}

func (s *OpenAIVoiceService) Synthesize(ctx context.Context, in VoiceSynthesisInput) (VoiceAudio, error) {
	if s == nil || s.apiKey == "" {
		return VoiceAudio{}, fmt.Errorf("voice: provider is not configured")
	}
	voice := in.Voice
	if voice == "" {
		voice = s.ttsVoice
	}
	payload, _ := json.Marshal(map[string]any{
		"model":           s.ttsModel,
		"voice":           voice,
		"input":           in.Text,
		"response_format": "mp3",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/audio/speech", bytes.NewReader(payload))
	if err != nil {
		return VoiceAudio{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return VoiceAudio{}, fmt.Errorf("voice: synthesize request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	audio, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return VoiceAudio{}, fmt.Errorf("voice: synthesize status %d: %s", resp.StatusCode, string(audio))
	}
	return VoiceAudio{Format: "mp3", Base64: base64.StdEncoding.EncodeToString(audio)}, nil
}

func safeVoiceFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "audio.wav"
	}
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}
