package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The songs that have been read in and played, newest first. It is a file of
// its own beside the config because a song can be in it without being on disk:
// the app remembers which version of a tab it read even after the file is
// removed, and that memory is what the search screen opens on.

// keep is how many are remembered. A list longer than a screen answers no
// question the search does not answer better.
const keep = 100

// Entry is one song. URL is the page it was read from and File is where it was
// written, and either can be empty: a song read in and then removed keeps the
// page it came from, which is what makes it findable again.
type Entry struct {
	Artist  string    `json:"artist"`
	Title   string    `json:"title"`
	Version int       `json:"version,omitempty"`
	URL     string    `json:"url,omitempty"`
	File    string    `json:"file,omitempty"`
	At      time.Time `json:"at"`
}

type Recent struct {
	Entries []Entry `json:"entries"`
}

func recentPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "recent.json"), nil
}

// LoadRecent reads the list. A file that is not there is an empty list, which
// is what a first run has.
func LoadRecent() Recent {
	var loaded Recent

	path, err := recentPath()
	if err != nil {
		return loaded
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return loaded
	}
	if err := json.Unmarshal(data, &loaded); err != nil {
		return Recent{}
	}

	sort.SliceStable(loaded.Entries, func(i, j int) bool {
		return loaded.Entries[i].At.After(loaded.Entries[j].At)
	})

	return loaded
}

func (r *Recent) Save() error {
	path, err := recentPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// Remember puts a song at the top of the list, or moves it there when it is
// already on it. What the new entry does not carry is kept from the old one:
// practising a song says nothing about which page it came from.
func (r *Recent) Remember(entry Entry) {
	if entry.At.IsZero() {
		entry.At = time.Now()
	}

	kept := make([]Entry, 0, len(r.Entries)+1)
	for _, old := range r.Entries {
		if !same(old, entry) {
			kept = append(kept, old)
			continue
		}
		if entry.URL == "" {
			entry.URL = old.URL
		}
		if entry.File == "" {
			entry.File = old.File
		}
		if entry.Version == 0 {
			entry.Version = old.Version
		}
	}

	r.Entries = append([]Entry{entry}, kept...)
	if len(r.Entries) > keep {
		r.Entries = r.Entries[:keep]
	}
}

// Forget drops a song. It is what d does, and the entry goes whether or not
// the file it names is still there.
func (r *Recent) Forget(entry Entry) {
	kept := make([]Entry, 0, len(r.Entries))
	for _, old := range r.Entries {
		if !same(old, entry) {
			kept = append(kept, old)
		}
	}
	r.Entries = kept
}

// same is when two entries are the one song. The page and the file each say so
// on their own, since a song is read in before it has a file and keeps its
// file after the page is forgotten.
func same(a, b Entry) bool {
	if a.URL != "" && a.URL == b.URL {
		return true
	}
	if a.File != "" && a.File == b.File {
		return true
	}
	return strings.EqualFold(a.Artist, b.Artist) && strings.EqualFold(a.Title, b.Title) &&
		a.Version == b.Version
}
