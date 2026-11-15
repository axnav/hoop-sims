package engine

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// plainPlayer is a player with every rating at v.
func plainPlayer(id, v int) *Player {
	return &Player{ID: id, Age: 27, Experience: 5, TeamID: -1,
		Ratings: Ratings{v, v, v, v, v, v, v}}
}

func TestPersonalityBadgesDeterministic(t *testing.T) {
	a, b := GenerateLeague(3), GenerateLeague(3)
	other := GenerateLeague(4)
	diff := false
	for ti := range a.Teams {
		leaders, mentors, stars := 0, 0, 0
		for pi, p := range a.Teams[ti].Roster {
			l1, m1, s1 := a.personality(p)
			l2, m2, s2 := b.personality(b.Teams[ti].Roster[pi])
			l3, m3, s3 := a.personality(p) // the same call twice
			if l1 != l2 || m1 != m2 || s1 != s2 || l1 != l3 || m1 != m3 || s1 != s3 {
				t.Fatalf("personality badges are not repeatable for player %d", p.ID)
			}
			if l, m, s := other.personality(p); l != l1 || m != m1 || s != s1 {
				diff = true
			}
			if l1 {
				leaders++
			}
			if m1 {
				mentors++
			}
			if s1 {
				stars++
			}
		}
		if leaders > 4 || mentors > 4 || stars > 4 {
			t.Errorf("team %d has too many personality badges: %d leaders, %d mentors, %d stars", ti, leaders, mentors, stars)
		}
	}
	if !diff {
		t.Error("a different league seed gave exactly the same personality badges")
	}
}

func TestPersonalityConditions(t *testing.T) {
	lg := GenerateLeague(1)
	young := plainPlayer(1, 90)
	young.Age, young.Experience = 20, 1
	for id := 0; id < 300; id++ {
		young.ID = id
		if l, m, _ := lg.personality(young); l || m {
			t.Fatalf("a 20-year-old with one season got Leader or Mentor (id %d)", id)
		}
	}
	weak := plainPlayer(1, 50)
	weak.Age, weak.Experience = 33, 12
	for id := 0; id < 300; id++ {
		weak.ID = id
		if _, _, s := lg.personality(weak); s {
			t.Fatalf("a 50-overall player got Star (id %d)", id)
		}
	}
	// Someone qualifies for each, so the rules are reachable.
	vet := plainPlayer(1, 90)
	vet.Age, vet.Experience = 33, 12
	var l, m, s int
	for id := 0; id < 300; id++ {
		vet.ID = id
		a, b, c := lg.personality(vet)
		if a {
			l++
		}
		if b {
			m++
		}
		if c {
			s++
		}
	}
	if l == 0 || m == 0 || s == 0 {
		t.Errorf("a qualifying veteran never got Leader/Mentor/Star: %d %d %d of 300", l, m, s)
	}
}

func TestProspectsGetNoBadges(t *testing.T) {
	lg := GenerateLeague(5)
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.BeginOffseason()
	if len(lg.Draft.Pool) == 0 {
		t.Fatal("no prospects")
	}
	for _, p := range lg.Draft.Pool {
		p.Ratings = Ratings{99, 99, 99, 99, 99, 99, 99} // would earn everything
		p.Age, p.Experience = 31, 12
		if got := lg.BadgesOf(p); len(got) != 0 {
			t.Fatalf("prospect %d has badges: %v", p.ID, got)
		}
		if lg.BadgeCount(p) != 0 || !lg.IsProspect(p) {
			t.Fatalf("prospect %d counted as having badges", p.ID)
		}
	}
	// A drafted player is no longer a prospect.
	lg.DraftPick(-1)
	p := lg.Draft.Results[0]
	if q := lg.FindPlayer(p.PlayerID); q == nil || lg.IsProspect(q) {
		t.Error("a drafted player is still treated as a prospect")
	}
}

func defByKey(t *testing.T, key string) *badgeDef {
	t.Helper()
	for i := range badgeDefs {
		if badgeDefs[i].Key == key {
			return &badgeDefs[i]
		}
	}
	t.Fatalf("no badge %q", key)
	return nil
}

