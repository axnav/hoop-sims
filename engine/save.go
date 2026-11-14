package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SavePath is where the web game keeps its save, relative to the working directory.
const SavePath = "saves/league.json"

// Save writes the whole league as JSON, creating the folder if needed.
func (lg *League) Save(path string) error {
	data, err := json.Marshal(lg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load reads a league saved with Save.
func Load(path string) (*League, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lg := &League{}
	if err := json.Unmarshal(data, lg); err != nil {
		return nil, err
	}
	if lg.Careers == nil {
		lg.Careers = map[int][]CareerRow{}
	}
	return lg, nil
}
