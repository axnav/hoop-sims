package engine

import (
	"math"
	"path/filepath"
	"testing"
)

func TestCoachesDeterministic(t *testing.T) {
	a, b, c := GenerateLeague(3), GenerateLeague(3), GenerateLeague(4)
	diff := false
	for i := range a.Teams {
		ca, cb, cc := a.Teams[i].Coach, b.Teams[i].Coach, c.Teams[i].Coach
		if ca == nil || *ca != *cb {
			t.Fatalf("team %d: same seed gave different coaches: %v %v", i, ca, cb)
		}
		if ca.Name == "" || ca.Offense < 40 || ca.Offense > 95 || ca.Defense < 40 || ca.Defense > 95 ||
			ca.Development < 40 || ca.Development > 95 || !ValidSystemKey(ca.System) {
			t.Errorf("team %d: bad coach %+v", i, ca)
		}
		if *ca != *cc {
			diff = true
		}
	}
	if !diff {
		t.Error("different seeds gave the same coaches")
	}
}

func TestLoadVersion3SaveGetsCoaches(t *testing.T) {
	lg := GenerateLeague(7)
	want := make([]Coach, len(lg.Teams))
	for i, tm := range lg.Teams {
		want[i] = *tm.Coach
		tm.Coach = nil // a save from before coaches
	}
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	rewriteVersion(t, path, 3)
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != SaveVersion || SaveVersion < 4 {
		t.Errorf("version %d, want %d (at least 4)", got.Version, SaveVersion)
	}
	for i, tm := range got.Teams {
		if tm.Coach == nil || *tm.Coach != want[i] {
			t.Fatalf("team %d: coach after upgrade %+v, want %+v", i, tm.Coach, want[i])
		}
		if tm.System != "" {
			t.Errorf("team %d: system %q, want the coach's favourite (empty)", i, tm.System)
		}
	}
	// A saved coach, system and pool survive a round trip.
	got.Teams[2].System = "run_gun"
	got.CoachPool = []*Coach{{ID: 1, Name: "Test Coach", Offense: 70, Defense: 60, Development: 80, System: "star_led"}}
	if err := got.Save(path); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.Teams[2].System != "run_gun" || *again.Teams[2].Coach != want[2] || len(again.CoachPool) != 1 || again.CoachPool[0].Name != "Test Coach" {
		t.Errorf("round trip lost coach data: %v %+v %v", again.Teams[2].System, again.Teams[2].Coach, again.CoachPool)
	}
}

func TestFitGradesAndReasons(t *testing.T) {
	lg := GenerateLeague(5)
	shooter := plainPlayer(9001, 50)
	shooter.Ratings.Three, shooter.Ratings.Mid, shooter.Ratings.Athleticism = 92, 80, 75
	big := plainPlayer(9002, 50)
	big.Ratings.Inside, big.Ratings.Rebounding, big.Ratings.Defense = 90, 88, 80
	big.Ratings.Three = 30
	ps := SystemByKey("pace_space")
	io := SystemByKey("inside_out")
	fs, fb := lg.PlayerFitUnder(ps, shooter), lg.PlayerFitUnder(ps, big)
	if fs.Score <= fb.Score+25 {
		t.Errorf("shooter %.0f vs big %.0f in Pace and Space", fs.Score, fb.Score)
	}
	if gs, gb := fs.Grade(), fb.Grade(); gs != "A" || (gb != "D" && gb != "C") {
		t.Errorf("grades shooter %s, big %s", gs, gb)
	}
	if g := lg.PlayerFitUnder(io, big).Grade(); g != "A" {
		t.Errorf("big in Inside-Out: %s", g)
	}
	if len(fs.Reasons) == 0 || len(fs.Reasons) > 3 {
		t.Errorf("reasons %v", fs.Reasons)
	}
	if r := fb.Reasons; len(r) == 0 || len(r) > 3 {
		t.Errorf("reasons %v", r)
	}
	found := false
	for _, r := range fs.Reasons {
		if len(r) > 4 && r[:4] == "Has " {
			found = true
		}
	}
	if !found {
		t.Errorf("shooter with Spot-Up has no 'Has ...' reason: %v", fs.Reasons)
	}
	low := false
	for _, r := range fb.Reasons {
		if len(r) > 3 && r[:3] == "Low" {
			low = true
		}
	}
	if !low {
		t.Errorf("big with a bad three has no 'Low ...' reason: %v", fb.Reasons)
	}
	// Balanced fits everyone the same.
	bal := SystemByKey("balanced")
	if lg.PlayerFitUnder(bal, shooter).Score != lg.PlayerFitUnder(bal, big).Score {
		t.Error("Balanced should fit everyone equally")
	}
	// Grade thresholds.
	for _, c := range []struct {
		s float64
		g string
	}{{100, "A"}, {SystemTuning.GradeA, "A"}, {SystemTuning.GradeA - 0.1, "B"}, {SystemTuning.GradeB, "B"},
		{SystemTuning.GradeC, "C"}, {SystemTuning.GradeC - 0.1, "D"}, {0, "D"}} {
		if g := GradeOf(c.s); g != c.g {
			t.Errorf("GradeOf(%.1f) = %s, want %s", c.s, g, c.g)
		}
	}
	// Scores stay in 0..100.
	for _, p := range lg.Teams[0].Roster {
		for i := range Systems {
			if f := lg.PlayerFitUnder(&Systems[i], p); f.Score < 0 || f.Score > 100 {
				t.Errorf("score %v out of range", f.Score)
			}
		}
	}
}

