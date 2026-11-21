package engine

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"testing"
)

// gamesDigest plays a full season, the playoffs and the offseason on
// GenerateLeague(7) and fingerprints the results.
func gamesDigest() (pts, wins, ovr string) {
	lg := GenerateLeague(7)
	lg.UserTeamID = 0
	lg.StartSeason()
	lg.SimRegularSeason()
	_ = lg.SimPlayoffs()
	h := fnv.New64a()
	hw := fnv.New64a()
	total := 0
	for _, ps := range lg.Season.Stats {
		total += ps.Pts
	}
	for _, ps := range lg.Season.PlayoffStats {
		total += ps.Pts
	}
	for _, d := range lg.Season.PointDiff {
		fmt.Fprintf(hw, "d%d,", d)
	}
	fmt.Fprintf(h, "%d", total)
	for _, w := range lg.Season.Wins {
		fmt.Fprintf(hw, "%d,", w)
	}
	lg.Offseason()
	ho := fnv.New64a()
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			fmt.Fprintf(ho, "%d:%d:%d,", p.ID, p.Overall(), p.Potential)
		}
	}
	return fmt.Sprintf("%d/%x", total, h.Sum64()), fmt.Sprintf("%x", hw.Sum64()), fmt.Sprintf("%x", ho.Sum64())
}

func TestAttributesDoNotChangeGames(t *testing.T) {
	pts, wins, ovr := gamesDigest()
	t.Logf("DIGEST pts=%s wins=%s ovr=%s", pts, wins, ovr)
	// Recorded on the code before attributes existed (GenerateLeague(7)).
	if pts != "292142/d29f75a42f4b644f" || wins != "b79ee7097f57e7fc" || ovr != "c228fc9929d5a696" {
		t.Fatalf("games changed: pts=%s wins=%s ovr=%s", pts, wins, ovr)
	}
}

func checkAttrSet(t *testing.T, p *Player) {
	t.Helper()
	if !p.hasAttributes() {
		t.Fatalf("%s has no tendencies", p.Name)
	}
	for _, a := range p.Attributes() {
		if a.Value < 25 || a.Value > 99 {
			t.Fatalf("%s %s = %d", p.Name, a.Key, a.Value)
		}
	}
	for _, v := range p.TendencyValues() {
		if v.Value < 0 || v.Value > 100 {
			t.Fatalf("%s tendency %s = %d", p.Name, v.Key, v.Value)
		}
	}
}

func TestAttributeWeightsAndOffsets(t *testing.T) {
	var gs [7]float64
	for i, w := range AttrTuning.Weights {
		gs[attrKeys[i].GroupIdx] += w
	}
	for g, s := range gs {
		if s < 0.999 || s > 1.001 {
			t.Errorf("group %d weights sum to %v", g, s)
		}
	}
	lg := GenerateLeague(5)
	n, big := 0, 0
	for _, p := range lg.allPlayers() {
		checkAttrSet(t, p)
		for _, gb := range groupBounds {
			m := 0.0
			for i := gb[0]; i < gb[1]; i++ {
				m += AttrTuning.Weights[i] * float64(p.AttrOffsets[i])
			}
			if m < -0.5 || m > 0.5 {
				t.Fatalf("%s group %v weighted mean %v", p.Name, gb, m)
			}
		}
		for _, o := range p.AttrOffsets {
			n++
			if o < -25 || o > 25 { // the cap plus the rounding fix
				big++
			}
		}
	}
	if big != 0 {
		t.Errorf("%d of %d offsets beyond the cap plus rounding", big, n)
	}
}

func TestAttributeShapeByPosition(t *testing.T) {
	lg := GenerateLeague(5)
	var dunk, speed [5]float64
	var cnt [5]float64
	for _, p := range lg.allPlayers() {
		dunk[p.Pos] += float64(p.AttrOf(AttrDunk))
		speed[p.Pos] += float64(p.AttrOf(AttrSpeed))
		cnt[p.Pos]++
	}
	for i := range cnt {
		dunk[i] /= cnt[i]
		speed[i] /= cnt[i]
	}
	if dunk[C] <= dunk[PG] || speed[PG] <= speed[C] {
		t.Errorf("shape wrong: dunk PG %.1f C %.1f, speed PG %.1f C %.1f", dunk[PG], dunk[C], speed[PG], speed[C])
	}
}

func TestAttributesDeterministic(t *testing.T) {
	a, b := GenerateLeague(9), GenerateLeague(9)
	for i, p := range a.allPlayers() {
		q := b.allPlayers()[i]
		if p.AttrOffsets != q.AttrOffsets || p.Tendencies != q.Tendencies {
			t.Fatal("not deterministic")
		}
	}
}

// A version 4 save has no attribute fields: load must give everyone a set,
// the same as the fictional rule, and a round trip keeps them.
func TestLoadVersion4SaveGetsAttributes(t *testing.T) {
	lg := GenerateLeague(7)
	want := map[int]Player{}
	for _, p := range lg.allPlayers() {
		want[p.ID] = *p
	}
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	var strip func(v any)
	strip = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			delete(x, "AttrOffsets")
			delete(x, "Tendencies")
			for _, c := range x {
				strip(c)
			}
		case []any:
			for _, c := range x {
				strip(c)
			}
		}
	}
	strip(m)
	m["version"] = 4
	out, _ := json.Marshal(m)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != SaveVersion || SaveVersion != 5 {
		t.Fatalf("version %d", got.Version)
	}
	for _, p := range got.allPlayers() {
		checkAttrSet(t, p)
		// The rule is deterministic: rebuilding from the same player matches.
		q := want[p.ID]
		q.AttrOffsets, q.Tendencies = [NumAttrs]int{}, [NumTendency]int{}
		fictionalAttributes(&q)
		if p.AttrOffsets != q.AttrOffsets || p.Tendencies != q.Tendencies {
			t.Fatalf("%s: upgrade differs from the rule", p.Name)
		}
	}
	if err := got.Save(path); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range again.allPlayers() {
		if p.AttrOffsets != got.allPlayers()[i].AttrOffsets || p.Tendencies != got.allPlayers()[i].Tendencies {
			t.Fatal("round trip lost attributes")
		}
	}
}
