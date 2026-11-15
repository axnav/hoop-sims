package main

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"hoopsim/engine"
)

var (
	tagRe  = regexp.MustCompile(`<[^>]*>`)
	fileRe = regexp.MustCompile(`(?i)File:[^:\n]*\.(jpe?g|png|tiff?|gif)\s*:\s*`)
	wsRe   = regexp.MustCompile(`\s*\n\s*`)
)

// cleanArtist turns the Commons author field into one short line: no HTML
// tags, no "File:Something.jpg:" prefixes, line breaks joined with "; ".
func cleanArtist(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	s = fileRe.ReplaceAllString(s, "")
	return strings.TrimSpace(wsRe.ReplaceAllString(strings.TrimSpace(s), "; "))
}

// loadPhotos reads the photo folder once and keeps the set of available files.
// A missing folder gives an empty set. Call with s.mu held (or before serving).
func (s *server) loadPhotos() {
	s.photos = map[string]string{}
	s.credits = map[string][3]string{}
	if s.photoDir == "" {
		return
	}
	ents, err := os.ReadDir(s.photoDir)
	if err != nil {
		return
	}
	for _, e := range ents {
		n := e.Name()
		ext := strings.ToLower(filepath.Ext(n))
		if e.IsDir() || (ext != ".jpg" && ext != ".png") {
			continue
		}
		s.photos[strings.TrimSuffix(n, filepath.Ext(n))] = n
	}
	f, err := os.Open(filepath.Join(s.photoDir, "credits.csv"))
	if err != nil {
		return
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil || len(rows) < 2 {
		return
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.TrimSpace(h)] = i
	}
	get := func(r []string, k string) string {
		if i, ok := col[k]; ok && i < len(r) {
			return strings.TrimSpace(r[i])
		}
		return ""
	}
	for _, r := range rows[1:] {
		name := get(r, "file")
		if name == "" {
			continue
		}
		artist := cleanArtist(get(r, "artist"))
		s.credits[name] = [3]string{artist, get(r, "licence"), get(r, "descriptionurl")}
	}
}

// addPhoto marks a player row when a photo file exists for his RealID.
// It also adds "badgeCount", since every roster-style row goes through here.
func (s *server) addPhoto(row M, p *engine.Player) {
	s.addBadgeCount(row, p)
	if p == nil || p.RealID == "" {
		return
	}
	if _, ok := s.photos[p.RealID]; ok {
		row["photo"] = true
	}
}

// photoName finds the cached file name for a player id; "" when none.
// Call with s.mu held.
func (s *server) photoName(idText string) string {
	id, err := strconv.Atoi(idText)
	if err != nil || s.lg == nil {
		return ""
	}
	p := s.lg.FindPlayer(id)
	if p == nil || p.RealID == "" {
		return ""
	}
	return s.photos[p.RealID]
}

func (s *server) photoFile(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	name := s.photoName(r.PathValue("id"))
	dir := s.photoDir
	s.mu.Unlock()
	if name == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, filepath.Join(dir, name))
}

func (s *server) photoCredit(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	name := s.photoName(r.PathValue("id"))
	c, ok := s.credits[name]
	s.mu.Unlock()
	if name == "" || !ok {
		writeJSON(w, 404, M{"error": "no credit"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	json.NewEncoder(w).Encode(M{"artist": c[0], "licence": c[1], "source": c[2]})
}