func TestOffFavouritePenalty(t *testing.T) {
	lg := GenerateLeague(6)
	tm := lg.Teams[4]
	fav := tm.Coach.System
	other := "balanced"
	if fav == other {
		other = "pace_space"
	}
	// Raw team fit under the other system, without the penalty.
	pms := tm.EffectiveMinutes()
	var sum, w float64
	for _, pm := range pms {
		if pm.Min > 0 {
			sum += float64(pm.Min) * SystemByKey(other).fitScore(inputsOf(pm.Player), lg.badgeSet(pm.Player))
			w += float64(pm.Min)
		}
	}
	got := lg.TeamFitUnder(tm, other)
	if math.Abs(got-(sum/w-CoachTuning.FavouritePenalty)) > 1e-9 {
		t.Errorf("fit under %s %.3f, want raw %.3f minus %.0f", other, got, sum/w, CoachTuning.FavouritePenalty)
	}
	if !tm.OnFavourite() {
		t.Error("a fresh team runs its favourite")
	}
	if err := lg.SetSystem(tm.ID, other); err != nil {
		t.Fatal(err)
	}
	if tm.OnFavourite() || tm.SystemKey() != other {
		t.Errorf("after SetSystem: favourite %v, system %s", tm.OnFavourite(), tm.SystemKey())
	}
	if lg.SetSystem(tm.ID, "nonsense") == nil {
		t.Error("unknown system accepted")
	}
	// Choosing the favourite by key is not off-favourite.
	lg.SetSystem(tm.ID, fav)
	if !tm.OnFavourite() {
		t.Error("choosing the favourite key counts as off-favourite")
	}
}

