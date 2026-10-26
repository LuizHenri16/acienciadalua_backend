package utils

import (
	"fmt"
	"io"
	"math/rand/v2"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	coverExtensions = map[string]bool{".jpg": true, ".jpeg": true, ".png": true}
	fileExtensions  = map[string]bool{".pdf": true, ".zip": true}
)

// EnsureStorageDirs creates the upload/download directories, like the
// mkdirSync calls at the top of the Nest ProductsController.
func EnsureStorageDirs(uploadDir, filesDir string) error {
	for _, dir := range []string{uploadDir, filesDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	return nil
}

// ValidateUploadExtension enforces the same extension allow-list used by the
// multer fileFilter in the NestJS controller.
func ValidateUploadExtension(fieldName, originalName string) error {
	extension := strings.ToLower(filepath.Ext(originalName))

	allowed := fileExtensions
	if fieldName == "cover" {
		allowed = coverExtensions
	}

	if allowed[extension] {
		return nil
	}

	return BadRequest(fmt.Sprintf(
		`Tipo de arquivo não permitido para "%s": %s`,
		fieldName,
		originalName,
	))
}

// GenerateFileName mirrors multer's
// `${Date.now()}-${Math.round(Math.random() * 1e9)}${extname(originalname)}`.
func GenerateFileName(originalName string) string {
	return fmt.Sprintf("%d-%d%s", time.Now().UnixMilli(), rand.IntN(1_000_000_000), filepath.Ext(originalName))
}

// SaveUploadedFile writes the uploaded file to dir and returns its stored
// filename (which is what the database keeps in coverUrl/fileUrl).
func SaveUploadedFile(dir string, file multipart.File, originalName string) (string, error) {
	name := GenerateFileName(originalName)

	destination, err := os.Create(filepath.Join(dir, filepath.Base(name)))
	if err != nil {
		return "", fmt.Errorf("creating upload file: %w", err)
	}
	defer destination.Close()

	if _, err := io.Copy(destination, file); err != nil {
		return "", fmt.Errorf("writing upload file: %w", err)
	}

	return name, nil
}

// DownloadContentType returns the Content-Type used by the download endpoint.
func DownloadContentType(fileURL string) string {
	switch strings.ToLower(filepath.Ext(fileURL)) {
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

// SafeDownloadName sanitizes the filename for the Content-Disposition header
// exactly like `fileUrl.replace(/[^\w.-]/g, '_')`.
func SafeDownloadName(fileURL string) string {
	var builder strings.Builder
	for _, r := range fileURL {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_', r == '.', r == '-':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	return builder.String()
}