func TestBadgeCatalogueShape(t *testing.T) {
	if len(badgeDefs) != 37 {
		t.Fatalf("%d skill badges, want 37", len(badgeDefs))
	}
	cat := BadgeCatalogue()
	if len(cat) != 40 {
		t.Fatalf("%d catalogue entries, want 40", len(cat))
	}
	groups := map[string]bool{"finishing": true, "shooting": true, "playmaking": true, "defense": true, "rebounding": true, "general": true, "personality": true}
	seen := map[string]bool{}
	for _, c := range cat {
		if seen[c.Key] || !groups[c.Group] || c.Rule == "" || c.Effect == "" {
			t.Errorf("bad catalogue entry %+v", c)
		}
		seen[c.Key] = true
		if strings.Contains(strings.ToLower(c.Rule+c.Effect), "impact") {
			t.Errorf("%s mentions the hidden number", c.Key)
		}
	}
	for _, d := range badgeDefs {
		for i := 1; i < 5; i++ {
			if d.thr[i] <= d.thr[i-1] {
				t.Errorf("%s: thresholds not increasing: %v", d.Key, d.thr)
			}
		}
		if d.hook < 0 || d.hook >= numHooks || d.share <= 0 {
			t.Errorf("%s: no hook", d.Key)
		}
	}
	for _, r := range perfRules {
		if defByKey(t, r.Key) == nil {
			t.Errorf("performance rule for unknown badge %s", r.Key)
		}
	}
	if len(perfRules) < 6 || len(perfRules) > 10 {
		t.Errorf("%d performance rules, want 6 to 10", len(perfRules))
	}
}

func TestBadgeTiersAndPositionLimits(t *testing.T) {
	d := defByKey(t, "rim_protector")
	mk := func(pos Position, v int) *Player {
		p := plainPlayer(1, v)
		p.Pos = pos
		return p
	}
	lg := GenerateLeague(2)
	lg.Season = &Season{Stats: map[int]*PlayerSeason{}}
	tier := func(pos Position, v int) Tier {
		bs := lg.badgeSet(mk(pos, v))
		for i := range badgeDefs {
			if badgeDefs[i].Key == d.Key {
				return bs.Tier[i]
			}
		}
		return NoTier
	}
	if tier(C, 99) != Legend || tier(PF, 99) != Legend {
		t.Errorf("a 99 big should reach Legend: C %v PF %v", tier(C, 99), tier(PF, 99))
	}
	if tier(SF, 99) != Gold {
		t.Errorf("SF limit: %v, want Gold", tier(SF, 99))
	}
	if tier(SG, 99) != Silver {
		t.Errorf("SG limit: %v, want Silver", tier(SG, 99))
	}
	if tier(PG, 99) != NoTier {
		t.Errorf("PG limit: %v, want none", tier(PG, 99))
	}
	// Thresholds are ">=": the Bronze line and one below.
	bronze := d.thr[0]
	if got := tier(C, bronze); got != Bronze {
		t.Errorf("rating at the Bronze line: %v", got)
	}
	if got := tier(C, bronze-1); got != NoTier {
		t.Errorf("rating just below the Bronze line: %v", got)
	}
	// A "both" badge needs both ratings.
	pi := defByKey(t, "power_inside")
	in := inputsOf(plainPlayer(1, 99))
	in[4] = 30
	if pi.ratingTier(in) != NoTier {
		t.Error("Power Inside with a low rebounding rating")
	}
}

func TestBadgesOfOrderTextAndNext(t *testing.T) {
	lg := GenerateLeague(2)
	lg.Season = &Season{Stats: map[int]*PlayerSeason{}}
	p := plainPlayer(77, 60)
	p.Pos = C
	p.Ratings.Inside, p.Ratings.Defense, p.Ratings.Rebounding = 99, 99, 99
	got := lg.BadgesOf(p)
	if len(got) < 5 {
		t.Fatalf("too few badges: %+v", got)
	}
	last := Legend
	for _, b := range got {
		if b.Kind == "personality" {
			continue
		}
		if b.Tier > last {
			t.Errorf("not sorted legend first: %+v", got)
		}
		last = b.Tier
		if b.Rule == "" || b.Effect == "" || b.Kind != "rating" {
			t.Errorf("badge missing text or kind: %+v", b)
		}
		d := defByKey(t, b.Key)
		if b.Tier < d.maxTier(C) && b.Next == "" {
			t.Errorf("%s: below the limit but no next", b.Key)
		}
		if b.Tier >= d.maxTier(C) && b.Next != "" {
			t.Errorf("%s: at the limit but next %q", b.Key, b.Next)
		}
		if b.Next != "" && !strings.Contains(b.Next, " at ") {
			t.Errorf("next text: %q", b.Next)
		}
	}
	if got[0].Tier != Legend {
		t.Errorf("a 99-inside C should start with a Legend: %+v", got[0])
	}
}

