package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// rewriteVersion edits the version key of a save file (nil removes it).
func rewriteVersion(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if v == nil {
		delete(m, "version")
	} else {
		m["version"] = v
	}
	out, _ := json.Marshal(m)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	lg := GenerateLeague(7)
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != SaveVersion {
		t.Errorf("version %d, want %d", got.Version, SaveVersion)
	}
	if len(got.Teams) != len(lg.Teams) || got.Seed != lg.Seed || got.Year != lg.Year {
		t.Errorf("basic fields differ after round trip")
	}
	if got.Teams[0].Roster[0].Name != lg.Teams[0].Roster[0].Name {
		t.Errorf("player name differs after round trip")
	}
}

func TestLoadOldSaveWithoutVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "league.json")
	if err := GenerateLeague(7).Save(path); err != nil {
		t.Fatal(err)
	}
	rewriteVersion(t, path, nil)
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != SaveVersion || got.Careers == nil {
		t.Errorf("old save not upgraded: version %d", got.Version)
	}
}

func TestLoadNewerSaveFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "league.json")
	if err := GenerateLeague(7).Save(path); err != nil {
		t.Fatal(err)
	}
	rewriteVersion(t, path, SaveVersion+1)
	if _, err := Load(path); err == nil {
		t.Error("expected error for newer save version")
	}
}

func TestLoadVersion2SaveGetsContracts(t *testing.T) {
	lg := GenerateLeague(7)
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			p.Contract = Contract{} // a save from before contracts
		}
	}
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	rewriteVersion(t, path, 2)
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != SaveVersion {
		t.Errorf("version %d", got.Version)
	}
	for _, tm := range got.Teams {
		if got.Payroll(tm.ID) > Cap.TaxLine {
			t.Errorf("team %d starts over the tax line", tm.ID)
		}
		for _, p := range tm.Roster {
			if p.Contract.Years < 1 || p.Contract.Salary < Cap.MinSalary {
				t.Fatalf("player %d has no contract after upgrade", p.ID)
			}
		}
	}
}
