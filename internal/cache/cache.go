package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

func dir() string {
	return filepath.Join(os.Getenv("HOME"), ".cache", "vetpkg")
}

func Hash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// GetApprovedHash returns the stored hash for pkgname, or "" if not cached.
func GetApprovedHash(pkgname string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir(), pkgname+".sha256"))
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

// GetApprovedContent returns the stored PKGBUILD content for pkgname, or "" if not cached.
func GetApprovedContent(pkgname string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir(), pkgname+".pkgbuild"))
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

// SaveApproved persists both the hash and full content of an approved PKGBUILD.
func SaveApproved(pkgname, content string) error {
	d := dir()
	if err := os.MkdirAll(d, 0700); err != nil {
		return err
	}
	hash := Hash(content)
	if err := os.WriteFile(filepath.Join(d, pkgname+".sha256"), []byte(hash), 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, pkgname+".pkgbuild"), []byte(content), 0600)
}