func TestPerfBoostsAwardedAndSaved(t *testing.T) {
	lg := GenerateLeague(6)
	lg.StartSeason()
	lg.SimRegularSeason()
	n := 0
	for _, p := range lg.allPlayers() {
		if len(p.PerfBadges) == 0 {
			continue
		}
		n++
		ps := lg.Season.Stats[p.ID]
		if ps == nil || ps.GP < BadgeTuning.PerfMinGP || ps.Min < BadgeTuning.PerfMinMinutes {
			t.Errorf("player %d has a boost without enough games or minutes", p.ID)
		}
	}
	if n == 0 {
		t.Error("nobody earned a performance boost in a full season")
	}

	// A weak-rated big with a huge rebounding season: Glass Cleaner goes up one tier.
	p := lg.Teams[0].Roster[0]
	p.Pos = C
	p.Ratings = Ratings{50, 50, 50, 50, 50, 50, 50}
	p.Ratings.Rebounding = defByKey(t, "glass_cleaner").thr[1] // Silver by rating
	p.PerfBadges = nil
	lg.Season.Stats[p.ID] = &PlayerSeason{PlayerID: p.ID, GP: 70, Min: 2100, Reb: 2100 * 14 / 36, FTA: 100, FTM: 50, Ast: 100}
	lg.awardPerfBadges()
	if p.PerfBadges["glass_cleaner"] != 1 || len(p.PerfBadges) != 1 {
		t.Fatalf("boosts %v, want only glass_cleaner", p.PerfBadges)
	}
	check := func(want Tier, kind string) {
		t.Helper()
		for _, b := range lg.BadgesOf(p) {
			if b.Key == "glass_cleaner" {
				if b.Tier != want || b.Kind != kind {
					t.Errorf("glass_cleaner: tier %v kind %s, want %v %s (rule %q)", b.Tier, b.Kind, want, kind, b.Rule)
				}
				return
			}
		}
		t.Errorf("glass_cleaner missing: %+v", lg.BadgesOf(p))
	}
	check(Gold, "performance")
	// Never above the position limit: a PG's Glass Cleaner limit is Bronze.
	p.Pos = PG
	p.Ratings.Rebounding = 99
	for _, b := range lg.BadgesOf(p) {
		if b.Key == "glass_cleaner" && b.Tier > defByKey(t, "glass_cleaner").maxTier(PG) {
			t.Errorf("boost went past the position limit: %+v", b)
		}
	}
	p.Pos = C

	// Saved and loaded.
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if q := got.FindPlayer(p.ID); q.PerfBadges["glass_cleaner"] != 1 || len(q.PerfBadges) != 1 {
		t.Errorf("boosts lost in the save: %v", q.PerfBadges)
	}
	// Too few games: nothing, and last season's boosts are replaced.
	lg.Season.Stats[p.ID] = &PlayerSeason{PlayerID: p.ID, GP: 30, Min: 2100, Reb: 2000}
	lg.awardPerfBadges()
	if len(p.PerfBadges) != 0 {
		t.Errorf("30 games earned %v", p.PerfBadges)
	}
}

// The league mean of every hook effect is zero: a player with an average
// badge load plays like today.
func TestBadgeEffectsCentred(t *testing.T) {
	lg := GenerateLeague(1)
	m := lg.badgeMeansNow()
	var num, den [numHooks]float64
	for _, tm := range lg.Teams {
		pms := tm.EffectiveMinutes()
		ws := hookWeights(pms)
		for i, pm := range pms {
			f := lg.badgeSet(pm.Player).hookF()
			for h := range f {
				num[h] += ws[i][h] * (f[h] - m.hook[h])
				den[h] += ws[i][h]
			}
		}
	}
	for h := range num {
		if den[h] == 0 || math.Abs(num[h]/den[h]) > 1e-9 {
			t.Errorf("hook %d: weighted mean effect %v, want 0", h, num[h]/den[h])
		}
	}
	// A heavy-badge star helps and a player with none hurts, on his own shots.
	var best, none *Player
	bestN := -1
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			if n := lg.BadgeCount(p); n > bestN {
				best, bestN = p, n
			}
			if lg.BadgeCount(p) == 0 {
				none = p
			}
		}
	}
	fb, fn := lg.badgeSet(best).hookF(), lg.badgeSet(none).hookF()
	if fb[HRim]-m.hook[HRim] <= 0 || fn[HRim]-m.hook[HRim] >= 0 {
		t.Errorf("star %v / empty %v around mean %v", fb[HRim], fn[HRim], m.hook[HRim])
	}
}

