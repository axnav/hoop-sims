package engine

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func moraleSnapshot(lg *League) []int {
	var out []int
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			out = append(out, p.Morale, p.TradeRequest, p.MoraleStreak)
		}
	}
	return out
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMoraleWeeklyDrift(t *testing.T) {
	lg := GenerateLeague(3)
	lg.StartSeason()
	p := lg.Teams[4].Roster[0]
	for _, start := range []int{-40, 40} {
		p.Morale = start
		gap := lg.moraleCalcOf(p).target - float64(start)
		step := gap * MoraleTuning.Share
		if math.Abs(step) < 1 && math.Abs(gap) >= 1 {
			step = math.Copysign(1, gap)
		}
		want := start + int(math.Round(step))
		lg.moraleWeek()
		if p.Morale != want {
			t.Errorf("start %d: morale %d, want %d (gap %.1f)", start, p.Morale, want, gap)
		}
	}
	// Only whole sim weeks update: day 6 changes nothing, day 7 does.
	lg2 := GenerateLeague(3)
	lg2.StartSeason()
	lg2.SimDays(6)
	if s := moraleSnapshot(lg2); !sameInts(s, make([]int, len(s))) {
		t.Error("morale changed before the first week was over")
	}
	lg2.SimDay()
	moved := 0
	for _, tm := range lg2.Teams {
		for _, q := range tm.Roster {
			if q.Morale != 0 {
				moved++
			}
		}
	}
	if moved < 100 {
		t.Errorf("only %d players moved after one week", moved)
	}
}

func TestMoraleDeterministicAndSaves(t *testing.T) {
	run := func() *League {
		lg := GenerateLeague(8)
		lg.StartSeason()
		lg.SimDays(45)
		return lg
	}
	a, b := run(), run()
	if !sameInts(moraleSnapshot(a), moraleSnapshot(b)) {
		t.Fatal("same seed, different morale")
	}
	// Save and load in the middle of a season: the rest is identical.
	path := filepath.Join(t.TempDir(), "league.json")
	if err := a.Save(path); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !sameInts(moraleSnapshot(a), moraleSnapshot(c)) {
		t.Fatal("morale changed through save and load")
	}
	a.SimDays(30)
	c.SimDays(30)
	if !sameInts(moraleSnapshot(a), moraleSnapshot(c)) {
		t.Fatal("after a load the season went differently (morale)")
	}
	for i := range a.Season.Wins {
		if a.Season.Wins[i] != c.Season.Wins[i] {
			t.Fatalf("team %d wins %d vs %d after a load", i, a.Season.Wins[i], c.Season.Wins[i])
		}
	}
	if len(a.Events) != len(c.Events) {
		t.Errorf("events %d vs %d", len(a.Events), len(c.Events))
	}
}

func TestMoraleOmittedWhenContent(t *testing.T) {
	b, _ := json.Marshal(&Player{Name: "A"})
	for _, k := range []string{"Morale", "TradeRequest", "MoraleStreak"} {
		if strings.Contains(string(b), k) {
			t.Errorf("%s saved for a content player: %s", k, b)
		}
	}
	var p Player
	if err := json.Unmarshal([]byte(`{"ID":1,"Name":"Old"}`), &p); err != nil || p.Morale != 0 || p.TradeRequest != 0 {
		t.Errorf("old save: %v %+v", err, p)
	}
}

