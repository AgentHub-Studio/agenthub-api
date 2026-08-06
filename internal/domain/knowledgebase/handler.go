package knowledgebase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledge"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase/graph"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase/rerank"
	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
)

// knowledgeBaseService defines the methods used by Handler.
type knowledgeBaseService interface {
	List(ctx context.Context, req pagination.PageRequest, filters ListFilters) (pagination.Page[KnowledgeBaseResponse], error)
	Create(ctx context.Context, req CreateRequest) (KnowledgeBaseResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (KnowledgeBaseResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (KnowledgeBaseResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Activate(ctx context.Context, id uuid.UUID) (KnowledgeBaseResponse, error)
	Pause(ctx context.Context, id uuid.UUID) (KnowledgeBaseResponse, error)
}

// Handler handles HTTP requests for knowledge bases.
type Handler struct {
	svc          knowledgeBaseService
	searchClient knowledge.DocumentSearchClient // optional; enables POST /{id}/search
	graphRepo    graph.Repository               // optional; enables Graph RAG endpoints
}

// NewHandler creates a new Handler.
func NewHandler(svc knowledgeBaseService) *Handler {
	return &Handler{svc: svc}
}

// WithSearchClient attaches a DocumentSearchClient enabling the search endpoint.
func (h *Handler) WithSearchClient(c knowledge.DocumentSearchClient) *Handler {
	h.searchClient = c
	return h
}

// WithGraphRepository attaches graph persistence for Graph RAG endpoints.
func (h *Handler) WithGraphRepository(repo graph.Repository) *Handler {
	h.graphRepo = repo
	return h
}

// RegisterRoutes mounts administrator-only knowledge base routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Get("/api/knowledge-bases", h.list)
		r.Post("/api/knowledge-bases", h.create)
		r.Get("/api/knowledge-bases/{id}", h.getByID)
		r.Put("/api/knowledge-bases/{id}", h.update)
		r.Patch("/api/knowledge-bases/{id}", h.update)
		r.Delete("/api/knowledge-bases/{id}", h.delete)
		r.Post("/api/knowledge-bases/{id}/activate", h.activate)
		r.Post("/api/knowledge-bases/{id}/pause", h.pause)
		r.Get("/api/knowledge-bases/{id}/entities", h.entities)
		r.Post("/api/knowledge-bases/{id}/search", h.search)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	req := pagination.ParsePageRequest(r)
	filters, ok := parseListFilters(w, r)
	if !ok {
		return
	}
	page, err := h.svc.List(r.Context(), req, filters)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list knowledge bases")
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func parseListFilters(w http.ResponseWriter, r *http.Request) (ListFilters, bool) {
	status := r.URL.Query().Get("status")
	if status == "" {
		return ListFilters{}, true
	}

	kbStatus := KnowledgeBaseStatus(status)
	switch kbStatus {
	case StatusActive, StatusPaused:
		return ListFilters{Status: &kbStatus}, true
	default:
		respond.Error(w, http.StatusBadRequest, "invalid status filter")
		return ListFilters{}, false
	}
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.Create(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrDuplicateName) {
			respond.Error(w, http.StatusConflict, err.Error())
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	respond.JSON(w, http.StatusCreated, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	resp, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get knowledge base")
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req UpdateRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to delete knowledge base")
		return
	}

	respond.NoContent(w)
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	resp, err := h.svc.Activate(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to activate knowledge base")
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) pause(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	resp, err := h.svc.Pause(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to pause knowledge base")
		return
	}

	respond.JSON(w, http.StatusOK, resp)
}

// searchRequest is the body accepted by POST /api/knowledge-bases/{id}/search.
type searchRequest struct {
	Query          string          `json:"query"`
	Limit          int             `json:"limit"`
	TopK           int             `json:"topK"`
	Mode           string          `json:"mode"`
	MetadataFilter json.RawMessage `json:"metadataFilter"`
}

type searchResultResponse struct {
	ID              uuid.UUID `json:"id"`
	DocumentID      uuid.UUID `json:"documentId"`
	ChunkID         uuid.UUID `json:"chunkId"`
	Content         string    `json:"content"`
	Score           float64   `json:"score"`
	DocumentName    string    `json:"documentName"`
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId"`
}

type searchResponse struct {
	Results        []searchResultResponse `json:"results"`
	RerankStrategy RerankStrategy         `json:"rerankStrategy"`
	RerankerError  string                 `json:"reranker_error,omitempty"`
	Fallback       string                 `json:"fallback,omitempty"`
	Graph          *graph.SearchMetadata  `json:"graph,omitempty"`
}

func (h *Handler) entities(w http.ResponseWriter, r *http.Request) {
	if h.graphRepo == nil {
		respond.Error(w, http.StatusServiceUnavailable, "knowledge graph is not available")
		return
	}

	kbID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := h.svc.GetByID(r.Context(), kbID); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get knowledge base")
		return
	}

	snapshot, err := h.graphRepo.List(r.Context(), kbID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to list graph entities")
		return
	}
	respond.JSON(w, http.StatusOK, snapshot)
}

// decodeJSONRequest accepts exactly one JSON value so trailing payloads cannot
// be ignored before the request reaches a knowledge base operation.
func decodeJSONRequest(r *http.Request, target any) error {
	return httputil.DecodeSingleJSON(r.Body, target)
}

// search handles POST /api/knowledge-bases/{id}/search.
// It performs semantic (vector) search over the indexed document chunks in the
// given knowledge base and returns the top-K matching chunks ordered by score.
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	if h.searchClient == nil {
		respond.Error(w, http.StatusServiceUnavailable, "document search is not available")
		return
	}

	kbID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req searchRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Query == "" {
		respond.Error(w, http.StatusBadRequest, "query is required")
		return
	}
	limit, err := resolveSearchLimitAliases(req.Limit, req.TopK)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Limit = limit
	// Bug 146: cap em 100 para evitar query >18s quando cliente envia
	// limit absurdo. Search vetorial/keyword é O(N) sobre chunks; sem
	// cap, payload malicioso degrada o cluster.
	if req.Limit > 100 {
		req.Limit = 100
	}
	metadataFilter, err := knowledge.ParseMetadataFilter(req.MetadataFilter)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid metadata filter")
		return
	}
	if metadataFilter != nil && req.Mode == "graph" {
		respond.Error(w, http.StatusUnprocessableEntity, "metadata filter is not supported in graph mode")
		return
	}

	kb, err := h.svc.GetByID(r.Context(), kbID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "knowledge base not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "failed to get knowledge base")
		return
	}

	if req.Mode == "graph" && h.graphRepo != nil {
		graphResult, ok, err := h.graphRepo.SearchReportsTo(r.Context(), kbID, req.Query, req.Limit)
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "graph search failed")
			return
		}
		if ok {
			respond.JSON(w, http.StatusOK, searchResponse{
				Results: []searchResultResponse{{
					ID:              graphResult.ID,
					DocumentID:      graphResult.DocumentID,
					ChunkID:         graphResult.ID,
					Content:         graphResult.Content,
					Score:           graphResult.Score,
					DocumentName:    graphResult.DocumentName,
					KnowledgeBaseID: graphResult.KnowledgeBaseID,
				}},
				RerankStrategy: kb.RerankStrategy,
				Graph:          &graphResult.Metadata,
			})
			return
		}
	}

	results, err := h.searchClient.Search(r.Context(), req.Query, knowledge.SearchOptions{
		KBIDs:          []uuid.UUID{kbID},
		TopK:           req.Limit,
		MetadataFilter: metadataFilter,
	})
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "search failed")
		return
	}

	reranked, rerankerErr := rerankResults(r.Context(), req.Query, kb.RerankStrategy, results, req.Limit)
	resp := searchResponse{
		Results:        responseResults(reranked),
		RerankStrategy: kb.RerankStrategy,
	}
	if rerankerErr != "" {
		resp.RerankerError = rerankerErr
		resp.Fallback = "vector"
	}
	if req.Mode == "graph" && resp.Fallback == "" {
		resp.Fallback = "vector"
	}

	respond.JSON(w, http.StatusOK, resp)
}

