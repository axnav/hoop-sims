package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SavePath is where the web game keeps its save, relative to the working directory.
const SavePath = "saves/league.json"

// SaveVersion is the current save format. Bump it and append one step to
// saveUpgrades whenever a saved field is added or changes meaning.
const SaveVersion = 5

// saveUpgrades[i] upgrades a league from version i to version i+1.
var saveUpgrades = []func(*League){
	// 0 -> 1: saves from before versioning; make sure Careers exists.
	func(lg *League) {
		if lg.Careers == nil {
			lg.Careers = map[int][]CareerRow{}
		}
	},
	// 1 -> 2: draft picks became owned objects; give every team its own picks.
	func(lg *League) {
		if len(lg.Picks) == 0 {
			lg.initPicks()
		}
	},
	// 2 -> 3: contracts and the salary cap; give everyone a contract if none has one.
	func(lg *League) {
		for _, t := range lg.Teams {
			for _, p := range t.Roster {
				if p.Contract.Years > 0 {
					return
				}
			}
		}
		lg.assignInitialContracts()
	},
	// 3 -> 4: coaches and systems; give every team a generated coach (the
	// system stays "", the coach's favourite).
	func(lg *League) { lg.initCoaches() },
	// 4 -> 5: detailed attributes and tendencies; give every player a set by
	// the fictional rule (deterministic: hashed from name and ID, stream 160000).
	func(lg *League) { lg.ensureAttributes() },
}

// Save writes the whole league as JSON, creating the folder if needed.
func (lg *League) Save(path string) error {
	lg.Version = SaveVersion
	for _, t := range lg.Teams {
		t.pruneLineup()
	}
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
	if lg.Version > SaveVersion {
		return nil, fmt.Errorf("save is version %d but this game only understands up to version %d: it was made by a newer build", lg.Version, SaveVersion)
	}
	if lg.Version < 0 {
		return nil, fmt.Errorf("save has invalid version %d", lg.Version)
	}
	for v := lg.Version; v < SaveVersion; v++ {
		saveUpgrades[v](lg)
	}
	lg.Version = SaveVersion
	return lg, nil
}
