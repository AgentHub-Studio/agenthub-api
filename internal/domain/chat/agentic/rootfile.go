package agentic

import (
	"io"
	"os"
	"path/filepath"
)

func readFileFromRoot(rootDir, name string) ([]byte, error) {
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()

	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return io.ReadAll(file)
}

func readFileScopedToParent(path string) ([]byte, error) {
	cleanPath := filepath.Clean(path)
	dir, name := filepath.Split(cleanPath)
	if dir == "" {
		dir = "."
	}
	return readFileFromRoot(dir, name)
}
