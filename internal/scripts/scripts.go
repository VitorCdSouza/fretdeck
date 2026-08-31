// Package scripts carries the python side inside the binary.
//
// It is embedded and unpacked at startup instead of being looked up on disk,
// because the three files import each other and a build that only works when
// run from the source folder is a build that breaks the first time it is
// copied somewhere.
package scripts

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
)

//go:embed *.py
var files embed.FS

// Unpack writes the scripts under the user cache folder and answers where.
//
// The folder is named after the hash of what is in it, so a new build never
// runs against the files the old one left behind.
func Unpack() (string, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return "", err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	sum := sha256.New()
	contents := make(map[string][]byte, len(names))
	for _, name := range names {
		data, err := files.ReadFile(name)
		if err != nil {
			return "", err
		}
		contents[name] = data
		sum.Write([]byte(name))
		sum.Write(data)
	}

	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(cache, "fretdeck", "scripts-"+hex.EncodeToString(sum.Sum(nil))[:12])
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	for name, data := range contents {
		path := filepath.Join(dir, name)
		if existing, err := os.ReadFile(path); err == nil && len(existing) == len(data) {
			continue
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return "", err
		}
	}

	return dir, nil
}
