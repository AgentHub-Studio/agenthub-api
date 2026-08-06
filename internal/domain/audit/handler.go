package audit

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/respond"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

const (
	auditExportChecksumAlgorithm = "sha256"
	auditExportPDFContentType    = "application/pdf"
	auditExportXLSXContentType   = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

// auditService is the interface required by Handler.
type auditService interface {
	ListAll(ctx context.Context, tenantID string, f ListFilter, pr pagination.PageRequest) ([]AuditLog, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (AuditLog, error)
	Record(ctx context.Context, tenantID string, req RecordRequest) (AuditLog, error)
	ApplyRetention(ctx context.Context, tenantID string, req AuditRetentionRequest, now time.Time) (AuditRetentionResponse, error)
}

// Handler handles HTTP requests for audit logs.
type Handler struct {
	svc auditService
}

// NewHandler creates a new Handler.
func NewHandler(svc auditService) *Handler {
	return &Handler{svc: svc}
}

// Routes mounts the handler routes.
// GET endpoints (list, export, getByID) require the "admin" role.
// POST (record) is accessible to any authenticated caller so other services can log events.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.record)
	r.With(middleware.RequireRole("admin")).Get("/", h.list)
	r.With(middleware.RequireRole("admin")).Get("/export", h.exportJSON)
	r.With(middleware.RequireRole("admin")).Post("/retention/apply", h.applyRetention)
	r.With(middleware.RequireRole("admin")).Get("/{id}", h.getByID)
	return r
}

// ExtractIP returns the client IP from X-Forwarded-For or RemoteAddr.
func ExtractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (leftmost) address, which is the original client.
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	pr := pagination.ParsePageRequest(r)
	f := parseListFilter(r)

	items, total, err := h.svc.ListAll(r.Context(), tenantID, f, pr)
	if err != nil {
		// Bug 194: nunca expor err.Error() em fallback 500.
		slog.Error("audit: list failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "list failed")
		return
	}

	dtos := make([]AuditLogResponse, len(items))
	for i, l := range items {
		dtos[i] = ResponseFrom(l)
	}
	respond.JSON(w, http.StatusOK, pagination.NewPage(dtos, int64(total), pr))
}

func parseListFilter(r *http.Request) ListFilter {
	f := ListFilter{
		EntityType: r.URL.Query().Get("entityType"),
		EntityID:   r.URL.Query().Get("entityId"),
		Action:     r.URL.Query().Get("action"),
		ActorID:    r.URL.Query().Get("actorId"),
	}

	if v := r.URL.Query().Get("dateFrom"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.DateFrom = &t
		}
	}
	if v := r.URL.Query().Get("dateTo"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.DateTo = &t
		}
	}
	return f
}

