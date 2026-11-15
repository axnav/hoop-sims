package engine

import (
	"path/filepath"
	"testing"
)

func lineupLeague(seed uint64) *League {
	lg := GenerateLeague(seed)
	lg.Options.NoInjuries = true
	return lg
}

func sumMin(pms []PlayerMinutes) int {
	n := 0
	for _, pm := range pms {
		n += pm.Min
	}
	return n
}

func TestEffectiveMinutesAutomatic(t *testing.T) {
	lg := lineupLeague(3)
	for _, tm := range lg.Teams[:5] {
		rot := tm.Rotation()
		got := tm.EffectiveMinutes()
		if len(got) != len(rot) {
			t.Fatalf("len %d vs %d", len(got), len(rot))
		}
		for i, pm := range got {
			if pm.Player != rot[i] || pm.Min != minutesByRank[i] {
				t.Fatalf("slot %d differs from automatic", i)
			}
		}
	}
}

func TestClearedLineupSameSeason(t *testing.T) {
	a, b := GenerateLeague(5), GenerateLeague(5)
	for i := range b.Teams {
		b.ClearLineup(i)
	}
	a.StartSeason()
	b.StartSeason()
	a.SimRegularSeason()
	b.SimRegularSeason()
	for i := range a.Teams {
		if a.Season.Wins[i] != b.Season.Wins[i] {
			t.Fatalf("team %d wins differ", i)
		}
	}
}

func TestSetLineupValidation(t *testing.T) {
	lg := lineupLeague(4)
	tm := lg.Teams[0]
	good := lg.AutoLineup(0)
	if err := lg.SetLineup(0, good); err != nil {
		t.Fatalf("auto lineup rejected: %v", err)
	}
	clone := func() Lineup {
		l := Lineup{Starters: good.Starters, Minutes: map[int]int{}}
		for k, v := range good.Minutes {
			l.Minutes[k] = v
		}
		return l
	}
	other := lg.Teams[1].Roster[0].ID
	bad := map[string]func(*Lineup){
		"duplicate starter":  func(l *Lineup) { l.Starters[1] = l.Starters[0] },
		"starter off roster": func(l *Lineup) { l.Starters[2] = other },
		"minutes off roster": func(l *Lineup) { l.Minutes[other] = 1 },
		"over max":           func(l *Lineup) { l.Minutes[l.Starters[0]] = MaxPlayerMinutes + 1 },
		"negative":           func(l *Lineup) { l.Minutes[l.Starters[0]] = -1 },
		"wrong total":        func(l *Lineup) { l.Minutes[l.Starters[0]]-- },
		"starter no minutes": func(l *Lineup) {
			m := l.Minutes[l.Starters[0]]
			l.Minutes[l.Starters[0]] = 0
			for id, v := range l.Minutes {
				isStarter := false
				for _, sid := range l.Starters {
					isStarter = isStarter || sid == id
				}
				if !isStarter && v+m <= MaxPlayerMinutes {
					l.Minutes[id] += m
					break
				}
			}
		},
		"too few players": func(l *Lineup) {
			l.Minutes = map[int]int{}
			for i, p := range tm.Roster[:7] {
				l.Minutes[p.ID] = 34
				if i == 6 {
					l.Minutes[p.ID] = 240 - 6*34
				}
			}
			for i := range l.Starters {
				l.Starters[i] = tm.Roster[i].ID
			}
		},
	}
	for name, f := range bad {
		l := clone()
		f(&l)
		if err := lg.SetLineup(0, l); err == nil {
			t.Errorf("%s: accepted", name)
		} else {
			t.Logf("%s: %v", name, err)
		}
	}
	if tm.Lineup == nil {
		t.Fatal("valid lineup not stored")
	}
	lg.ClearLineup(0)
	if tm.Lineup != nil {
		t.Fatal("not cleared")
	}
}

// customLineup makes the 11th-ranked bench player a 30-minute starter.
func customLineup(t *testing.T, lg *League, tid int) (Lineup, *Player) {
	l := lg.AutoLineup(tid)
	tm := lg.Teams[tid]
	rot := tm.Rotation()
	scrub := rot[11] // 4 minutes today
	star := tm.player(l.Starters[0])
	l.Minutes[scrub.ID] += 26 // 30
	l.Minutes[star.ID] -= 26
	l.Starters[0] = scrub.ID
	if err := lg.SetLineup(tid, l); err != nil {
		t.Fatal(err)
	}
	return l, scrub
}