func TestCoachEffectsAreCentred(t *testing.T) {
	lg := GenerateLeague(9)
	m := lg.coachMeansNow()
	var offSum, defSum, fitSum, devSum float64
	for _, tm := range lg.Teams {
		offSum += float64(tm.Coach.Offense) - m.off
		defSum += float64(tm.Coach.Defense) - m.def
		fitSum += lg.TeamFit(tm) - m.fit
		devSum += lg.devMult(tm.ID)
	}
	n := float64(len(lg.Teams))
	if math.Abs(offSum) > 1e-6 || math.Abs(defSum) > 1e-6 || math.Abs(fitSum) > 1e-6 {
		t.Errorf("deviations from the league mean do not sum to zero: %v %v %v", offSum, defSum, fitSum)
	}
	if math.Abs(devSum/n-1) > 0.01 {
		t.Errorf("mean Development multiplier %.4f, want about 1", devSum/n)
	}
	// The best and worst coaches sit inside the owner's +-8% band.
	for _, tm := range lg.Teams {
		if g := lg.devMult(tm.ID); g < 1-CoachTuning.DevRange-1e-9 || g > 1+CoachTuning.DevRange+1e-9 {
			t.Errorf("devMult %v outside the band", g)
		}
	}
	// The effect sizes: best against worst coach and team fit, in points of FG%.
	lo, hi := 100.0, 0.0
	for _, tm := range lg.Teams {
		lo, hi = math.Min(lo, lg.TeamFit(tm)), math.Max(hi, lg.TeamFit(tm))
	}
	if sw := CoachTuning.FitScale * (hi - lo); sw > 0.016 {
		t.Errorf("fit swing %.3f of FG%%, owner asked for at most about 0.015", sw)
	}
	// A coach with the top Development rating makes growth faster, the bottom slower.
	lg.Teams[0].Coach.Development, lg.Teams[1].Coach.Development = 95, 40
	if lg.devMult(0) <= 1 || lg.devMult(1) >= 1 || lg.devMult(-1) != 1 {
		t.Errorf("devMult %v %v %v", lg.devMult(0), lg.devMult(1), lg.devMult(-1))
	}
	// Games without a league ignore coaches and systems entirely.
	r1, r2 := NewRNG(5), NewRNG(5)
	a := SimGame(r1, lg.Teams[0], lg.Teams[1])
	lg.Teams[0].System = "run_gun"
	b := SimGame(r2, lg.Teams[0], lg.Teams[1])
	if a.Home.Score != b.Home.Score || a.Away.Score != b.Away.Score {
		t.Error("a league-less game depends on the system")
	}
}

func TestSystemShiftsChangeTheGame(t *testing.T) {
	lg := GenerateLeague(11)
	home, away := lg.Teams[3], lg.Teams[8]
	stat := func(sys string) (poss, tpa float64) {
		lg.SetSystem(home.ID, sys)
		r := NewRNG(31)
		for i := 0; i < 300; i++ {
			res := lg.SimGame(r, home, away)
			for _, l := range res.Home.Lines {
				poss += float64(l.FGA + l.Tov)
				tpa += float64(l.TPA)
			}
		}
		return
	}
	pb, tb := stat("balanced")
	pr, _ := stat("run_gun")
	pd, _ := stat("defense_first")
	_, ts := stat("pace_space")
	_, ti := stat("inside_out")
	if !(pr > pb && pb > pd) {
		t.Errorf("shot-ish possessions: run_gun %.0f, balanced %.0f, defense_first %.0f", pr, pb, pd)
	}
	if !(ts > tb && tb > ti) {
		t.Errorf("threes: pace_space %.0f, balanced %.0f, inside_out %.0f", ts, tb, ti)
	}
}

func TestHireCoachOffseasonOnly(t *testing.T) {
	lg := GenerateLeague(12)
	lg.StartSeason()
	if lg.CoachPoolOpen() || lg.HireCoach(0) != ErrCoachPoolClosed {
		t.Fatal("hiring must be refused in the season")
	}
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	if lg.HireCoach(0) != ErrCoachPoolClosed {
		t.Fatal("hiring must be refused before the offseason")
	}
	lg.BeginOffseason()
	if len(lg.CoachPool) != CoachTuning.PoolSize || !lg.CoachPoolOpen() {
		t.Fatalf("pool of %d", len(lg.CoachPool))
	}
	for i, c := range lg.CoachPool {
		if c.ID != i || c.Name == "" || !ValidSystemKey(c.System) {
			t.Fatalf("bad candidate %+v", c)
		}
	}
	if lg.HireCoach(99) != ErrNoSuchCoach {
		t.Error("unknown candidate accepted")
	}
	u := lg.Teams[lg.UserTeamID]
	old := *u.Coach
	u.System = "run_gun"
	want := *lg.CoachPool[5]
	if err := lg.HireCoach(5); err != nil {
		t.Fatal(err)
	}
	if u.Coach.Name != want.Name || u.Coach.Offense != want.Offense || u.Coach.Name == old.Name && old.Name != want.Name {
		t.Errorf("hired %+v, want %+v", u.Coach, want)
	}
	if u.System != "" || len(lg.CoachPool) != CoachTuning.PoolSize-1 {
		t.Errorf("system %q, pool %d", u.System, len(lg.CoachPool))
	}
	for _, c := range lg.CoachPool {
		if c.ID == 5 {
			t.Error("hired candidate still in the pool")
		}
	}
	lg.DraftAutoAll()
	lg.FinishOffseason()
	if len(lg.CoachPool) != 0 || lg.HireCoach(0) != ErrCoachPoolClosed {
		t.Errorf("pool %d after the offseason; hiring must be refused", len(lg.CoachPool))
	}
	if u.Coach.Name != want.Name {
		t.Error("the user's coach must stay through the offseason")
	}
}

