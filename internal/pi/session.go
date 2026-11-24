package pi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Session is a conversation pi saved, as a file of JSON lines.
type Session struct {
	Path    string
	ID      string
	Name    string    // the name set for it, "" for none
	First   string    // the text of its first message of the user
	Created time.Time // when it started
	Updated time.Time // when it was last written
	size    int64
}

// Sessions lists the conversations saved in dir and the directories in
// it, the last written first. It reads only the files written since the
// last call with the same cache.
func Sessions(dir string, cache *SessionCache) []Session {
	var paths []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".jsonl") {
			paths = append(paths, p)
		}
		return nil
	})
	if cache == nil {
		cache = &SessionCache{}
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.m == nil {
		cache.m = map[string]Session{}
	}
	var out []Session
	seen := map[string]bool{}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		seen[p] = true
		s, ok := cache.m[p]
		if !ok || !s.Updated.Equal(st.ModTime()) || s.size != st.Size() {
			s, ok = readSession(p)
			if !ok {
				continue
			}
			s.Updated, s.size = st.ModTime(), st.Size()
			cache.m[p] = s
		}
		out = append(out, s)
	}
	for p := range cache.m {
		if !seen[p] {
			delete(cache.m, p)
		}
	}
	slices.SortFunc(out, func(a, b Session) int { return b.Updated.Compare(a.Updated) })
	return out
}

// SessionCache keeps what Sessions read of the files.
type SessionCache struct {
	mu sync.Mutex
	m  map[string]Session
}

// readSession reads a session's header, its name and its first message of
// the user.
func readSession(path string) (Session, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Session{}, false
	}
	defer f.Close()
	s := Session{Path: path}
	r := bufio.NewReaderSize(f, 64<<10)
	first := true
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			switch {
			case first:
				var h struct {
					Type      string    `json:"type"`
					ID        string    `json:"id"`
					Timestamp time.Time `json:"timestamp"`
				}
				if json.Unmarshal(line, &h) != nil || h.Type != "session" {
					return Session{}, false
				}
				s.ID, s.Created = h.ID, h.Timestamp
				first = false
			case bytes.Contains(line, []byte(`"type":"session_info"`)):
				var e struct {
					Name string `json:"name"`
				}
				json.Unmarshal(line, &e)
				s.Name = e.Name
			case s.First == "" && bytes.Contains(line, []byte(`"role":"user"`)):
				var e struct {
					Type    string  `json:"type"`
					Message Message `json:"message"`
				}
				if json.Unmarshal(line, &e) == nil && e.Type == "message" && e.Message.Role == "user" {
					s.First = e.Message.Text()
				}
			}
		}
		if err != nil {
			break
		}
	}
	return s, !first
}