func TestTradeRequestStartsAndEnds(t *testing.T) {
	lg := GenerateLeague(2)
	lg.StartSeason()
	saved := MoraleTuning
	defer func() { MoraleTuning = saved }()
	MoraleTuning.RequestChance = 1
	p := lg.Teams[6].Roster[3]

	// Two weeks already at the bottom: the third week starts a request.
	p.Morale, p.MoraleStreak = -50, MoraleTuning.RequestWeeks-1
	lg.moraleWeek()
	if p.TradeRequest == 0 {
		t.Fatalf("no request after %d weeks at the bottom (morale %d)", MoraleTuning.RequestWeeks, p.Morale)
	}
	n := len(lg.Events)
	if n == 0 || lg.Events[n-1].Type != EvTradeRequest || lg.Events[n-1].PlayerID != p.ID {
		t.Fatalf("event %v", lg.Events)
	}
	// Not before: one week short.
	q := lg.Teams[9].Roster[3]
	q.Morale, q.MoraleStreak = -50, MoraleTuning.RequestWeeks-2
	lg.moraleWeek()
	if q.TradeRequest != 0 {
		t.Error("request started a week early")
	}
	// A happy week resets the count.
	q.Morale, q.MoraleStreak = 40, 2
	lg.moraleWeek()
	if q.MoraleStreak != 0 {
		t.Errorf("streak %d after a good week", q.MoraleStreak)
	}

	// Pressure: while the request stands his target carries an extra minus,
	// growing by the week, and a teammate's target a small one.
	mate := lg.Teams[6].Roster[4]
	with := lg.moraleCalcOf(mate).target
	p.TradeRequest = 0
	without := lg.moraleCalcOf(mate).target
	p.TradeRequest = lg.requestMarker()
	if with > without || (without > float64(MoraleTuning.Min)+10 && with >= without) {
		t.Errorf("teammate target %.1f with a requester, %.1f without", with, without)
	}

	// Recovery: weeks in a row above the recovery line end it.
	r := lg.Teams[6].Roster[5]
	r.Morale, r.TradeRequest, r.MoraleStreak = 40, lg.requestMarker(), MoraleTuning.RecoverWeeks-1
	lg.moraleWeek()
	if r.TradeRequest != 0 {
		t.Errorf("request still on after recovering (morale %d)", r.Morale)
	}
	if e := lg.Events[len(lg.Events)-1]; e.Type != EvRequestWithdrawn || e.PlayerID != r.ID {
		t.Errorf("last event %v", e)
	}
}

func TestTradeRequestClearedOnTrade(t *testing.T) {
	lg := GenerateLeague(4)
	lg.StartSeason()
	a, b := lg.Teams[1], lg.Teams[2]
	p, q := a.Roster[2], b.Roster[2]
	p.Morale, p.TradeRequest, p.MoraleStreak = -40, lg.requestMarker(), 2
	q.Morale = 30
	lg.applyTrade(1, 2, []*Player{p}, []*Player{q})
	if p.TradeRequest != 0 || p.MoraleStreak != 0 {
		t.Errorf("request survived the trade: %+v", p)
	}
	if p.Morale != -20 || q.Morale != 15 {
		t.Errorf("morale after a trade: %d and %d, want -20 and 15", p.Morale, q.Morale)
	}
	e := lg.Events[len(lg.Events)-1]
	if e.Type != EvRequestTraded || e.PlayerID != p.ID || e.TeamID != 1 {
		t.Errorf("event %v", e)
	}
	// Released: reset too.
	p.Morale, p.TradeRequest = -40, 5
	lg.releaseToFA(p)
	if p.TradeRequest != 0 || p.Morale != -20 {
		t.Errorf("release: %+v", p)
	}
}

func TestTradeRequestLowersTradeValue(t *testing.T) {
	lg := GenerateLeague(6)
	p := lg.Teams[3].Roster[0]
	before := TradeValue(p, lg.Teams[3])
	p.TradeRequest = 1
	after := TradeValue(p, lg.Teams[3])
	want := before * (1 - MoraleTuning.RequestDiscount)
	if math.Abs(after-want) > 1e-9 {
		t.Errorf("value %.3f, want %.3f (before %.3f)", after, want, before)
	}
}

func TestRequesterMovedByComputerTeams(t *testing.T) {
	// With a requester on a computer team and the discount in place, the
	// computer teams can swap him away; the request is gone afterwards.
	moved := 0
	for seed := uint64(1); seed <= 6 && moved == 0; seed++ {
		lg := GenerateLeague(seed)
		lg.StartSeason()
		for _, tm := range lg.Teams {
			if tm.ID == lg.UserTeamID {
				continue
			}
			p := tm.Roster[len(tm.Roster)-1]
			p.Morale, p.TradeRequest = -40, lg.requestMarker()
		}
		r := NewRNG(seed)
		if res, ok := lg.aiTradeOnce(r, map[int]bool{lg.UserTeamID: true}, map[int]bool{}, true); ok {
			moved++
			for _, m := range res.Moved {
				if p := lg.FindPlayer(m.PlayerID); p.TradeRequest != 0 {
					t.Errorf("%s still has a request after the swap", p.Name)
				}
			}
		}
	}
	if moved == 0 {
		t.Error("no requester was ever moved by a computer team")
	}
}