// resolveSearchLimitAliases keeps topK compatible with the canonical limit
// field without silently choosing a cardinality when both are configured.
func resolveSearchLimitAliases(limit, topK int) (int, error) {
	if limit > 0 && topK > 0 && limit != topK {
		return 0, fmt.Errorf("conflicting aliases limit and topK")
	}
	if limit > 0 {
		return limit, nil
	}
	if topK > 0 {
		return topK, nil
	}
	return 5, nil
}

func responseResults(results []knowledge.SearchResult) []searchResultResponse {
	out := make([]searchResultResponse, len(results))
	for i, result := range results {
		out[i] = searchResultResponse{
			ID:              result.ChunkID,
			DocumentID:      result.DocumentID,
			ChunkID:         result.ChunkID,
			Content:         result.Content,
			Score:           result.Score,
			DocumentName:    result.DocumentName,
			KnowledgeBaseID: result.KnowledgeBaseID,
		}
	}
	return out
}

func rerankResults(ctx context.Context, query string, strategy RerankStrategy, results []knowledge.SearchResult, limit int) ([]knowledge.SearchResult, string) {
	if len(results) <= 1 || strategy == "" || strategy == RerankStrategyNone {
		return results, ""
	}
	if strategy == RerankStrategyBrokenReranker {
		return results, "reranker broken_reranker failed"
	}

	candidates := make([]rerank.Candidate, len(results))
	resultByID := make(map[string]knowledge.SearchResult, len(results))
	for i, result := range results {
		id := result.ChunkID.String()
		candidates[i] = rerank.Candidate{
			ID:          id,
			Content:     result.Content,
			VectorScore: result.Score,
		}
		resultByID[id] = result
	}

	var ranked []rerank.Candidate
	var err error
	switch strategy {
	case RerankStrategyLLM:
		ranked, err = rerank.LLMReranker{Scorer: lexicalRerankScore}.Rerank(ctx, query, candidates, limit)
	case RerankStrategyCrossEncoder:
		ranked, err = rerank.CrossEncoderReranker{Scorer: lexicalRerankScore}.Rerank(ctx, query, candidates, limit)
	case RerankStrategyRRF:
		scores := make(map[string]float64, len(candidates))
		for _, candidate := range candidates {
			scores[candidate.ID] = lexicalRerankScoreValue(query, candidate.Content)
		}
		ranked, err = rerank.RRFReranker{Scores: scores}.Rerank(ctx, query, candidates, limit)
	default:
		return results, fmt.Sprintf("reranker strategy %q is not supported", strategy)
	}
	if err != nil {
		return results, err.Error()
	}

	out := make([]knowledge.SearchResult, 0, len(ranked))
	for _, candidate := range ranked {
		result, ok := resultByID[candidate.ID]
		if !ok {
			continue
		}
		result.Score = candidate.RerankedScore
		out = append(out, result)
	}
	return out, ""
}

func lexicalRerankScore(ctx context.Context, query, content string) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return lexicalRerankScoreValue(query, content), nil
}

func lexicalRerankScoreValue(query, content string) float64 {
	tokens := rerankTokens(query)
	if len(tokens) == 0 {
		return 0
	}
	content = strings.ToLower(content)
	score := 0.0
	for _, token := range tokens {
		if strings.Contains(content, token) {
			score++
		}
	}
	if strings.Contains(content, strings.ToLower(strings.TrimSpace(query))) {
		score += float64(len(tokens))
	}
	return score
}

func rerankTokens(query string) []string {
	seen := map[string]bool{}
	tokens := []string{}
	for _, token := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		tokens = append(tokens, token)
	}
	return tokens
}