func TestComputerTeamsReplaceWeakCoaches(t *testing.T) {
	lg := GenerateLeague(13)
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	// Make every computer coach weak so the replacement rule has work to do.
	weak := map[int]*Coach{}
	for _, tm := range lg.Teams {
		if tm.ID != lg.UserTeamID {
			tm.Coach.Offense, tm.Coach.Defense, tm.Coach.Development = 42, 42, 42
			weak[tm.ID] = tm.Coach
		}
	}
	userCoach := lg.Teams[lg.UserTeamID].Coach
	lg.BeginOffseason()
	lg.DraftAutoAll()
	lg.FinishOffseason()
	replaced := 0
	for id, c := range weak {
		if lg.Teams[id].Coach != c {
			replaced++
			if lg.Teams[id].Coach.ID != 0 || lg.Teams[id].Coach.Overall() < c.Overall()+CoachTuning.ReplaceMargin {
				t.Errorf("team %d: replacement %+v is not clearly better", id, lg.Teams[id].Coach)
			}
		}
	}
	if replaced == 0 || replaced > CoachTuning.PoolSize {
		t.Errorf("replaced %d coaches", replaced)
	}
	if lg.Teams[lg.UserTeamID].Coach != userCoach {
		t.Error("the user's coach must never be replaced by the computer")
	}
	for _, tm := range lg.Teams {
		if !ValidSystemKey(tm.SystemKey()) || (tm.System != "" && !ValidSystemKey(tm.System)) {
			t.Errorf("team %d: bad system %q", tm.ID, tm.System)
		}
	}
}

func TestOffseasonCoachesDeterministic(t *testing.T) {
	play := func() *League {
		lg := GenerateLeague(14)
		lg.StartSeason()
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		lg.Offseason()
		return lg
	}
	a, b := play(), play()
	for i := range a.Teams {
		ca, cb := a.Teams[i].Coach, b.Teams[i].Coach
		if *ca != *cb || a.Teams[i].System != b.Teams[i].System {
			t.Fatalf("team %d differs between identical runs: %+v/%q vs %+v/%q", i, ca, a.Teams[i].System, cb, b.Teams[i].System)
		}
	}
}

func TestChooseSystemNeedsAMargin(t *testing.T) {
	lg := GenerateLeague(15)
	tm := lg.Teams[7]
	lg.chooseSystem(tm)
	best, bf := lg.BestSystem(tm)
	fav := lg.TeamFitUnder(tm, tm.Coach.System)
	switch {
	case bf > fav+CoachTuning.SystemMargin && best.Key != tm.Coach.System:
		if tm.System != best.Key {
			t.Errorf("should have switched to %s (%.1f vs favourite %.1f), has %q", best.Key, bf, fav, tm.System)
		}
	default:
		if tm.System != "" && tm.System != best.Key {
			t.Errorf("kept %q although nothing beats the favourite by the margin", tm.System)
		}
	}
	// A roster that cannot fit the favourite switches.
	tm.Coach.System = "star_led"
	for _, p := range tm.Roster {
		p.Ratings = Ratings{30, 30, 30, 30, 30, 90, 90}
		p.UsageTendency = 25
	}
	lg.chooseSystem(tm)
	if tm.System == "" || tm.System == "star_led" {
		t.Errorf("a roster of defenders kept star_led (%q)", tm.System)
	}
}