func TestMentorGrowth(t *testing.T) {
	lg := GenerateLeague(8)
	tm := lg.Teams[0]
	young, old := tm.Roster[0], tm.Roster[1]
	young.Age, old.Age = 24, 25
	mentors := map[int]bool{tm.ID: true}
	if g := lg.growthMult(young, mentors); g != 1+BadgeTuning.MentorGrowth {
		t.Errorf("24-year-old teammate multiplier %v", g)
	}
	if g := lg.growthMult(old, mentors); g != 1 {
		t.Errorf("25-year-old multiplier %v, want 1", g)
	}
	other := lg.Teams[1].Roster[0]
	other.Age = 20
	if g := lg.growthMult(other, mentors); g != 1 {
		t.Errorf("player on another team gets %v", g)
	}
	fa := plainPlayer(9999, 60)
	fa.Age = 20
	if g := lg.growthMult(fa, mentors); g != 1 {
		t.Errorf("free agent gets %v", g)
	}
	if g := lg.growthMult(young, map[int]bool{}); g != 1 {
		t.Errorf("no mentor on the team: %v", g)
	}

	// mentorTeams finds a team with a Mentor, and one Mentor is enough (no stacking).
	m := tm.Roster[2]
	m.Age, m.Experience = 34, 14
	for sd := uint64(1); sd < 500; sd++ {
		lg.Seed = sd
		if _, ok, _ := lg.personality(m); ok {
			break
		}
	}
	if !lg.mentorTeams()[tm.ID] {
		t.Fatal("a team with a Mentor is not marked")
	}
	for _, p := range tm.Roster {
		if p != m && p.Age > 24 {
			p.Age, p.Experience = 35, 15 // make as many Mentors as the dice allow
		}
	}
	if g := lg.growthMult(young, lg.mentorTeams()); g != 1+BadgeTuning.MentorGrowth {
		t.Errorf("more Mentors must not stack: %v", g)
	}

	// The multiplier only widens positive steps: same random numbers, never less growth.
	a := GenerateLeague(8).Teams[0].Roster[3]
	b := GenerateLeague(8).Teams[0].Roster[3]
	a.Age, b.Age = 20, 20
	a.Potential, b.Potential = 95, 95
	ra, rb := NewRNG(5), NewRNG(5)
	a.ProgressMentored(ra, 20, nil, 1)
	b.ProgressMentored(rb, 20, nil, 1.3)
	for i, v := range a.Ratings.array() {
		if b.Ratings.array()[i] < v {
			t.Errorf("rating %d: mentored %d below plain %d", i, b.Ratings.array()[i], v)
		}
	}
}

func TestBadgeGameDeterministicAndSaveSafe(t *testing.T) {
	lg := GenerateLeague(1)
	x := lg.SimGame(NewRNG(11), lg.Teams[0], lg.Teams[1])
	y := lg.SimGame(NewRNG(11), lg.Teams[0], lg.Teams[1])
	if x.Home.Score != y.Home.Score || x.Away.Score != y.Away.Score {
		t.Error("same seed, different game")
	}
	z := lg.SimGameRec(NewRNG(11), lg.Teams[0], lg.Teams[1], true)
	if z.Home.Score != x.Home.Score {
		t.Error("recording changed the game")
	}
	// A save and load in the middle of a season changes nothing (the league
	// means are derived from the state, not kept).
	a := GenerateLeague(9)
	a.StartSeason()
	a.SimDays(12)
	path := filepath.Join(t.TempDir(), "l.json")
	if err := a.Save(path); err != nil {
		t.Fatal(err)
	}
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a.SimDays(6)
	b.SimDays(6)
	for i, g := range a.Season.Games {
		if h := b.Season.Games[i]; g.HomeScore != h.HomeScore || g.AwayScore != h.AwayScore {
			t.Fatalf("game %d differs after a reload: %d-%d vs %d-%d", i, g.HomeScore, g.AwayScore, h.HomeScore, h.AwayScore)
		}
	}
	// A plain SimGame (no league) has no badge effects and is repeatable too.
	p1 := SimGame(NewRNG(3), lg.Teams[2], lg.Teams[3])
	p2 := SimGame(NewRNG(3), lg.Teams[2], lg.Teams[3])
	if p1.Home.Score != p2.Home.Score {
		t.Error("plain SimGame not repeatable")
	}
}
