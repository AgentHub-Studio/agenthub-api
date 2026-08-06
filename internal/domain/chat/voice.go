package chat

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

const (
	EventTranscription        = "transcription"
	EventAudioDelta           = "audio_delta"
	defaultOpenAIVoiceBaseURL = "https://api.openai.com/v1"
)

var errOpenAIVoiceRedirectNotAllowed = errors.New("voice: redirect target is not allowed")

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
	Text     string
	Voice    string
	Model    string
	Language string
}

// VoiceSynthesisConfig is a resolved set of voice settings for one run.
type VoiceSynthesisConfig struct {
	Enabled  bool
	Model    string
	Voice    string
	Language string
}

// parseVoiceConfigBool normalizes boolean-like values in JSON model_config objects.
func parseVoiceConfigBool(v any) (bool, bool) {
	switch cast := v.(type) {
	case bool:
		return cast, true
	default:
		return false, false
	}
}

// parseVoiceConfigString normalizes string-like values in JSON model_config objects.
func parseVoiceConfigString(v any) (string, bool) {
	str, ok := v.(string)
	if !ok {
		return "", false
	}
	s := strings.TrimSpace(str)
	if s == "" {
		return "", false
	}
	return s, true
}

// resolveVoiceConfigFromModelConfig returns voice settings from a model_config blob.
// Unknown and unsupported keys are ignored.
func resolveVoiceConfigFromModelConfig(raw json.RawMessage) VoiceSynthesisConfig {
	cfg := VoiceSynthesisConfig{Enabled: true}
	if len(raw) == 0 {
		return cfg
	}

	var rawConfig map[string]any
	if err := json.Unmarshal(raw, &rawConfig); err != nil {
		return cfg
	}

	// Top-level legacy flags.
	if enabled, ok := parseVoiceConfigBool(rawConfig["voiceEnabled"]); ok {
		cfg.Enabled = enabled
	}
	if model, ok := parseVoiceConfigString(rawConfig["ttsModel"]); ok {
		cfg.Model = model
	}
	if voice, ok := parseVoiceConfigString(rawConfig["ttsVoice"]); ok {
		cfg.Voice = voice
	}
	if language, ok := parseVoiceConfigString(rawConfig["language"]); ok {
		cfg.Language = language
	}

	// Nested `voice` block (preferred format).
	voiceCfg, ok := rawConfig["voice"].(map[string]any)
	if !ok {
		return cfg
	}
	if enabled, ok := parseVoiceConfigBool(voiceCfg["enabled"]); ok {
		cfg.Enabled = enabled
	}
	if model, ok := parseVoiceConfigString(voiceCfg["model"]); ok {
		cfg.Model = model
	}
	if model, ok := parseVoiceConfigString(voiceCfg["ttsModel"]); ok {
		cfg.Model = model
	}
	if voice, ok := parseVoiceConfigString(voiceCfg["voice"]); ok {
		cfg.Voice = voice
	}
	if voice, ok := parseVoiceConfigString(voiceCfg["ttsVoice"]); ok {
		cfg.Voice = voice
	}
	if language, ok := parseVoiceConfigString(voiceCfg["language"]); ok {
		cfg.Language = language
	}
	return cfg
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
		baseURL = defaultOpenAIVoiceBaseURL
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

	endpoint, err := buildOpenAIVoiceEndpoint(s.baseURL, "audio", "transcriptions")
	if err != nil {
		return VoiceTranscription{}, fmt.Errorf("voice: transcribe endpoint: %w", err)
	}
	// #nosec G704 -- endpoint is built from a validated OpenAI-compatible base URL and escaped path segments.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return VoiceTranscription{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// #nosec G704 -- the configured base URL is parsed and every redirect is checked against the SSRF policy.
	resp, err := s.validatedHTTPClient().Do(req)
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
	model := in.Model
	if model == "" {
		model = s.ttsModel
	}
	payload, _ := json.Marshal(map[string]any{
		"model":           model,
		"voice":           voice,
		"input":           in.Text,
		"response_format": "mp3",
	})
	endpoint, err := buildOpenAIVoiceEndpoint(s.baseURL, "audio", "speech")
	if err != nil {
		return VoiceAudio{}, fmt.Errorf("voice: synthesize endpoint: %w", err)
	}
	// #nosec G704 -- endpoint is built from a validated OpenAI-compatible base URL and escaped path segments.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return VoiceAudio{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	// #nosec G704 -- the configured base URL is parsed and every redirect is checked against the SSRF policy.
	resp, err := s.validatedHTTPClient().Do(req)
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

func (s *OpenAIVoiceService) validatedHTTPClient() *http.Client {
	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	protected := *client
	previousCheckRedirect := client.CheckRedirect
	protected.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := ssrf.ValidateURL(req.URL.String()); err != nil {
			return errOpenAIVoiceRedirectNotAllowed
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		return nil
	}
	return &protected
}

func buildOpenAIVoiceEndpoint(rawBase string, segments ...string) (string, error) {
	if strings.TrimSpace(rawBase) == "" {
		rawBase = defaultOpenAIVoiceBaseURL
	}
	base, err := parseOpenAIVoiceBaseURL(rawBase)
	if err != nil {
		return "", err
	}
	if err := appendEscapedOpenAIVoicePathSegments(base, segments...); err != nil {
		return "", err
	}
	return base.String(), nil
}

func parseOpenAIVoiceBaseURL(rawBase string) (*url.URL, error) {
	rawBase = strings.TrimRight(strings.TrimSpace(rawBase), "/")
	if rawBase == "" {
		return nil, fmt.Errorf("empty base URL")
	}
	if containsOpenAIVoiceURLControlChar(rawBase) {
		return nil, fmt.Errorf("base URL contains control characters")
	}
	parsed, err := url.Parse(rawBase)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("missing host")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("userinfo is not allowed")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("query and fragment are not allowed")
	}
	return parsed, nil
}

func appendEscapedOpenAIVoicePathSegments(base *url.URL, segments ...string) error {
	escapedPath := strings.TrimRight(base.EscapedPath(), "/")
	for _, segment := range segments {
		if segment == "" || containsOpenAIVoiceURLControlChar(segment) {
			return fmt.Errorf("invalid path segment")
		}
		escapedPath += "/" + url.PathEscape(segment)
	}
	decodedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return err
	}
	base.Path = decodedPath
	base.RawPath = escapedPath
	return nil
}

func containsOpenAIVoiceURLControlChar(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	}) >= 0
}
