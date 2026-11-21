package engine

import (
	"path/filepath"
	"testing"
)

// potentialDigest sums overall and potential over every player, weighted by ID,
// so any change in an unedited player shows up.
func potentialDigest(lg *League) (int, int) {
	o, p := 0, 0
	for _, pl := range lg.allPlayers() {
		o += pl.Overall() * (pl.ID%97 + 1)
		p += pl.Potential * (pl.ID%89 + 1)
	}
	return o, p
}

func TestUneditedPlayersUnchanged(t *testing.T) {
	lg := GenerateLeague(7)
	playSeason(lg)
	lg.Offseason()
	o, p := potentialDigest(lg)
	t.Logf("digest %d %d", o, p)
	if o != 1231702 || p != 1231626 {
		t.Fatalf("unedited players changed: got %d %d", o, p)
	}
}

func editedPlayer(t *testing.T, lg *League, age, pot int) *Player {
	lg.Options.Commissioner = true
	var best *Player
	for _, p := range lg.allPlayers() {
		if p.Overall() >= 70 && p.Overall() <= 80 && (best == nil || p.Overall() > best.Overall()) {
			best = p
		}
	}
	if best == nil {
		t.Fatal("no player")
	}
	if err := lg.EditPlayer(best.ID, PlayerEdit{Age: &age, Potential: &pot}); err != nil {
		t.Fatal(err)
	}
	return best
}

func TestEditedPotentialOldPlayerGrowsToIt(t *testing.T) {
	lg := GenerateLeague(7)
	p := editedPlayer(t, lg, 28, 92)
	id, start := p.ID, p.Overall()
	if p.EditedCeiling != 92 {
		t.Fatal("ceiling not recorded")
	}
	playSeason(lg)
	if p.Potential != 92 || p.EditedCeiling != 92 {
		t.Fatalf("after in-season steps potential %d ceiling %d", p.Potential, p.EditedCeiling)
	}
	lg.Offseason()
	if p.Overall() <= start {
		t.Fatalf("overall did not rise: %d -> %d", start, p.Overall())
	}
	reached, peak := false, 0
	for y := 0; y < 12 && lg.findPlayer(id) != nil; y++ {
		peak = max(peak, p.Overall())
		if p.EditedCeiling == 0 {
			reached = true
			break
		}
		if p.Potential != 92 {
			t.Fatalf("year %d potential %d", y, p.Potential)
		}
		playSeason(lg)
		lg.Offseason()
	}
	if !reached {
		t.Fatalf("never reached the ceiling, overall %d", p.Overall())
	}
	// He may have reached 92 mid-season and aged a little since.
	if peak < 90 && p.Overall() < 88 {
		t.Fatalf("cleared early: peak %d overall %d", peak, p.Overall())
	}
	if p.Potential != p.Overall() && p.Age >= Progression.PotentialEqualAge {
		t.Fatalf("after reaching it potential %d overall %d", p.Potential, p.Overall())
	}
}

func TestEditedPotentialYoungKeepsIt(t *testing.T) {
	lg := GenerateLeague(7)
	p := editedPlayer(t, lg, 22, 95)
	playSeason(lg)
	lg.Offseason()
	if p.Potential != 95 || p.EditedCeiling != 95 {
		t.Fatalf("potential %d ceiling %d", p.Potential, p.EditedCeiling)
	}
}

func TestEditedCeilingClearedWhenOverallPassesIt(t *testing.T) {
	lg := GenerateLeague(7)
	p := editedPlayer(t, lg, 28, 92)
	ov := 95
	if err := lg.EditPlayer(p.ID, PlayerEdit{Overall: &ov}); err != nil {
		t.Fatal(err)
	}
	if p.EditedCeiling != 0 {
		t.Fatalf("ceiling %d should be cleared", p.EditedCeiling)
	}
}

func TestEditedCeilingSaved(t *testing.T) {
	lg := GenerateLeague(7)
	p := editedPlayer(t, lg, 28, 92)
	path := filepath.Join(t.TempDir(), "s.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	lg2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if q := lg2.findPlayer(p.ID); q == nil || q.EditedCeiling != 92 {
		t.Fatal("edited ceiling lost in save")
	}
}
