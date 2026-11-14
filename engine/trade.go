package engine

import (
	"fmt"
	"math"
	"sort"
)

// Trade tunables.
const (
	TradeMargin        = 0.05 // the computer needs to receive 5% more value than it gives
	UntouchableMargin  = 0.25 // ... and 25% more to move one of its top 2 players
	TradeDeadlineDay   = 50   // trades are open while at most this many days are played (about 50 of 82 games)
	MaxTradePlayers    = 3    // players per side
	tradeOffset        = 7000 // RNG stream for offseason AI trades (see FinishOffseason)
	MidSeasonTradeStep = 7500 // suggested RNG offset for a mid-season SimAITrades call
)

// TradeOffer is a proposal: FromTeam gives Give and receives Get from ToTeam.
// FromTeam must be the user's team; ToTeam is a computer team.
type TradeOffer struct {
	FromTeam, ToTeam int
	Give, Get        []int // player IDs
}

// TradeVerdict is the computer team's answer. Values are on the computer
// team's own valuation (TheyGive/TheyGet) and the user team's (YouGive/YouGet).
type TradeVerdict struct {
	Valid   bool
	Problem string
	Accept  bool
	Reason  string

	TheyGive, TheyGet float64
	YouGive, YouGet   float64
	Needed            float64 // multiple of TheyGive that TheyGet must reach
}

// TxPlayer is a player in a transaction. From/To are team IDs, -1 = free agency.
type TxPlayer struct {
	PlayerID int
	Name     string
	From, To int
}

// TradeResult is one executed trade, as stored in League.Transactions.
type TradeResult struct {
	Year, Day    int
	TeamA, TeamB int
	Moved        []TxPlayer
	Released     []TxPlayer
	Signed       []TxPlayer

	// Added by deals (deal.go). Empty for plain two-team player trades, so old
	// saves load unchanged. Teams lists every team in a deal (TeamA and TeamB
	// are the first two); Picks lists the draft picks that changed hands.
	Teams []int    `json:",omitempty"`
	Picks []TxPick `json:",omitempty"`
}

// TxPick is a draft pick in a transaction (From/To are team IDs).
type TxPick struct {
	Year, Round, Original int
	From, To              int
}

// TradeAsset is a player on a team's trade block.
type TradeAsset struct {
	PlayerID          int
	Name, Pos         string
	Age, Overall, Pot int
	Value             float64
	Untouchable       bool
}

// TradeWindowOpen says whether trades are allowed right now, and why not.
func TradeWindowOpen(lg *League) (bool, string) {
	if lg.Draft != nil {
		return false, "The offseason draft is in progress; trades reopen when the offseason is done."
	}
	s := lg.Season
	if s == nil || s.Year < lg.Year {
		return true, "The offseason trade window is open."
	}
	if s.PlayoffsStarted || s.Done() {
		return false, "The trade window is closed for the playoffs and offseason report."
	}
	if s.Day > TradeDeadlineDay {
		return false, fmt.Sprintf("The trade deadline (day %d) has passed.", TradeDeadlineDay)
	}
	return true, "The trade window is open."
}

func teamTop8(t *Team) float64 {
	o := make([]int, 0, len(t.Roster))
	for _, p := range t.Roster {
		o = append(o, p.Overall())
	}
	sort.Sort(sort.Reverse(sort.IntSlice(o)))
	n := min(8, len(o))
	if n == 0 {
		return 0
	}
	s := 0
	for _, v := range o[:n] {
		s += v
	}
	return float64(s) / float64(n)
}

// contendFactors returns 0 (rebuilding) to 1 (contending) per team, from the
// rank of its top-8 average overall, blended with record once 20 games in.
func (lg *League) contendFactors() []float64 {
	n := len(lg.Teams)
	ids := make([]int, n)
	str := make([]float64, n)
	for i := range ids {
		ids[i] = i
		str[i] = teamTop8(lg.Teams[i])
	}
	sort.SliceStable(ids, func(a, b int) bool { return str[ids[a]] > str[ids[b]] })
	out := make([]float64, n)
	for rank, id := range ids {
		out[id] = 1 - float64(rank)/float64(n-1)
	}
	if s := lg.Season; s != nil && s.Year == lg.Year && len(s.Wins) == n {
		played := 0
		for i := range s.Wins {
			played = max(played, s.Wins[i]+s.Losses[i])
		}
		if played >= 20 {
			for rank, id := range s.Standings() {
				out[id] = 0.5*out[id] + 0.5*(1-float64(rank)/float64(n-1))
			}
		}
	}
	return out
}

