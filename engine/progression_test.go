package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProgressRecordsDevChange(t *testing.T) {
	lg := GenerateLeague(5)
	p := lg.Teams[0].Roster[0]
	if p.Dev != nil {
		t.Fatal("new player already has Dev")
	}
	oo, op, or := p.Overall(), p.Potential, p.Ratings.array()
	p.Progress(NewRNG(99))
	if p.Dev == nil {
		t.Fatal("no Dev after Progress")
	}
	if p.Dev.Overall != p.Overall()-oo || p.Dev.Potential != p.Potential-op {
		t.Errorf("dev ovr/pot %+v, want %d/%d", p.Dev, p.Overall()-oo, p.Potential-op)
	}
	n, d := p.Ratings.array(), p.Dev.Ratings.array()
	for i := range n {
		if d[i] != n[i]-or[i] {
			t.Errorf("rating %d change %d, want %d", i, d[i], n[i]-or[i])
		}
	}

	// Round trip keeps it; a player without Dev has no field in the file.
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	g := got.FindPlayer(p.ID)
	if g == nil || g.Dev == nil || *g.Dev != *p.Dev {
		t.Fatalf("Dev lost in round trip: %+v", g)
	}
	if g2 := got.Teams[0].Roster[1]; g2.Dev != nil {
		t.Fatal("untouched player gained Dev")
	}
	b, _ := os.ReadFile(path)
	if c := strings.Count(string(b), `"Dev"`); c != 1 {
		t.Fatalf("Dev appears %d times in save, want 1", c)
	}
}
