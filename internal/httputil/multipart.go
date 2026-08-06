package httputil

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// LimitedMultipartOptions configures bounded multipart parsing for one file.
type LimitedMultipartOptions struct {
	FileFields    []string
	MaxFileBytes  int64
	MaxBodyBytes  int64
	MaxFieldBytes int64
}

// LimitedMultipartFile is a bounded multipart file held in memory.
type LimitedMultipartFile struct {
	FieldName   string
	Filename    string
	ContentType string
	Size        int64
	Content     []byte
}

// Reader returns a fresh reader over Content.
func (f LimitedMultipartFile) Reader() io.Reader {
	return bytes.NewReader(f.Content)
}

// ReadLimitedMultipartFile reads a single accepted file field and small text
// fields without using Request.ParseMultipartForm.
func ReadLimitedMultipartFile(w http.ResponseWriter, r *http.Request, opts LimitedMultipartOptions) (LimitedMultipartFile, map[string]string, error) {
	if opts.MaxFileBytes <= 0 {
		return LimitedMultipartFile{}, nil, fmt.Errorf("max file size must be positive")
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = opts.MaxFileBytes
	}
	if opts.MaxFieldBytes <= 0 {
		opts.MaxFieldBytes = 8 << 10
	}
	if len(opts.FileFields) == 0 {
		return LimitedMultipartFile{}, nil, fmt.Errorf("at least one file field is required")
	}

	acceptedFields := make(map[string]struct{}, len(opts.FileFields))
	for _, field := range opts.FileFields {
		acceptedFields[field] = struct{}{}
	}

	r.Body = http.MaxBytesReader(w, r.Body, opts.MaxBodyBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		return LimitedMultipartFile{}, nil, fmt.Errorf("multipart form required: %w", err)
	}

	fields := make(map[string]string)
	var file LimitedMultipartFile
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return LimitedMultipartFile{}, nil, fmt.Errorf("read multipart part: %w", err)
		}

		name := part.FormName()
		if part.FileName() == "" {
			data, readErr := readLimitedPart(part, opts.MaxFieldBytes)
			closeErr := part.Close()
			if readErr != nil {
				return LimitedMultipartFile{}, nil, fmt.Errorf("read multipart field %q: %w", name, readErr)
			}
			if closeErr != nil {
				return LimitedMultipartFile{}, nil, fmt.Errorf("close multipart field %q: %w", name, closeErr)
			}
			if _, exists := fields[name]; !exists {
				fields[name] = string(data)
			}
			continue
		}

		if _, ok := acceptedFields[name]; !ok || file.Filename != "" {
			if err := part.Close(); err != nil {
				return LimitedMultipartFile{}, nil, fmt.Errorf("close ignored multipart file %q: %w", name, err)
			}
			continue
		}

		data, readErr := readLimitedPart(part, opts.MaxFileBytes)
		closeErr := part.Close()
		if readErr != nil {
			return LimitedMultipartFile{}, nil, fmt.Errorf("read multipart file %q: %w", name, readErr)
		}
		if closeErr != nil {
			return LimitedMultipartFile{}, nil, fmt.Errorf("close multipart file %q: %w", name, closeErr)
		}
		file = LimitedMultipartFile{
			FieldName:   name,
			Filename:    part.FileName(),
			ContentType: part.Header.Get("Content-Type"),
			Size:        int64(len(data)),
			Content:     data,
		}
	}

	if file.Filename == "" {
		return LimitedMultipartFile{}, nil, fmt.Errorf("file field is required")
	}
	return file, fields, nil
}

func readLimitedPart(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("part exceeds %d bytes", limit)
	}
	return data, nil
}
