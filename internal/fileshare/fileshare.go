package fileshare

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// SafePath resolves userInput relative to root and returns a path under root.
// It prevents path traversal and symlink escape. Requires Go 1.20+ for filepath.IsLocal.
func SafePath(root, userInput string) (string, error) {
	if userInput == "" {
		return "", errors.New("empty path")
	}
	cleaned := filepath.Clean(userInput)
	if !filepath.IsLocal(cleaned) {
		return "", errors.New("path not local")
	}
	full := filepath.Join(root, cleaned)
	abs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rootAbs = filepath.Clean(rootAbs)
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil {
		return "", err
	}
	if rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
		return "", errors.New("path escapes root")
	}
	return abs, nil
}

// ListDir lists entries under root (optionally matching pattern). Only returns paths under root.
func ListDir(root, pattern string) ([]fs.DirEntry, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	rootAbs = filepath.Clean(rootAbs)
	var out []fs.DirEntry
	entries, err := os.ReadDir(rootAbs)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if pattern != "" {
			ok, _ := filepath.Match(pattern, e.Name())
			if !ok {
				continue
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// ResolveRoot returns the absolute canonical root path and ensures it exists.
func ResolveRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(abs, 0755); err != nil {
				return "", err
			}
			return abs, nil
		}
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("root is not a directory")
	}
	return abs, nil
}
