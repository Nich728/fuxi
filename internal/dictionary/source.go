package dictionary

import (
	"compress/bzip2"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// fileSHA256 hashes a file's raw bytes, so an import can be skipped when the
// same snapshot was already loaded (SYSTEM_DESIGN §5 step 1, "Pin").
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// combinedSHA256 identifies an import made from several files.
func combinedSHA256(paths ...string) (string, error) {
	h := sha256.New()
	for _, p := range paths {
		sum, err := fileSHA256(p)
		if err != nil {
			return "", err
		}
		h.Write([]byte(sum))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// openData opens a dataset file, decompressing .gz and .bz2 transparently.
func openData(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	switch {
	case strings.HasSuffix(path, ".gz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("gunzip %s: %w", path, err)
		}
		return readCloser{gz, func() error { _ = gz.Close(); return f.Close() }}, nil
	case strings.HasSuffix(path, ".bz2"):
		return readCloser{bzip2.NewReader(f), f.Close}, nil
	default:
		return f, nil
	}
}

type readCloser struct {
	io.Reader
	close func() error
}

func (r readCloser) Close() error { return r.close() }
