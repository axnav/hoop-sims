package engine

import (
	"path/filepath"
	"testing"
)

func draftLeague(seed uint64) *League {
	lg := finishedSeasonLeague(seed)
	lg.BeginOffseason()
	return lg
}

func TestScoutRanges(t *testing.T) {
	lg := draftLeague(7)
	d := lg.Draft
	off, total := 0, 0
	for lv := 0; lv <= MaxScoutLevel; lv++ {
		for _, p := range d.Pool {
			d.ScoutLevel[p.ID] = lv
			v, ok := lg.ScoutView(p.ID)
			if !ok || v.Level != lv {
				t.Fatalf("view %+v ok=%v", v, ok)
			}
			if p.Overall() < v.OvrLo || p.Overall() > v.OvrHi || p.Potential < v.PotLo || p.Potential > v.PotHi || v.PotLo < v.OvrLo {
				t.Fatalf("level %d: truth %d/%d outside %+v", lv, p.Overall(), p.Potential, v)
			}
			if w := ScoutTuning.OvrHalf[lv]; v.OvrHi-v.OvrLo > 2*w {
				t.Fatalf("range too wide %+v", v)
			}
			rr, _ := lg.ScoutRatings(p.ID)
			for _, k := range RatingKeys() {
				truth, _ := GetRating(p.Ratings, k.Key)
				if b := rr[k.Key]; truth < b[0] || truth > b[1] {
					t.Fatalf("rating %s truth %d outside %v", k.Key, truth, b)
				}
			}
			if lv == 0 {
				total++
				if (v.OvrLo+v.OvrHi)/2 != p.Overall() {
					off++
				}
			}
		}
	}
	if off*2 < total {
		t.Errorf("only %d of %d level-0 centres differ from the truth", off, total)
	}
	// Example for the report.
	p := d.Pool[0]
	for lv := 0; lv <= 3; lv++ {
		d.ScoutLevel[p.ID] = lv
		v, _ := lg.ScoutView(p.ID)
		t.Logf("true %d/%d level %d: ovr %d-%d pot %d-%d", p.Overall(), p.Potential, lv, v.OvrLo, v.OvrHi, v.PotLo, v.PotHi)
	}
}

func TestScoutSpendAndErrors(t *testing.T) {
	lg := GenerateLeague(3)
	if lg.Scout(1) == nil {
		t.Error("no draft should be an error")
	}
	lg = draftLeague(3)
	if lg.ScoutPointsLeft() != 12 {
		t.Fatalf("points %d", lg.ScoutPointsLeft())
	}
	if lg.Scout(-5) == nil {
		t.Error("unknown prospect should fail")
	}
	id := lg.Draft.Pool[0].ID
	for i := 0; i < 3; i++ {
		if err := lg.Scout(id); err != nil {
			t.Fatal(err)
		}
	}
	if lg.Scout(id) == nil {
		t.Error("level 3 should be a limit")
	}
	for i := 1; lg.ScoutPointsLeft() > 0; i++ {
		if err := lg.Scout(lg.Draft.Pool[i].ID); err != nil {
			t.Fatal(err)
		}
	}
	if lg.Scout(lg.Draft.Pool[20].ID) == nil {
		t.Error("out of points should fail")
	}
	// Drafted prospects cannot be scouted or viewed.
	lg.DraftPick(id)
	if lg.Scout(id) == nil {
		t.Error("drafted prospect should fail")
	}
	if _, ok := lg.ScoutView(id); ok {
		t.Error("drafted prospect has no view")
	}
}

func TestComputerDraftNoise(t *testing.T) {
	lg := draftLeague(8)
	best, bestScore := 0, -1.0
	for _, p := range lg.Draft.Pool {
		if s := draftScore(p); s > bestScore {
			bestScore, best = s, p.ID
		}
	}
	rank := map[int]int{}
	for _, p := range lg.Draft.Pool {
		n := 0
		for _, q := range lg.Draft.Pool {
			if draftScore(q) > draftScore(p) {
				n++
			}
		}
		rank[p.ID] = n
	}
	hits, n, worst := 0, 0, 0
	for seed := uint64(1); seed <= 200; seed++ {
		lg.Seed = seed
		for team := 0; team < 30; team++ {
			if team == lg.UserTeamID {
				continue
			}
			id := lg.Draft.Pool[lg.autoChoice(team)].ID
			n++
			if id == best {
				hits++
			}
			worst = max(worst, rank[id])
		}
	}
	if hits == n {
		t.Error("the best prospect always went first")
	}
	if worst > 5 {
		t.Errorf("a computer team took the true #%d prospect first", worst+1)
	}
	t.Logf("true best goes first %.1f%% of the time; worst first pick was true #%d", float64(hits)/float64(n)*100, worst+1)
}

func TestComputerDraftDeterministic(t *testing.T) {
	a, b := draftLeague(9), draftLeague(9)
	a.DraftAutoAll()
	b.DraftAutoAll()
	for i := range a.Draft.Results {
		if a.Draft.Results[i] != b.Draft.Results[i] {
			t.Fatalf("pick %d differs", i+1)
		}
	}
}

func TestUserAutoPickFollowsView(t *testing.T) {
	lg := draftLeague(10)
	lg.UserTeamID = lg.DraftOnClock()
	for i := 0; i < 5; i++ {
		lg.Scout(lg.Draft.Pool[i].ID)
	}
	want, ws := -1, -1.0
	for _, p := range lg.Draft.Pool {
		if s := lg.userDraftScore(p); s > ws {
			want, ws = p.ID, s
		}
	}
	lg.DraftPick(-1)
	if got := lg.Draft.Results[0].PlayerID; got != want {
		t.Errorf("auto picked %d, want %d", got, want)
	}
}

func TestScoutSaveRoundTripAndOldSave(t *testing.T) {
	lg := draftLeague(11)
	lg.Scout(lg.Draft.Pool[3].ID)
	lg.Scout(lg.Draft.Pool[3].ID)
	lg.Scout(lg.Draft.Pool[5].ID)
	for i := 0; i < 4; i++ {
		lg.DraftPick(-1)
	}
	path := filepath.Join(t.TempDir(), "s.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a, b := lg.Draft, got.Draft
	if b.ScoutPoints != a.ScoutPoints || len(b.ScoutLevel) != len(a.ScoutLevel) || len(b.OvrErr) != len(a.OvrErr) || len(b.Order2) != 30 || len(b.Lottery) != 14 {
		t.Fatal("draft state changed in the round trip")
	}
	for id, v := range a.ScoutLevel {
		if b.ScoutLevel[id] != v {
			t.Fatal("levels differ")
		}
	}
	for id, v := range a.OvrErr {
		if b.OvrErr[id] != v || b.PotErr[id] != a.PotErr[id] {
			t.Fatal("errors differ")
		}
	}

	// A version-3 save made mid-draft has no scouting state and one order.
	a.OvrErr, a.PotErr, a.ScoutLevel, a.Order2, a.Lottery, a.ScoutPoints = nil, nil, nil, nil, nil, 0
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	rewriteVersion(t, path, 3)
	old, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if old.ScoutPointsLeft() != ScoutTuning.Points {
		t.Errorf("old save points %d", old.ScoutPointsLeft())
	}
	if _, ok := old.ScoutView(old.Draft.Pool[0].ID); !ok {
		t.Error("old save has no view")
	}
	old.DraftAutoAll()
	if !old.DraftDone() || len(old.Draft.Results) != 60 {
		t.Error("old-save draft did not finish")
	}
}