func absContend(t *Team) float64 {
	return math.Max(0, math.Min(1, (teamTop8(t)-56)/12))
}

// tradeValue is the core valuation. others is the rest of the team's roster.
func tradeValue(p *Player, contend float64, others []*Player) float64 {
	ovr, pot := float64(p.Overall()), float64(p.Potential)
	eff := ovr
	if p.Age < 25 && pot > ovr {
		yw := math.Min(1, float64(25-p.Age)/5)
		eff += (pot - ovr) * yw * (0.25 + 0.5*(1-contend))
	}
	v := 1 + math.Pow(math.Max(0, eff-45)/10, 3.2)
	if p.Age >= 30 {
		v *= math.Max(0.3, 1-0.06*float64(p.Age-29)-0.06*float64(max(0, p.Age-33)))
	}
	if p.Age >= 29 {
		v *= 1 - 0.2*(1-contend)*math.Min(1, float64(p.Age-28)/3)
	}
	n := 0
	for _, q := range others {
		if q != p && q.CanPlay(p.Pos) {
			n++
		}
	}
	if n <= 2 {
		v *= 1.05 // fills a thin position
	}
	return v * contractAdjust(p, contend)
}

// TradeValue is a player's worth to a team from visible data only, with the
// team's context judged from its roster alone. (Trade evaluation itself also
// uses league rank and record.)
func TradeValue(p *Player, forTeam *Team) float64 {
	return tradeValue(p, absContend(forTeam), forTeam.Roster)
}

func (lg *League) value(p *Player, tid int, cf []float64) float64 {
	return tradeValue(p, cf[tid], lg.Teams[tid].Roster)
}

func (lg *League) untouchables(tid int, cf []float64) map[int]bool {
	ps := append([]*Player(nil), lg.Teams[tid].Roster...)
	vals := map[int]float64{}
	for _, p := range ps {
		vals[p.ID] = lg.value(p, tid, cf)
	}
	sort.SliceStable(ps, func(i, j int) bool {
		if vals[ps[i].ID] != vals[ps[j].ID] {
			return vals[ps[i].ID] > vals[ps[j].ID]
		}
		return ps[i].ID < ps[j].ID
	})
	m := map[int]bool{}
	for i := 0; i < 2 && i < len(ps); i++ {
		m[ps[i].ID] = true
	}
	return m
}

// TradeBlock lists a team's players with their value to that team.
func TradeBlock(lg *League, teamID int) []TradeAsset {
	if teamID < 0 || teamID >= len(lg.Teams) {
		return nil
	}
	cf := lg.contendFactors()
	un := lg.untouchables(teamID, cf)
	var out []TradeAsset
	for _, p := range lg.Teams[teamID].Roster {
		out = append(out, TradeAsset{p.ID, p.Name, p.PosLabel(), p.Age, p.Overall(), p.Potential,
			lg.value(p, teamID, cf), un[p.ID]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Value > out[j].Value })
	return out
}

func posCounts(ps []*Player) [5]int {
	var c [5]int
	for _, p := range ps {
		for _, pos := range p.Positions() {
			c[pos]++
		}
	}
	return c
}

// sideEval returns what team tid gives and gets, including a penalty for
// newly leaving fewer than 2 players at a position, and the acceptance
// threshold (higher if it moves an untouchable).
func (lg *League) sideEval(tid int, out, in []*Player, cf []float64) (gives, gets, need float64) {
	for _, p := range out {
		gives += lg.value(p, tid, cf)
	}
	for _, p := range in {
		gets += lg.value(p, tid, cf)
	}
	outSet := map[int]bool{}
	for _, p := range out {
		outSet[p.ID] = true
	}
	var after []*Player
	for _, p := range lg.Teams[tid].Roster {
		if !outSet[p.ID] {
			after = append(after, p)
		}
	}
	after = append(after, in...)
	before, now := posCounts(lg.Teams[tid].Roster), posCounts(after)
	for i := range before {
		if now[i] < 2 && now[i] < before[i] {
			gives *= 1.08
		}
	}
	need = 1 + TradeMargin
	un := lg.untouchables(tid, cf)
	for _, p := range out {
		if un[p.ID] {
			need = 1 + UntouchableMargin
		}
	}
	return
}

func (lg *League) players(tid int, ids []int) ([]*Player, bool) {
	var out []*Player
	seen := map[int]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, false
		}
		seen[id] = true
		var found *Player
		for _, p := range lg.Teams[tid].Roster {
			if p.ID == id {
				found = p
			}
		}
		if found == nil {
			return nil, false
		}
		out = append(out, found)
	}
	return out, true
}