func TestMoraleFeedsStanceAndAsk(t *testing.T) {
	lg := GenerateLeague(9)
	lg.StartSeason()
	flipped := false
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			p.Morale = 0
			base := lg.StanceOf(p)
			p.Morale = -50
			low := lg.StanceOf(p)
			p.Morale = 50
			high := lg.StanceOf(p)
			p.Morale = 0
			if base.Key != "test" && low.Key == "test" {
				flipped = true
				found := false
				for _, r := range low.Reasons {
					found = found || r == "Unhappy with his situation"
				}
				if !found {
					t.Errorf("%s: stance turned to test without the morale reason: %v", p.Name, low.Reasons)
				}
			}
			if base.Key == "test" && high.Key != "test" {
				flipped = true
			}
			rank := map[string]int{"test": 0, "open": 1, "resign": 2}
			if rank[low.Key] > rank[base.Key] || rank[high.Key] < rank[base.Key] {
				t.Errorf("%s: stance %s at 0, %s at -50, %s at +50", p.Name, base.Key, low.Key, high.Key)
			}
		}
	}
	if !flipped {
		t.Error("morale never changed a stance in the whole league")
	}
	// The ask: more when unhappy, less when happy.
	p := &Player{}
	if f := moraleAskFactor(p); f != 1 {
		t.Errorf("content factor %v", f)
	}
	p.Morale = -50
	lo := moraleAskFactor(p)
	p.Morale = 50
	hi := moraleAskFactor(p)
	if !(lo > 1 && hi < 1) {
		t.Errorf("ask factors %v unhappy, %v happy", lo, hi)
	}
}

func TestMoraleEffectIsCentred(t *testing.T) {
	lg := GenerateLeague(12)
	lg.StartSeason()
	i := 0
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			p.Morale = (i*7)%21 - 3 // a spread that stays inside the cap
			i++
		}
	}
	lg.mmeans, lg.bmeans = nil, nil
	mm := lg.moraleMeansNow().mean
	if mm == 0 {
		t.Fatal("mean mood is zero for a lopsided spread")
	}
	sum, w := 0.0, 0.0
	for _, tm := range lg.Teams {
		for _, pm := range tm.EffectiveMinutes() {
			sum += float64(pm.Min) * moraleEffect(pm.Player, mm)
			w += float64(pm.Min)
		}
	}
	if math.Abs(sum/w) > 1e-9 {
		t.Errorf("minutes-weighted effect %.3g, want 0", sum/w)
	}
	// The cap holds at the extremes, and content in a content league is zero.
	if e := moraleEffect(&Player{Morale: 50}, -50); e > MoraleTuning.EffectCap+1e-12 || e < MoraleTuning.EffectCap-1e-12 {
		t.Errorf("extreme effect %v, cap %v", e, MoraleTuning.EffectCap)
	}
	if moraleEffect(&Player{}, 0) != 0 {
		t.Error("content effect not zero")
	}
	// In a game the numbers reach the sides.
	h := newSide(lg, lg.Teams[0], true)
	if len(h.moraleAdd) != len(h.box.Lines) {
		t.Errorf("%d morale numbers for %d lines", len(h.moraleAdd), len(h.box.Lines))
	}
}

func TestMoraleInjuredNotPunished(t *testing.T) {
	lg := GenerateLeague(5)
	lg.StartSeason()
	tm := lg.Teams[2]
	p := tm.Roster[len(tm.Roster)-1] // deep bench
	healthy := lg.moraleCalcOf(p).target
	p.Injury = &Injury{Name: "Sprain", GamesLeft: 20}
	hurt := lg.moraleCalcOf(p).target
	if hurt < healthy {
		t.Errorf("injured target %.1f below healthy %.1f", hurt, healthy)
	}
}

func TestMoraleLabelsAndReasons(t *testing.T) {
	for m, want := range map[int]string{-50: "Furious", -31: "Furious", -30: "Unhappy", -11: "Unhappy", -10: "Content", 9: "Content", 10: "Happy", 29: "Happy", 30: "Thrilled", 50: "Thrilled"} {
		if got := MoraleLabel(m); got != want {
			t.Errorf("label(%d) = %s, want %s", m, got, want)
		}
	}
	lg := GenerateLeague(5)
	lg.StartSeason()
	lg.SimDays(30)
	n := 0
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			info, ok := lg.MoraleOf(p)
			if !ok || len(info.Reasons) > 4 || info.Score != 50+p.Morale {
				t.Fatalf("%s: %+v ok=%v", p.Name, info, ok)
			}
			n += len(info.Reasons)
		}
	}
	if n == 0 {
		t.Error("no reasons anywhere")
	}
	if _, ok := lg.MoraleOf(&Player{TeamID: -1}); ok {
		t.Error("a free agent has morale")
	}
}