func (h *Handler) exportJSON(w http.ResponseWriter, r *http.Request) {
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" && format != "zip_bundle" && format != "xlsx" && format != "pdf" {
		respond.Error(w, http.StatusBadRequest, "unsupported export format")
		return
	}

	tenantID := tenant.FromContext(r.Context())
	pr := pagination.ParsePageRequest(r)
	f := parseListFilter(r)
	items, total, err := h.svc.ListAll(r.Context(), tenantID, f, pr)
	if err != nil {
		slog.Error("audit: export failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "export failed")
		return
	}

	dtos := make([]AuditLogResponse, len(items))
	for i, l := range items {
		dtos[i] = ResponseFrom(l)
	}
	if format == "csv" {
		h.writeCSVExport(w, tenantID, f, pr, total, dtos)
		return
	}
	if format == "xlsx" {
		h.writeXLSXExport(w, tenantID, f, pr, total, dtos)
		return
	}
	if format == "pdf" {
		h.writePDFExport(w, tenantID, f, pr, total, dtos)
		return
	}
	if format == "zip_bundle" {
		h.writeZipBundleExport(w, tenantID, f, pr, total, dtos)
		return
	}

	resp, err := auditExportPayload(tenantID, "json", f, pr, total, dtos, time.Now().UTC())
	if err != nil {
		slog.Error("audit: json export marshal failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "export failed")
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="audit-logs.json"`)
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) writeCSVExport(w http.ResponseWriter, tenantID string, f ListFilter, pr pagination.PageRequest, total int, records []AuditLogResponse) {
	body, err := auditLogsCSV(records)
	if err != nil {
		slog.Error("audit: csv export marshal failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "export failed")
		return
	}
	sum := sha256.Sum256(body)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="audit-logs.csv"`)
	setAuditExportHeaders(w, tenantID, "csv", f, pr, total, hex.EncodeToString(sum[:]))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) writePDFExport(w http.ResponseWriter, tenantID string, f ListFilter, pr pagination.PageRequest, total int, records []AuditLogResponse) {
	body, err := auditLogsPDF(time.Now().UTC(), records)
	if err != nil {
		slog.Error("audit: pdf export marshal failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "export failed")
		return
	}

	w.Header().Set("Content-Type", auditExportPDFContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="audit-logs.pdf"`)
	setAuditExportHeaders(w, tenantID, "pdf", f, pr, total, sha256Hex(body))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) writeXLSXExport(w http.ResponseWriter, tenantID string, f ListFilter, pr pagination.PageRequest, total int, records []AuditLogResponse) {
	body, err := auditLogsXLSX(time.Now().UTC(), records)
	if err != nil {
		slog.Error("audit: xlsx export marshal failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "export failed")
		return
	}

	w.Header().Set("Content-Type", auditExportXLSXContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="audit-logs.xlsx"`)
	setAuditExportHeaders(w, tenantID, "xlsx", f, pr, total, sha256Hex(body))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) writeZipBundleExport(w http.ResponseWriter, tenantID string, f ListFilter, pr pagination.PageRequest, total int, records []AuditLogResponse) {
	generatedAt := time.Now().UTC()
	jsonPayload, err := auditExportPayload(tenantID, "json", f, pr, total, records, generatedAt)
	if err != nil {
		slog.Error("audit: zip json export marshal failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "export failed")
		return
	}
	jsonBody, err := json.Marshal(jsonPayload)
	if err != nil {
		slog.Error("audit: zip json body marshal failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "export failed")
		return
	}
	csvBody, err := auditLogsCSV(records)
	if err != nil {
		slog.Error("audit: zip csv export marshal failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "export failed")
		return
	}

	jsonChecksum := sha256Hex(jsonBody)
	csvChecksum := sha256Hex(csvBody)
	manifest := AuditExportBundleManifest{
		TenantID:      tenantID,
		Format:        "zip_bundle",
		GeneratedAt:   generatedAt,
		Filters:       ExportFilterResponseFrom(f),
		Page:          pr.Page,
		Size:          pr.Size,
		TotalElements: int64(total),
		Files: []AuditExportBundleFile{
			{
				Path:      "audit-logs.json",
				Format:    "json",
				Bytes:     int64(len(jsonBody)),
				Algorithm: auditExportChecksumAlgorithm,
				Checksum:  jsonChecksum,
			},
			{
				Path:      "audit-logs.csv",
				Format:    "csv",
				Bytes:     int64(len(csvBody)),
				Algorithm: auditExportChecksumAlgorithm,
				Checksum:  csvChecksum,
			},
		},
	}
	manifestForChecksum, err := json.Marshal(manifest)
	if err != nil {
		slog.Error("audit: zip manifest checksum marshal failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "export failed")
		return
	}
	manifest.Integrity = AuditExportIntegrity{
		Algorithm: auditExportChecksumAlgorithm,
		Checksum:  sha256Hex(manifestForChecksum),
	}
	manifestBody, err := json.Marshal(manifest)
	if err != nil {
		slog.Error("audit: zip manifest marshal failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "export failed")
		return
	}

	body, err := auditLogsZipBundle(generatedAt, []auditExportZipFile{
		{name: "manifest.json", body: manifestBody},
		{name: "audit-logs.json", body: jsonBody},
		{name: "audit-logs.csv", body: csvBody},
	})
	if err != nil {
		slog.Error("audit: zip bundle marshal failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "export failed")
		return
	}
	sum := sha256.Sum256(body)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="audit-logs.zip"`)
	setAuditExportHeaders(w, tenantID, "zip_bundle", f, pr, total, hex.EncodeToString(sum[:]))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func setAuditExportHeaders(w http.ResponseWriter, tenantID, format string, f ListFilter, pr pagination.PageRequest, total int, checksum string) {
	w.Header().Set("X-Audit-Export-Format", format)
	w.Header().Set("X-Audit-Export-Tenant-ID", tenantID)
	w.Header().Set("X-Audit-Export-Page", strconv.Itoa(pr.Page))
	w.Header().Set("X-Audit-Export-Size", strconv.Itoa(pr.Size))
	w.Header().Set("X-Audit-Export-Total-Elements", strconv.Itoa(total))
	w.Header().Set("X-Audit-Export-Integrity-Algorithm", auditExportChecksumAlgorithm)
	w.Header().Set("X-Audit-Export-Integrity-Checksum", checksum)
	if f.EntityType != "" {
		w.Header().Set("X-Audit-Export-Filter-Entity-Type", f.EntityType)
	}
	if f.EntityID != "" {
		w.Header().Set("X-Audit-Export-Filter-Entity-ID", f.EntityID)
	}
	if f.Action != "" {
		w.Header().Set("X-Audit-Export-Filter-Action", f.Action)
	}
	if f.ActorID != "" {
		w.Header().Set("X-Audit-Export-Filter-Actor-ID", f.ActorID)
	}
	if f.DateFrom != nil {
		w.Header().Set("X-Audit-Export-Filter-Date-From", f.DateFrom.UTC().Format(time.RFC3339Nano))
	}
	if f.DateTo != nil {
		w.Header().Set("X-Audit-Export-Filter-Date-To", f.DateTo.UTC().Format(time.RFC3339Nano))
	}
}

func auditExportPayload(tenantID, format string, f ListFilter, pr pagination.PageRequest, total int, records []AuditLogResponse, generatedAt time.Time) (AuditExportResponse, error) {
	recordPayload, err := json.Marshal(records)
	if err != nil {
		return AuditExportResponse{}, err
	}
	sum := sha256.Sum256(recordPayload)
	return AuditExportResponse{
		TenantID:      tenantID,
		Format:        format,
		GeneratedAt:   generatedAt,
		Filters:       ExportFilterResponseFrom(f),
		Page:          pr.Page,
		Size:          pr.Size,
		TotalElements: int64(total),
		Records:       records,
		Integrity: AuditExportIntegrity{
			Algorithm: auditExportChecksumAlgorithm,
			Checksum:  hex.EncodeToString(sum[:]),
		},
	}, nil
}

func auditLogsCSV(records []AuditLogResponse) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"id", "entityType", "entityId", "action", "actorId", "actorEmail", "ipAddress", "createdAt", "metadata"}); err != nil {
		return nil, err
	}
	for _, record := range records {
		if err := w.Write([]string{
			record.ID.String(),
			record.EntityType,
			record.EntityID,
			string(record.Action),
			record.ActorID,
			record.ActorEmail,
			record.IPAddress,
			record.CreatedAt.UTC().Format(time.RFC3339Nano),
			record.Metadata,
		}); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type auditExportZipFile struct {
	name string
	body []byte
}

func auditLogsZipBundle(generatedAt time.Time, files []auditExportZipFile) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, file := range files {
		h := &zip.FileHeader{
			Name:   file.name,
			Method: zip.Deflate,
		}
		h.Modified = generatedAt
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(file.body); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func auditLogsPDF(generatedAt time.Time, records []AuditLogResponse) ([]byte, error) {
	lines := auditLogsPDFLines(generatedAt, records)
	const linesPerPage = 32
	pageCount := (len(lines) + linesPerPage - 1) / linesPerPage
	if pageCount == 0 {
		pageCount = 1
	}

	totalObjects := 3 + pageCount*2
	objects := make([]string, totalObjects+1)
	objects[1] = "<< /Type /Catalog /Pages 2 0 R >>"
	objects[3] = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"

	kids := make([]string, 0, pageCount)
	for i := 0; i < pageCount; i++ {
		start := i * linesPerPage
		end := start + linesPerPage
		if end > len(lines) {
			end = len(lines)
		}
		content := auditLogsPDFPageContent(lines[start:end])
		contentObject := 4 + i*2
		pageObject := contentObject + 1
		objects[contentObject] = "<< /Length " + strconv.Itoa(len(content)) + " >>\nstream\n" + content + "\nendstream"
		objects[pageObject] = "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 842 595] /Resources << /Font << /F1 3 0 R >> >> /Contents " + strconv.Itoa(contentObject) + " 0 R >>"
		kids = append(kids, strconv.Itoa(pageObject)+" 0 R")
	}
	objects[2] = "<< /Type /Pages /Kids [" + strings.Join(kids, " ") + "] /Count " + strconv.Itoa(pageCount) + " >>"

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, totalObjects+1)
	for i := 1; i <= totalObjects; i++ {
		offsets[i] = buf.Len()
		buf.WriteString(strconv.Itoa(i))
		buf.WriteString(" 0 obj\n")
		buf.WriteString(objects[i])
		buf.WriteString("\nendobj\n")
	}
	xrefOffset := buf.Len()
	buf.WriteString("xref\n0 ")
	buf.WriteString(strconv.Itoa(totalObjects + 1))
	buf.WriteString("\n0000000000 65535 f \n")
	for i := 1; i <= totalObjects; i++ {
		buf.WriteString(pdfXrefOffset(offsets[i]))
		buf.WriteString(" 00000 n \n")
	}
	buf.WriteString("trailer\n<< /Size ")
	buf.WriteString(strconv.Itoa(totalObjects + 1))
	buf.WriteString(" /Root 1 0 R >>\nstartxref\n")
	buf.WriteString(strconv.Itoa(xrefOffset))
	buf.WriteString("\n%%EOF\n")
	return buf.Bytes(), nil
}

func auditLogsPDFLines(generatedAt time.Time, records []AuditLogResponse) []string {
	lines := []string{
		"AgentHub Audit Logs",
		"Generated at: " + generatedAt.UTC().Format(time.RFC3339),
		"",
		"id | entityType | entityId | action | actorId | actorEmail | ipAddress | createdAt | metadata",
	}
	for _, record := range records {
		lines = append(lines, truncateAuditPDFLine(strings.Join([]string{
			record.ID.String(),
			record.EntityType,
			record.EntityID,
			string(record.Action),
			record.ActorID,
			record.ActorEmail,
			record.IPAddress,
			record.CreatedAt.UTC().Format(time.RFC3339Nano),
			record.Metadata,
		}, " | ")))
	}
	return lines
}

func auditLogsPDFPageContent(lines []string) string {
	var b strings.Builder
	b.WriteString("BT\n/F1 10 Tf\n14 TL\n40 555 Td\n")
	for i, line := range lines {
		if i > 0 {
			b.WriteString("T*\n")
		}
		b.WriteString("(")
		b.WriteString(pdfEscapeText(line))
		b.WriteString(") Tj\n")
	}
	b.WriteString("ET")
	return b.String()
}

func pdfXrefOffset(offset int) string {
	value := strconv.Itoa(offset)
	if len(value) >= 10 {
		return value
	}
	return strings.Repeat("0", 10-len(value)) + value
}

func truncateAuditPDFLine(value string) string {
	const maxRunes = 220
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes-3]) + "..."
}

func pdfEscapeText(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch r {
		case '\\', '(', ')':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\r', '\n', '\t':
			b.WriteByte(' ')
		default:
			if r < 32 || r > 126 {
				b.WriteByte('?')
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

func auditLogsXLSX(generatedAt time.Time, records []AuditLogResponse) ([]byte, error) {
	generated := generatedAt.UTC().Format(time.RFC3339)
	return auditLogsZipBundle(generatedAt, []auditExportZipFile{
		{name: "[Content_Types].xml", body: []byte(auditLogsXLSXContentTypes())},
		{name: "_rels/.rels", body: []byte(auditLogsXLSXRootRelationships())},
		{name: "docProps/core.xml", body: []byte(auditLogsXLSXCoreProperties(generated))},
		{name: "docProps/app.xml", body: []byte(auditLogsXLSXAppProperties())},
		{name: "xl/workbook.xml", body: []byte(auditLogsXLSXWorkbook())},
		{name: "xl/_rels/workbook.xml.rels", body: []byte(auditLogsXLSXWorkbookRelationships())},
		{name: "xl/worksheets/sheet1.xml", body: []byte(auditLogsXLSXSheet(records))},
	})
}

func auditLogsXLSXContentTypes() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
		`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
		`</Types>`
}

func auditLogsXLSXRootRelationships() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>` +
		`</Relationships>`
}

func auditLogsXLSXCoreProperties(generated string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
		`xmlns:dc="http://purl.org/dc/elements/1.1/" ` +
		`xmlns:dcterms="http://purl.org/dc/terms/" ` +
		`xmlns:dcmitype="http://purl.org/dc/dcmitype/" ` +
		`xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
		`<dc:creator>AgentHub</dc:creator>` +
		`<cp:lastModifiedBy>AgentHub</cp:lastModifiedBy>` +
		`<dcterms:created xsi:type="dcterms:W3CDTF">` + xmlEscapeText(generated) + `</dcterms:created>` +
		`<dcterms:modified xsi:type="dcterms:W3CDTF">` + xmlEscapeText(generated) + `</dcterms:modified>` +
		`</cp:coreProperties>`
}

func auditLogsXLSXAppProperties() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" ` +
		`xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">` +
		`<Application>AgentHub</Application>` +
		`</Properties>`
}

func auditLogsXLSXWorkbook() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="Audit Logs" sheetId="1" r:id="rId1"/></sheets>` +
		`</workbook>`
}

func auditLogsXLSXWorkbookRelationships() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
		`</Relationships>`
}

func auditLogsXLSXSheet(records []AuditLogResponse) string {
	rows := make([][]string, 0, len(records)+1)
	rows = append(rows, []string{"id", "entityType", "entityId", "action", "actorId", "actorEmail", "ipAddress", "createdAt", "metadata"})
	for _, record := range records {
		rows = append(rows, []string{
			record.ID.String(),
			record.EntityType,
			record.EntityID,
			string(record.Action),
			record.ActorID,
			record.ActorEmail,
			record.IPAddress,
			record.CreatedAt.UTC().Format(time.RFC3339Nano),
			record.Metadata,
		})
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for rowIndex, row := range rows {
		rowNumber := strconv.Itoa(rowIndex + 1)
		b.WriteString(`<row r="`)
		b.WriteString(rowNumber)
		b.WriteString(`">`)
		for colIndex, value := range row {
			cellRef := xlsxColumnName(colIndex+1) + rowNumber
			b.WriteString(`<c r="`)
			b.WriteString(cellRef)
			b.WriteString(`" t="inlineStr"><is><t>`)
			b.WriteString(xmlEscapeText(value))
			b.WriteString(`</t></is></c>`)
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func xlsxColumnName(index int) string {
	var chars []byte
	for index > 0 {
		index--
		chars = append([]byte{byte('A' + index%26)}, chars...)
		index /= 26
	}
	return string(chars)
}

func xmlEscapeText(value string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(value))
	return buf.String()
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (h *Handler) record(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	var req RecordRequest
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Automatically extract IP if not provided by the caller.
	if req.IPAddress == "" {
		req.IPAddress = ExtractIP(r)
	}

	l, err := h.svc.Record(r.Context(), tenantID, req)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("audit: record failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "record failed")
		return
	}
	respond.JSON(w, http.StatusCreated, ResponseFrom(l))
}

func (h *Handler) applyRetention(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())

	var req AuditRetentionRequest
	if err := httputil.DecodeOptionalSingleJSON(r.Body, &req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.svc.ApplyRetention(r.Context(), tenantID, req, time.Now().UTC())
	if err != nil {
		if errors.Is(err, ErrValidation) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		slog.Error("audit: retention failed", "tenantID", tenantID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "retention failed")
		return
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	l, err := h.svc.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "audit log not found")
			return
		}
		slog.Error("audit: getByID failed", "tenantID", tenantID, "id", id, "err", err)
		respond.Error(w, http.StatusInternalServerError, "get failed")
		return
	}
	respond.JSON(w, http.StatusOK, ResponseFrom(l))
}