func describe(p *Player) string {
	return fmt.Sprintf("%s, a %d-year-old rated %d", p.Name, p.Age, p.Overall())
}

// EvaluateTrade asks the computer team (offer.ToTeam) whether it accepts.
func EvaluateTrade(lg *League, offer TradeOffer) TradeVerdict {
	bad := func(msg string) TradeVerdict { return TradeVerdict{Problem: msg} }
	if ok, why := TradeWindowOpen(lg); !ok {
		return bad(why)
	}
	n := len(lg.Teams)
	if offer.FromTeam < 0 || offer.FromTeam >= n || offer.ToTeam < 0 || offer.ToTeam >= n || offer.FromTeam == offer.ToTeam {
		return bad("That is not a valid pair of teams.")
	}
	if offer.FromTeam != lg.UserTeamID || offer.ToTeam == lg.UserTeamID {
		return bad("Trades are between your team and a computer team.")
	}
	if len(offer.Give) < 1 || len(offer.Give) > MaxTradePlayers || len(offer.Get) < 1 || len(offer.Get) > MaxTradePlayers {
		return bad(fmt.Sprintf("Each side must include 1 to %d players.", MaxTradePlayers))
	}
	give, ok1 := lg.players(offer.FromTeam, offer.Give)
	get, ok2 := lg.players(offer.ToTeam, offer.Get)
	if !ok1 || !ok2 {
		return bad("A player in the offer is not on the team you named, or is listed twice.")
	}
	if d := len(offer.Get) - len(offer.Give); d > len(lg.FreeAgents) || -d > len(lg.FreeAgents) {
		return bad("Not enough free agents to fill the short roster.")
	}
	if err := lg.salaryProblem(offer.FromTeam, give, get); err != nil {
		return bad(err.Error())
	}
	if err := lg.salaryProblem(offer.ToTeam, get, give); err != nil {
		return bad(err.Error())
	}
	cf := lg.contendFactors()
	gives, gets, need := lg.sideEval(offer.ToTeam, get, give, cf)
	yg, yt, _ := lg.sideEval(offer.FromTeam, give, get, cf)
	v := TradeVerdict{Valid: true, TheyGive: gives, TheyGet: gets, YouGive: yg, YouGet: yt, Needed: need}
	v.Accept = gets >= gives*need
	un := lg.untouchables(offer.ToTeam, cf)
	best := get[0]
	for _, p := range get {
		if lg.value(p, offer.ToTeam, cf) > lg.value(best, offer.ToTeam, cf) {
			best = p
		}
	}
	incomingBest := give[0]
	for _, p := range give {
		if lg.value(p, offer.ToTeam, cf) > lg.value(incomingBest, offer.ToTeam, cf) {
			incomingBest = p
		}
	}
	switch {
	case v.Accept && lg.value(incomingBest, offer.ToTeam, cf) >= lg.value(best, offer.ToTeam, cf):
		v.Reason = "They accept: they get the best player in the deal."
	case v.Accept:
		v.Reason = "They accept: the deal comes out ahead for them on value."
	case un[best.ID] && need > 1+TradeMargin:
		v.Reason = fmt.Sprintf("They see %s as a core player and want a much bigger offer.", describe(best))
	default:
		v.Reason = fmt.Sprintf("They want more for %s.", describe(best))
	}
	return v
}

