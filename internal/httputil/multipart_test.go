package httputil

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadLimitedMultipartFilePreservesFileAndFields(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "asset.tgz")
	require.NoError(t, err)
	_, err = part.Write([]byte("asset-content"))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("versionId", "version-a"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	file, fields, err := ReadLimitedMultipartFile(httptest.NewRecorder(), req, LimitedMultipartOptions{
		FileFields:    []string{"file"},
		MaxFileBytes:  64,
		MaxBodyBytes:  1024,
		MaxFieldBytes: 64,
	})

	require.NoError(t, err)
	assert.Equal(t, "file", file.FieldName)
	assert.Equal(t, "asset.tgz", file.Filename)
	assert.Equal(t, int64(len("asset-content")), file.Size)
	content, err := io.ReadAll(file.Reader())
	require.NoError(t, err)
	assert.Equal(t, []byte("asset-content"), content)
	assert.Equal(t, "version-a", fields["versionId"])
}

func TestReadLimitedMultipartFileRejectsOversizedFile(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "large.bin")
	require.NoError(t, err)
	_, err = part.Write([]byte("12345"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, _, err = ReadLimitedMultipartFile(httptest.NewRecorder(), req, LimitedMultipartOptions{
		FileFields:    []string{"file"},
		MaxFileBytes:  4,
		MaxBodyBytes:  1024,
		MaxFieldBytes: 64,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "part exceeds 4 bytes")
}

func TestReadLimitedMultipartFileRejectsMissingFile(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("versionId", "version-a"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, _, err := ReadLimitedMultipartFile(httptest.NewRecorder(), req, LimitedMultipartOptions{
		FileFields:    []string{"file"},
		MaxFileBytes:  64,
		MaxBodyBytes:  1024,
		MaxFieldBytes: 64,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "file field is required")
}