func TestCustomLineupHonoured(t *testing.T) {
	lg := lineupLeague(6)
	l, scrub := customLineup(t, lg, 0)
	tm := lg.Teams[0]
	res := SimGame(NewRNG(1), tm, lg.Teams[1])
	for _, ln := range res.Home.Lines {
		if ln.Min != l.Minutes[ln.Player.ID] {
			t.Fatalf("%s: %d min, target %d", ln.Player.Name, ln.Min, l.Minutes[ln.Player.ID])
		}
	}
	if res.Home.Lines[0].Player != scrub {
		t.Fatal("custom starter not first")
	}
	lg.StartSeason()
	lg.SimDays(20)
	ps := lg.Season.Stats[scrub.ID]
	if ps == nil || ps.GP < 15 {
		t.Fatal("scrub barely played")
	}
	if avg := float64(ps.Min) / float64(ps.GP); avg < 29.5 || avg > 30.5 {
		t.Fatalf("avg %.1f min", avg)
	}
}

func TestLineupStarterInjured(t *testing.T) {
	lg := lineupLeague(7)
	l, scrub := customLineup(t, lg, 0)
	tm := lg.Teams[0]
	scrub.Injury = &Injury{Name: "Test", GamesLeft: 5}
	got := tm.EffectiveMinutes()
	if sumMin(got) != TeamMinutes {
		t.Fatalf("total %d", sumMin(got))
	}
	for i, pm := range got {
		if pm.Player == scrub {
			t.Fatal("injured player plays")
		}
		if pm.Min > MaxPlayerMinutes {
			t.Fatal("over max")
		}
		if i < 5 && pm.Player.Injured() {
			t.Fatal("injured starter")
		}
	}
	// The listed player with the most minutes who was not starting takes the slot.
	var want *Player
	for _, p := range tm.Roster {
		isStarter := false
		for _, id := range l.Starters {
			isStarter = isStarter || id == p.ID
		}
		if !isStarter && !p.Injured() && (want == nil || l.Minutes[p.ID] > l.Minutes[want.ID]) {
			want = p
		}
	}
	if got[0].Player != want {
		t.Fatalf("slot 0 is %s, want %s", got[0].Player.Name, want.Name)
	}
	SimGame(NewRNG(2), tm, lg.Teams[1])
}

func TestLineupDepartedPlayer(t *testing.T) {
	lg := lineupLeague(8)
	l, scrub := customLineup(t, lg, 0)
	tm := lg.Teams[0]
	for i, p := range tm.Roster {
		if p == scrub {
			tm.Roster = append(tm.Roster[:i], tm.Roster[i+1:]...)
			break
		}
	}
	other := lg.Teams[1]
	other.Roster = append(other.Roster, scrub)
	got := tm.EffectiveMinutes()
	if sumMin(got) != TeamMinutes {
		t.Fatalf("total %d", sumMin(got))
	}
	if tm.Lineup.Starters[0] != NoPlayer {
		t.Fatalf("slot not vacated: %d", tm.Lineup.Starters[0])
	}
	if _, ok := tm.Lineup.Minutes[scrub.ID]; ok {
		t.Fatal("minutes remain")
	}
	_ = l
	SimGame(NewRNG(3), tm, other)
}

func TestLineupSaveRoundTrip(t *testing.T) {
	lg := lineupLeague(9)
	l, _ := customLineup(t, lg, 2)
	path := filepath.Join(t.TempDir(), "l.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	lg2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := lg2.Teams[2].Lineup
	if got == nil || got.Starters != l.Starters || len(got.Minutes) != len(l.Minutes) {
		t.Fatal("lineup lost")
	}
	for k, v := range l.Minutes {
		if got.Minutes[k] != v {
			t.Fatal("minutes differ")
		}
	}
}

func TestMinutesDevelopment(t *testing.T) {
	mk := func() *Player {
		p := buildPlayer(NewRNG(1), 900, PG, 50, 20)
		p.Potential, p.Impact = 80, 0
		return p
	}
	sum := func(age int, mpg float64) float64 {
		tot := 0.0
		for s := uint64(0); s < 200; s++ {
			p := mk()
			p.Age = age
			p.ProgressMinutes(NewRNG(s+10), mpg)
			tot += float64(p.Overall())
		}
		return tot / 200
	}
	if hi, lo := sum(20, 30), sum(20, 2); hi < lo+0.8 {
		t.Fatalf("young heavy minutes %.2f vs few %.2f", hi, lo)
	}
	if a, b := sum(26, 30), sum(26, 2); a != b {
		t.Fatalf("older player affected: %.2f vs %.2f", a, b)
	}
	if a, b := sum(20, -1), func() float64 {
		x := Progression
		Progression.MinutesDevBonus, Progression.MinutesDevPenalty = 0, 0
		defer func() { Progression = x }()
		return sum(20, 30)
	}(); a != b {
		t.Fatalf("no-stats vs switched off differ: %.2f vs %.2f", a, b)
	}
}

func TestMinutesDevNeutral(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3} {
		lg := GenerateLeague(seed)
		lg.StartSeason()
		lg.SimRegularSeason()
		n, sum, nAll := 0, 0.0, 0
		for _, p := range lg.allPlayers() {
			if p.Age > Progression.MinutesDevMaxAge {
				continue
			}
			nAll++
			if ps := lg.Season.statsFor(p.ID); ps != nil {
				n++
				sum += MinutesAdjust(p.Age, float64(ps.Min)/SeasonGames)
			}
		}
		t.Logf("seed %d: %d young players (%d with stats), avg adj over stats %.3f, over all %.3f", seed, nAll, n, sum/float64(n), sum/float64(nAll))
		if avg := sum / float64(nAll); avg < -0.1 || avg > 0.1 {
			t.Errorf("average adjustment %.3f not near zero", avg)
		}
	}
}