// SuggestCounter finds the smallest change to the user's side (add one of
// your players, or swap one for a more valuable player) that gets a refused
// offer accepted.
func SuggestCounter(lg *League, offer TradeOffer) (TradeOffer, bool) {
	v := EvaluateTrade(lg, offer)
	if !v.Valid || v.Accept {
		return offer, false
	}
	var best TradeOffer
	bestVal, found := 0.0, false
	try := func(give []int) {
		o := TradeOffer{offer.FromTeam, offer.ToTeam, give, offer.Get}
		if r := EvaluateTrade(lg, o); r.Valid && r.Accept && (!found || r.TheyGet < bestVal) {
			best, bestVal, found = o, r.TheyGet, true
		}
	}
	inOffer := map[int]bool{}
	for _, id := range offer.Give {
		inOffer[id] = true
	}
	for _, p := range lg.Teams[offer.FromTeam].Roster {
		if inOffer[p.ID] {
			continue
		}
		if len(offer.Give) < MaxTradePlayers {
			try(append(append([]int(nil), offer.Give...), p.ID))
		}
		for i := range offer.Give {
			g := append([]int(nil), offer.Give...)
			g[i] = p.ID
			try(g)
		}
	}
	return best, found
}

func (lg *League) currentDay() int {
	if s := lg.Season; s != nil && s.Year == lg.Year {
		return s.Day
	}
	return 0
}

// applyTrade swaps the players and fixes both rosters back to 13.
func (lg *League) applyTrade(a, b int, give, get []*Player) TradeResult {
	res := TradeResult{Year: lg.Year, Day: lg.currentDay(), TeamA: a, TeamB: b}
	move := func(from, to int, ps []*Player) {
		ids := map[int]bool{}
		for _, p := range ps {
			ids[p.ID] = true
			res.Moved = append(res.Moved, TxPlayer{p.ID, p.Name, from, to})
		}
		var keep []*Player
		for _, p := range lg.Teams[from].Roster {
			if !ids[p.ID] {
				keep = append(keep, p)
			}
		}
		lg.Teams[from].Roster = keep
	}
	move(a, b, give)
	move(b, a, get)
	lg.Teams[b].Roster = append(lg.Teams[b].Roster, give...)
	lg.Teams[a].Roster = append(lg.Teams[a].Roster, get...)
	for _, p := range give {
		p.TeamID = b
	}
	for _, p := range get {
		p.TeamID = a
	}
	lg.fixRosters([]int{a, b}, &res)
	lg.Transactions = append(lg.Transactions, res)
	return res
}

// fixRosters trims any listed team above RosterSize (releasing its least
// valuable players to free agency) and fills any below it from the free
// agents, recording both in res.
func (lg *League) fixRosters(tids []int, res *TradeResult) {
	cf := lg.contendFactors()
	// A player who just arrived in this trade is never the one released to
	// make room: the team wanted him, so it cuts someone it already had.
	arrived := map[int]bool{}
	for _, m := range res.Moved {
		arrived[m.PlayerID] = true
	}
	for _, tid := range tids {
		t := lg.Teams[tid]
		if len(t.Roster) > RosterSize {
			sort.SliceStable(t.Roster, func(i, j int) bool {
				if ai, aj := arrived[t.Roster[i].ID], arrived[t.Roster[j].ID]; ai != aj {
					return ai
				}
				vi, vj := lg.value(t.Roster[i], tid, cf), lg.value(t.Roster[j], tid, cf)
				if vi != vj {
					return vi > vj
				}
				return t.Roster[i].ID < t.Roster[j].ID
			})
			for _, p := range t.Roster[RosterSize:] {
				// Simplification: a released player's contract just ends (no dead money).
				lg.releaseToFA(p)
				res.Released = append(res.Released, TxPlayer{p.ID, p.Name, tid, -1})
			}
			t.Roster = t.Roster[:RosterSize]
		}
	}
	// Fill after every team has trimmed, so players released by one team in
	// the deal are available to a team the deal left short.
	for _, tid := range tids {
		t := lg.Teams[tid]
		for len(t.Roster) < RosterSize && len(lg.FreeAgents) > 0 {
			bi := 0
			for i, p := range lg.FreeAgents {
				q := lg.FreeAgents[bi]
				if p.Overall() > q.Overall() || (p.Overall() == q.Overall() && p.ID < q.ID) {
					bi = i
				}
			}
			p := lg.FreeAgents[bi]
			// A free agent signed to fill a roster gets a one-year minimum deal
			// (even past the hard ceiling, so a roster can always be filled).
			lg.forceSign(tid, p, Cap.MinSalary, 1)
			res.Signed = append(res.Signed, TxPlayer{p.ID, p.Name, -1, tid})
		}
	}
}