// slotTeam builds a team whose position slots would start two weak players:
// three good guards, a good big and a good forward, plus weak SG/SF-only
// players who would fill the SG and SF slots under position picking.
func slotTeam() *Team {
	guard := Ratings{Inside: 50, Mid: 70, Three: 72, Playmaking: 85, Rebounding: 40, Defense: 65, Athleticism: 70}
	big := Ratings{Inside: 85, Mid: 40, Three: 30, Playmaking: 35, Rebounding: 85, Defense: 80, Athleticism: 65}
	forward := Ratings{Inside: 75, Mid: 55, Three: 45, Playmaking: 40, Rebounding: 75, Defense: 70, Athleticism: 65}
	weak := Ratings{Inside: 45, Mid: 45, Three: 45, Playmaking: 45, Rebounding: 45, Defense: 45, Athleticism: 45}
	mk := func(id int, pos Position, r Ratings) *Player {
		return &Player{ID: id, Name: "P", Age: 27, Pos: pos, Ratings: r, Potential: 60}
	}
	return &Team{Roster: []*Player{
		mk(0, SG, weak), mk(1, SF, weak), mk(2, PG, guard), mk(3, PG, guard),
		mk(4, PG, guard), mk(5, C, big), mk(6, PF, forward), mk(7, PG, weak),
	}}
}

func TestRotationStartsBestFive(t *testing.T) {
	tm := slotTeam()
	rot := tm.Rotation()
	if len(rot) != 8 {
		t.Fatalf("rotation has %d players", len(rot))
	}
	// The five best by overall start (ties: lower ID), best first.
	for i := 1; i < len(rot); i++ {
		a, b := rot[i-1], rot[i]
		if a.Overall() < b.Overall() || (a.Overall() == b.Overall() && a.ID > b.ID) {
			t.Fatalf("rotation not sorted at %d: %d (%d) before %d (%d)", i, a.ID, a.Overall(), b.ID, b.Overall())
		}
	}
	got := map[int]bool{}
	for _, p := range rot[:5] {
		got[p.ID] = true
	}
	for _, id := range []int{2, 3, 4, 5, 6} {
		if !got[id] {
			t.Errorf("player %d should start", id)
		}
	}
	for _, id := range []int{0, 1, 7} {
		if got[id] {
			t.Errorf("weak player %d should not start", id)
		}
	}
}

func TestStarterSlotsSensible(t *testing.T) {
	tm := slotTeam()
	slots := tm.StarterSlots()
	if slots[C] == nil || slots[C].ID != 5 {
		t.Fatalf("the pure centre should play C, got %+v", slots[C])
	}
	if slots[PF] == nil || slots[PF].ID != 6 {
		t.Fatalf("the forward should play PF, got %+v", slots[PF])
	}
	seen := map[int]bool{}
	for _, p := range slots {
		if p == nil || seen[p.ID] {
			t.Fatalf("slots must hold five different players: %+v", slots)
		}
		seen[p.ID] = true
	}
	for _, p := range tm.Rotation()[:5] {
		if !seen[p.ID] {
			t.Fatalf("slot player set differs from the top five (missing %d)", p.ID)
		}
	}
	again := slotTeam().StarterSlots()
	for i := range slots {
		if slots[i].ID != again[i].ID {
			t.Fatalf("slot %d not deterministic", i)
		}
	}
}

func TestAutoLineupSeatsBestFive(t *testing.T) {
	lg := lineupLeague(4)
	for tid, tm := range lg.Teams[:6] {
		l := lg.AutoLineup(tid)
		rot := tm.Rotation()
		top := map[int]bool{}
		for _, p := range rot[:5] {
			top[p.ID] = true
		}
		for _, id := range l.Starters {
			if !top[id] {
				t.Fatalf("team %d: starter %d is not among the five best", tid, id)
			}
		}
	}
}