// ExecuteTrade re-validates the offer, requires the computer team to accept
// it, then carries it out.
func ExecuteTrade(lg *League, offer TradeOffer) (TradeResult, error) {
	v := EvaluateTrade(lg, offer)
	if !v.Valid {
		return TradeResult{}, fmt.Errorf("%s", v.Problem)
	}
	if !v.Accept {
		return TradeResult{}, fmt.Errorf("%s", v.Reason)
	}
	give, _ := lg.players(offer.FromTeam, offer.Give)
	get, _ := lg.players(offer.ToTeam, offer.Get)
	return lg.applyTrade(offer.FromTeam, offer.ToTeam, give, get), nil
}

type aiCand struct {
	a, b    *Player
	ta, tb  int
	surplus float64
}

// SimAITrades makes 0-2 one-for-one trades between computer teams where both
// sides gain by their own valuation. Never involves the user's team. Use an
// RNG derived from (seed, year, a distinct offset): FinishOffseason uses
// tradeOffset; a mid-season call can use
// NewRNG(lg.Seed*1000003 + uint64(lg.Year)*10007 + MidSeasonTradeStep).
func (lg *League) SimAITrades(r *RNG) []TradeResult {
	want := r.IntN(3)
	var done []TradeResult
	usedTeam := map[int]bool{lg.UserTeamID: true}
	usedPlayer := map[int]bool{}
	for len(done) < want {
		cf := lg.contendFactors()
		un := make([]map[int]bool, len(lg.Teams))
		for i := range lg.Teams {
			un[i] = lg.untouchables(i, cf)
		}
		var cands []aiCand
		for i := 0; i < len(lg.Teams); i++ {
			for j := i + 1; j < len(lg.Teams); j++ {
				if usedTeam[i] || usedTeam[j] {
					continue
				}
				for _, p := range lg.Teams[i].Roster {
					if un[i][p.ID] || usedPlayer[p.ID] {
						continue
					}
					pa, pb := lg.value(p, i, cf), lg.value(p, j, cf)
					for _, q := range lg.Teams[j].Roster {
						if un[j][q.ID] || usedPlayer[q.ID] {
							continue
						}
						qa, qb := lg.value(q, i, cf), lg.value(q, j, cf)
						ga, gb := qa/pa, pb/qb
						if ga >= 1+TradeMargin+0.05 && gb >= 1+TradeMargin+0.05 {
							cands = append(cands, aiCand{p, q, i, j, (ga - 1) + (gb - 1)})
						}
					}
				}
			}
		}
		sort.SliceStable(cands, func(x, y int) bool {
			if cands[x].surplus != cands[y].surplus {
				return cands[x].surplus > cands[y].surplus
			}
			if cands[x].a.ID != cands[y].a.ID {
				return cands[x].a.ID < cands[y].a.ID
			}
			return cands[x].b.ID < cands[y].b.ID
		})
		var valid []aiCand
		for _, c := range cands {
			ga, gt, na := lg.sideEval(c.ta, []*Player{c.a}, []*Player{c.b}, cf)
			ha, ht, nb := lg.sideEval(c.tb, []*Player{c.b}, []*Player{c.a}, cf)
			if gt >= ga*na && ht >= ha*nb &&
				lg.salaryProblem(c.ta, []*Player{c.a}, []*Player{c.b}) == nil &&
				lg.salaryProblem(c.tb, []*Player{c.b}, []*Player{c.a}) == nil {
				valid = append(valid, c)
				if len(valid) == 6 {
					break
				}
			}
		}
		if len(valid) == 0 {
			break
		}
		c := valid[r.IntN(len(valid))]
		done = append(done, lg.applyTrade(c.ta, c.tb, []*Player{c.a}, []*Player{c.b}))
		usedTeam[c.ta], usedTeam[c.tb] = true, true
		usedPlayer[c.a.ID], usedPlayer[c.b.ID] = true, true
	}
	return done
}
