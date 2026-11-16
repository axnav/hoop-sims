package engine

import (
	"fmt"
	"sort"
	"strings"
)

// The trade assistant: three read-only helpers for the Trade screens.
//
//   - NeedsOf: what a computer team wants (its direction, what it values, what
//     it lacks), from state that already exists.
//   - WhatWouldItTake: the cheapest packages from the user's side that a team
//     would accept for one of its players. It only proposes a package the real
//     rules accept: every candidate is checked with the same salary rules
//     (salaryProblem), the same value test (sideEval) and finally with
//     evaluateDealBase, the code EvaluateDeal and ExecuteDeal use.
//   - AdvisorOf: Keep / Shop / Sell for each player on the user's roster.
//
// Nothing is saved and nothing is random. The hidden Impact is never read.

// AssistTuning holds the numbers behind NeedsOf and WhatWouldItTake. Edit here.
var AssistTuning = struct {
	// Needs: a skill or position counts as a lack when the team ranks in the
	// bottom WeakShare of the league on it.
	WeakShare float64
	// A position is thin when this many (or fewer) of the team's ten best
	// players can play it.
	ThinAt int
	// Payroll above this asks for cap relief.
	CapReliefPayroll int
	// What would it take: most picks in a package (players use MaxTradePlayers).
	MaxPicks int
	// A package piece the team values (young player for a rebuilding team, pick,
	// veteran for a contender) counts this much less against the user.
	LikeDiscount float64
	// Each asset in a package adds this to its cost (fewer pieces are better).
	PieceCost float64
	// Young: player age at or below this is "young"; veteran: at or above.
	YoungAge, VeteranAge int
	// Candidates tried per strategy before giving up (salary-legal ones).
	MaxTries int
	// Most packages returned.
	MaxPackages int
}{
	WeakShare: 0.25, ThinAt: 1, CapReliefPayroll: 160000,
	MaxPicks: 2, LikeDiscount: 0.12, PieceCost: 0.4,
	YoungAge: 24, VeteranAge: 27,
	MaxTries: 60, MaxPackages: 4,
}

var posNames = [5]string{"point guard", "shooting guard", "small forward", "power forward", "center"}

// skillDef is one skill a team can lack: a rating (index of Ratings.array),
// the badges that show it, and the sentences.
type skillDef struct {
	name   string
	rating int
	badges []string
	none   string // no player in the rotation has a badge for it
	weak   string // the team's best three are weak against the league
}

var skillDefs = []skillDef{
	{"three", 2, []string{"spot_up", "deep_range", "catch_and_fire", "corner_specialist"}, "No reliable three-point shooters", "Weak three-point shooting"},
	{"rim", 5, []string{"rim_protector", "post_anchor"}, "No rim protector", "Weak rim protection"},
	{"perimeter", 5, []string{"lockdown", "perimeter_wall", "pickpocket", "passing_lane"}, "No lockdown perimeter defender", "Weak perimeter defense"},
	{"playmaking", 3, []string{"floor_general", "needle_threader", "lob_passer"}, "No true playmaker", "Little playmaking"},
	{"rebounding", 4, []string{"glass_cleaner", "box_out_boss"}, "No strong rebounder", "Weak rebounding"},
	{"inside", 0, []string{"rim_finisher", "power_inside", "post_craft"}, "No inside scorer", "Weak interior scoring"},
}

// rotationOf is a team's n best players by overall (ties: lower ID).
func rotationOf(t *Team, n int) []*Player {
	ps := append([]*Player(nil), t.Roster...)
	sort.SliceStable(ps, func(i, j int) bool {
		if a, b := ps[i].Overall(), ps[j].Overall(); a != b {
			return a > b
		}
		return ps[i].ID < ps[j].ID
	})
	if len(ps) > n {
		ps = ps[:n]
	}
	return ps
}

// topMean is the mean of the k largest values.
func topMean(v []float64, k int) float64 {
	s := append([]float64(nil), v...)
	sort.Sort(sort.Reverse(sort.Float64Slice(s)))
	if len(s) > k {
		s = s[:k]
	}
	if len(s) == 0 {
		return 0
	}
	t := 0.0
	for _, x := range s {
		t += x
	}
	return t / float64(len(s))
}

// teamProfile is what NeedsOf compares across the league.
type teamProfile struct {
	skill   [6]float64 // top-3 mean of the skill's rating in the rotation
	load    [6]int     // badge tiers for the skill in the rotation
	posBest [5]float64 // best overall at each position
	posN    [5]int     // players among the ten best who can play it
}

func (lg *League) profileOf(t *Team) teamProfile {
	var pr teamProfile
	rot := rotationOf(t, 8)
	sets := make([]BadgeSet, len(rot))
	for i, p := range rot {
		sets[i] = lg.badgeSet(p)
	}
	for si, d := range skillDefs {
		vals := make([]float64, len(rot))
		for i, p := range rot {
			vals[i] = float64(p.Ratings.array()[d.rating])
			for _, k := range d.badges {
				if idx := badgeIdx(k); idx >= 0 {
					pr.load[si] += int(sets[i].Tier[idx])
				}
			}
		}
		pr.skill[si] = topMean(vals, 3)
	}
	for pos := PG; pos <= C; pos++ {
		for _, p := range t.Roster {
			if p.CanPlay(pos) {
				pr.posBest[pos] = max(pr.posBest[pos], float64(p.OverallAt(pos)))
			}
		}
		for _, p := range rotationOf(t, 10) {
			if p.CanPlay(pos) {
				pr.posN[pos]++
			}
		}
	}
	return pr
}

// rankAmong is the rank (0 = best) of values[i] among values (higher is
// better; ties share the better rank).
func rankAmong(values []float64, i int) int {
	r := 0
	for _, v := range values {
		if v > values[i] {
			r++
		}
	}
	return r
}

// TeamNeeds is what a team wants, for the screens.
type TeamNeeds struct {
	Status  TeamStatus
	Values  []string
	Lacks   []string
	Summary string
}

// NeedsOf says what team teamID wants: its direction, what it values and what
// it lacks. Deterministic, from existing state only.
func (lg *League) NeedsOf(teamID int) TeamNeeds {
	if teamID < 0 || teamID >= len(lg.Teams) {
		return TeamNeeds{}
	}
	t := &AssistTuning
	tm := lg.Teams[teamID]
	st := lg.TeamStatusOf(teamID)
	n := TeamNeeds{Status: st, Values: []string{}, Lacks: []string{}}

	switch st.Key {
	case "contender", "buyer":
		n.Values = append(n.Values, "Win-now veterans")
	case "seller":
		n.Values = append(n.Values, "Draft picks", "Young players with upside")
	default:
		n.Values = append(n.Values, "Young players with upside", "Draft picks")
	}
	if lg.Payroll(teamID) > t.CapReliefPayroll {
		n.Values = append(n.Values, "Cap relief")
	}

	// Lacks: compare the team's profile with the league.
	profiles := make([]teamProfile, len(lg.Teams))
	for i, x := range lg.Teams {
		profiles[i] = lg.profileOf(x)
	}
	me := profiles[teamID]
	type lack struct {
		sev  float64
		text string
	}
	var lacks []lack
	cut := 1 - t.WeakShare
	nt := float64(len(lg.Teams))
	for si, d := range skillDefs {
		col := make([]float64, len(profiles))
		for i := range profiles {
			col[i] = profiles[i].skill[si]
		}
		frac := float64(rankAmong(col, teamID)) / nt
		switch {
		case me.load[si] == 0 && frac >= 0.5:
			lacks = append(lacks, lack{1 + frac, d.none})
		case frac >= cut:
			lacks = append(lacks, lack{frac, d.weak})
		}
	}
	for pos := PG; pos <= C; pos++ {
		col := make([]float64, len(profiles))
		for i := range profiles {
			col[i] = profiles[i].posBest[pos]
		}
		frac := float64(rankAmong(col, teamID)) / nt
		name := posNames[pos]
		switch {
		case me.posN[pos] <= t.ThinAt:
			lacks = append(lacks, lack{1.5, strings.ToUpper(name[:1]) + name[1:] + " depth"})
		case frac >= cut:
			lacks = append(lacks, lack{frac, "Weak at " + name})
		}
	}
	sort.SliceStable(lacks, func(i, j int) bool { return lacks[i].sev > lacks[j].sev })
	for _, l := range lacks {
		if len(n.Lacks) < 4 {
			n.Lacks = append(n.Lacks, l.text)
		}
	}

	low := make([]string, len(n.Values))
	for i, v := range n.Values {
		low[i] = strings.ToLower(v[:1]) + v[1:]
	}
	s := fmt.Sprintf("The %s are %s: they want %s", tm.Name, directionPhrase(st.Key), joinList(low))
	if len(n.Lacks) > 0 {
		lk := make([]string, len(n.Lacks))
		for i, v := range n.Lacks {
			lk[i] = strings.ToLower(v[:1]) + v[1:]
		}
		s += fmt.Sprintf(". Weak spots: %s.", strings.Join(lk, "; "))
	} else {
		s += ". No glaring weak spots."
	}
	n.Summary = s
	return n
}

// directionPhrase says a direction in a sentence ("are ...").
func directionPhrase(key string) string {
	switch key {
	case "contender":
		return "chasing a title"
	case "buyer":
		return "in the playoff race"
	case "seller":
		return "falling out of the race"
	}
	return "rebuilding"
}

// ---- what would it take ----

// TakePackage is one way to get the player: the user's Give for the target.
type TakePackage struct {
	Label    string
	Give     []Asset // from the user's team
	Receive  []Asset // from the other team (the target)
	Note     string
	Cost     float64 // what the user gives up, on the user's own valuation
	Cheapest bool
}

// Deal is the package as a deal between the user and team.
func (p TakePackage) Deal(user, team int) Deal {
	var d Deal
	for _, a := range p.Give {
		d.Moves = append(d.Moves, Move{user, team, a})
	}
	for _, a := range p.Receive {
		d.Moves = append(d.Moves, Move{team, user, a})
	}
	return d
}

// TakeResult is the answer to "what would it take?".
type TakeResult struct {
	Player   *Player
	Team     int
	Possible bool
	Reason   string
	Packages []TakePackage
}

type takeCand struct {
	a      Asset
	p      *Player // nil for a pick
	theirs float64 // value to the other team
	cost   float64 // cost to the user, after the "they like it" discount
	young  bool
	vet    bool
	core   bool
}

type takeCombo struct {
	cands   []*takeCand
	players int
	picks   int
	theirs  float64
	cost    float64
}

// WhatWouldItTake builds the cheapest packages from the user's roster and
// picks that the target's team accepts. Players the team treats as core (its
// top two) can still be asked for; the user's own core players (top two by
// value) are left out unless nothing else works. The error is for a target
// that cannot be asked about (not on a computer team).
func (lg *League) WhatWouldItTake(playerID int) (TakeResult, error) {
	user := lg.UserTeamID
	target := lg.FindPlayer(playerID)
	if target == nil || target.TeamID < 0 || target.TeamID >= len(lg.Teams) || lg.findRosterPlayer(target.TeamID, playerID) == nil {
		return TakeResult{}, fmt.Errorf("That player is not on a team.")
	}
	tid := target.TeamID
	if tid == user {
		return TakeResult{}, fmt.Errorf("He is already on your team.")
	}
	res := TakeResult{Player: target, Team: tid}
	if ok, why := TradeWindowOpen(lg); !ok {
		res.Reason = why
		return res, nil
	}
	if lg.Draft != nil && target.Contract.Years < 1 {
		res.Reason = target.Name + " has an expiring contract and cannot be traded during the draft."
		return res, nil
	}
	cf := lg.contendFactors()
	un := lg.untouchables(tid, cf)
	needs := lg.NeedsOf(tid)
	wantYoung := needs.Status.Key == "rebuilding" || needs.Status.Key == "seller"
	wantPicks := wantYoung
	wantVets := needs.Status.Key == "contender" || needs.Status.Key == "buyer"

	ownUn := lg.untouchables(user, cf)
	var cands []*takeCand
	for _, p := range lg.Teams[user].Roster {
		if lg.Draft != nil && p.Contract.Years < 1 {
			continue
		}
		c := &takeCand{a: PlayerOf(p.ID), p: p, theirs: lg.value(p, tid, cf), core: ownUn[p.ID]}
		c.cost = lg.value(p, user, cf)
		c.young = p.Age <= AssistTuning.YoungAge
		c.vet = p.Age >= AssistTuning.VeteranAge && p.Overall() >= tierRotationOvr
		if (c.young && p.Potential > p.Overall() && wantYoung) || (c.vet && wantVets) {
			c.cost *= 1 - AssistTuning.LikeDiscount
		}
		cands = append(cands, c)
	}
	for _, k := range lg.PicksOwnedBy(user) {
		a := PickOf(k.Year, k.Round, k.Original)
		c := &takeCand{a: a, theirs: lg.pickValue(a, tid, cf), cost: lg.pickValue(a, user, cf)}
		if wantPicks {
			c.cost *= 1 - AssistTuning.LikeDiscount
		}
		cands = append(cands, c)
	}

	baseGives, _, need := lg.sideEval(tid, []*Player{target}, nil, cf)
	baseGives = lg.value(target, tid, cf)
	threshold := baseGives * need

	diag := takeDiag{}
	build := func(allowCore bool) []TakePackage {
		var pool []*takeCand
		for _, c := range cands {
			if allowCore || !c.core {
				pool = append(pool, c)
			}
		}
		combos := lg.takeCombos(pool, threshold)
		type strat struct {
			label string
			ok    func(c *takeCombo) bool
		}
		strats := []strat{
			{"", func(c *takeCombo) bool { return true }},
			{"Players only", func(c *takeCombo) bool { return c.picks == 0 }},
			{"With picks", func(c *takeCombo) bool { return c.picks >= 1 && c.players <= 2 }},
			{"Young players", func(c *takeCombo) bool {
				if c.picks > 0 {
					return false
				}
				for _, x := range c.cands {
					if !x.young {
						return false
					}
				}
				return true
			}},
		}
		if wantVets {
			strats = append(strats, strat{"Veterans", func(c *takeCombo) bool {
				if c.picks > 0 {
					return false
				}
				for _, x := range c.cands {
					if !x.vet {
						return false
					}
				}
				return true
			}})
		}
		var out []TakePackage
		seen := map[string]bool{}
		for si, s := range strats {
			var list []*takeCombo
			for _, c := range combos {
				if c.players+c.picks > 0 && s.ok(c) {
					list = append(list, c)
				}
			}
			sort.SliceStable(list, func(i, j int) bool { return list[i].cost < list[j].cost })
			found := lg.firstAccepted(list, target, tid, user, cf, &diag)
			if found == nil {
				continue
			}
			key := comboKey(found)
			if seen[key] {
				continue
			}
			seen[key] = true
			pk := TakePackage{Label: s.label, Cost: 0, Cheapest: si == 0}
			for _, x := range found.cands {
				pk.Give = append(pk.Give, x.a)
				if x.p != nil {
					pk.Cost += lg.value(x.p, user, cf)
				} else {
					pk.Cost += lg.pickValue(x.a, user, cf)
				}
			}
			pk.Receive = []Asset{PlayerOf(target.ID)}
			if si == 0 {
				pk.Label = comboLabel(found)
			}
			pk.Note = lg.takeNote(found, needs.Status.Key, user, tid, target, allowCore)
			out = append(out, pk)
			if len(out) == AssistTuning.MaxPackages {
				break
			}
		}
		return out
	}

	res.Packages = build(false)
	if len(res.Packages) == 0 {
		res.Packages = build(true)
	}
	if len(res.Packages) > 0 {
		res.Possible = true
		return res, nil
	}
	switch {
	case un[target.ID] && (PlayerTier(target) == "Franchise" || lg.isTopValue(target, tid, cf)):
		res.Reason = "They won't move him: he's their franchise player."
	case un[target.ID] && diag.valueOK == 0:
		res.Reason = "They won't move him: he is one of their core players and nothing you have is worth enough to them."
	case diag.valueOK == 0:
		res.Reason = "You do not have enough to offer: even your best players and picks fall short of what he is worth to them."
	case diag.salaryFail > 0 && diag.otherFail == "":
		res.Reason = "Salary can't be matched under the cap rules. " + diag.salaryMsg
	case diag.otherFail != "":
		res.Reason = diag.otherFail
	default:
		res.Reason = "Nothing in your roster and picks makes a deal that works."
	}
	return res, nil
}

func (lg *League) isTopValue(p *Player, tid int, cf []float64) bool {
	v := lg.value(p, tid, cf)
	for _, q := range lg.Teams[tid].Roster {
		if q != p && lg.value(q, tid, cf) > v {
			return false
		}
	}
	return true
}

// takeDiag collects why candidates failed, to explain an impossible case.
type takeDiag struct {
	valueOK    int // combos that reached the value threshold
	salaryFail int
	salaryMsg  string
	otherFail  string
}

// takeCombos lists every set of up to MaxTradePlayers players and MaxPicks
// picks whose value to the other team reaches threshold (a necessary
// condition: the position penalty and the roster only make it harder).
func (lg *League) takeCombos(pool []*takeCand, threshold float64) []*takeCombo {
	var ps, ks []*takeCand
	for _, c := range pool {
		if c.p != nil {
			ps = append(ps, c)
		} else {
			ks = append(ks, c)
		}
	}
	var out []*takeCombo
	var pickSets [][]*takeCand
	pickSets = append(pickSets, nil)
	for i := range ks {
		pickSets = append(pickSets, []*takeCand{ks[i]})
		if AssistTuning.MaxPicks >= 2 {
			for j := i + 1; j < len(ks); j++ {
				pickSets = append(pickSets, []*takeCand{ks[i], ks[j]})
			}
		}
	}
	var playerSets [][]*takeCand
	playerSets = append(playerSets, nil)
	n := len(ps)
	for i := 0; i < n; i++ {
		playerSets = append(playerSets, []*takeCand{ps[i]})
		for j := i + 1; j < n; j++ {
			playerSets = append(playerSets, []*takeCand{ps[i], ps[j]})
			if MaxTradePlayers >= 3 {
				for k := j + 1; k < n; k++ {
					playerSets = append(playerSets, []*takeCand{ps[i], ps[j], ps[k]})
				}
			}
		}
	}
	for _, p := range playerSets {
		pv, pc := 0.0, 0.0
		for _, c := range p {
			pv += c.theirs
			pc += c.cost
		}
		for _, k := range pickSets {
			if len(p)+len(k) == 0 {
				continue
			}
			v, cost := pv, pc
			for _, c := range k {
				v += c.theirs
				cost += c.cost
			}
			if v < threshold {
				continue
			}
			cs := append(append([]*takeCand(nil), p...), k...)
			out = append(out, &takeCombo{cands: cs, players: len(p), picks: len(k), theirs: v,
				cost: cost + AssistTuning.PieceCost*float64(len(cs))})
		}
	}
	return out
}

// firstAccepted walks the combos in order and returns the first one that the
// real rules accept: salary rules for both teams, the value test, then the
// full deal evaluation.
func (lg *League) firstAccepted(list []*takeCombo, target *Player, tid, user int, cf []float64, diag *takeDiag) *takeCombo {
	tries := 0
	for _, c := range list {
		diag.valueOK++
		var out []*Player
		var pickIn float64
		for _, x := range c.cands {
			if x.p != nil {
				out = append(out, x.p)
			} else {
				pickIn += lg.pickValue(x.a, tid, cf)
			}
		}
		tgt := []*Player{target}
		if err := lg.salaryProblem(user, out, tgt); err != nil {
			diag.salaryFail++
			if diag.salaryMsg == "" {
				diag.salaryMsg = err.Error()
			}
			continue
		}
		if err := lg.salaryProblem(tid, tgt, out); err != nil {
			diag.salaryFail++
			if diag.salaryMsg == "" {
				diag.salaryMsg = err.Error()
			}
			continue
		}
		gives, gets, need := lg.sideEval(tid, tgt, out, cf)
		if gets+pickIn < gives*need {
			continue
		}
		tries++
		if tries > AssistTuning.MaxTries {
			return nil
		}
		d := TakePackage{Give: assetsOf(c), Receive: []Asset{PlayerOf(target.ID)}}.Deal(user, tid)
		v := evaluateDealBase(lg, d, user)
		if !v.Valid {
			if diag.otherFail == "" {
				diag.otherFail = v.Problem
			}
			continue
		}
		if v.Accept {
			return c
		}
	}
	return nil
}

func assetsOf(c *takeCombo) []Asset {
	var out []Asset
	for _, x := range c.cands {
		out = append(out, x.a)
	}
	return out
}

func comboKey(c *takeCombo) string {
	var ks []string
	for _, x := range c.cands {
		ks = append(ks, fmt.Sprintf("%d/%d/%d/%d/%d", x.a.Kind, x.a.PlayerID, x.a.Year, x.a.Round, x.a.Original))
	}
	sort.Strings(ks)
	return strings.Join(ks, ",")
}

func comboLabel(c *takeCombo) string {
	switch {
	case c.picks == 0:
		return "Players only"
	case c.players == 0:
		return "Picks only"
	}
	return "Players and picks"
}

func (lg *League) takeNote(c *takeCombo, key string, user, tid int, target *Player, core bool) string {
	var notes []string
	young, picks, vets, hasCore := false, false, false, false
	for _, x := range c.cands {
		switch {
		case x.p == nil:
			picks = true
		case x.young && x.p.Potential > x.p.Overall():
			young = true
		case x.vet:
			vets = true
		}
		hasCore = hasCore || x.core
	}
	rebuild := key == "rebuilding" || key == "seller"
	if young && rebuild {
		notes = append(notes, "They value young players with upside.")
	}
	if picks && rebuild {
		notes = append(notes, "They value draft picks.")
	}
	if vets && !rebuild {
		notes = append(notes, "They want win-now veterans.")
	}
	if hasCore {
		notes = append(notes, "Includes one of your core players.")
	}
	var out []*Player
	for _, x := range c.cands {
		if x.p != nil {
			out = append(out, x.p)
		}
	}
	if len(notes) < 2 && (lg.Payroll(user)-sumSalary(out)+target.Contract.counted() > Cap.SalaryCap || lg.Payroll(tid)-target.Contract.counted()+sumSalary(out) > Cap.SalaryCap) {
		notes = append(notes, "Salaries are matched under the cap rules.")
	}
	if len(notes) == 0 {
		notes = append(notes, "Fair value for both sides.")
	}
	if len(notes) > 2 {
		notes = notes[:2]
	}
	return strings.Join(notes, " ")
}

func sumSalary(ps []*Player) int {
	s := 0
	for _, p := range ps {
		s += p.Contract.counted()
	}
	return s
}

// ---- advisor ----

// AdvisorTuning holds the numbers behind Keep / Shop / Sell. Edit here. Points
// add up per player; at or above KeepAt is Keep, at or below SellAt is Sell,
// anything between is Shop.
var AdvisorTuning = struct {
	KeepAt, SellAt float64

	// Cornerstone: among the top CornerRank players by value, age at most
	// CornerMaxAge, and not asking out: always Keep.
	CornerRank, CornerMaxAge int

	// Role by rank on the roster (0 = best): points for rank under
	// StarterRank, under RotationRank, and for rank at or above DeepRank.
	StarterPts, RotationPts, DeepPts float64
	StarterRank, RotationRank        int
	DeepRank                         int

	// Direction. Rebuilding or selling: points per year of age over
	// OldAge (negative), and Young points for a young player with upside.
	// Contending or buying: Vet points for a productive veteran.
	OldAge      int
	OldPerYear  float64
	YoungAge    int
	YoungPts    float64
	VetPts      float64
	VetMinOvr   int
	ChipPts     float64 // a young player who will not help a contender now (negative)
	ChipMaxOvr  int
	ChipMaxAge  int
	BigSalary   int     // salary a veteran must be paid for "big contract" (extra penalty for a rebuilding team)
	BigSalaryPt float64 // negative

	// Contract against his market value: relative gap at which he counts as
	// overpaid / a bargain, points, and the most years that scale it.
	ValueGap     float64
	OverpaidPts  float64 // negative
	BargainPts   float64
	ContractMinY int // years left needed for a contract to matter

	// Expiring contract (last year): will test free agency, or open to it.
	WalkPts float64 // negative
	OpenPts float64 // negative

	// Mood and role.
	RequestPts float64 // negative
	UnhappyPts float64 // negative
	HappyPts   float64
	FitAPts    float64
	FitDPts    float64 // negative
	ShortPts   float64 // fewer minutes than his rank deserves (negative)
	NoMinPts   float64 // deep on the bench and not young (negative)
}{
	KeepAt: 0.5, SellAt: -1.0,
	CornerRank: 2, CornerMaxAge: 28,
	StarterPts: 1.2, RotationPts: 0.6, DeepPts: -0.4,
	StarterRank: 5, RotationRank: 8, DeepRank: 10,
	OldAge: 28, OldPerYear: -0.6, YoungAge: 24, YoungPts: 1.0, VetPts: 0.5, VetMinOvr: 60,
	ChipPts: -0.5, ChipMaxOvr: 58, ChipMaxAge: 22,
	BigSalary: 20000, BigSalaryPt: -0.8,
	ValueGap: 0.2, OverpaidPts: -1.0, BargainPts: 1.0, ContractMinY: 2,
	WalkPts: -1.5, OpenPts: -0.3,
	RequestPts: -3, UnhappyPts: -1, HappyPts: 0.3, FitAPts: 0.5, FitDPts: -0.7,
	ShortPts: -0.5, NoMinPts: -0.4,
}

// Advice values.
const (
	AdviceKeep = "keep"
	AdviceShop = "shop"
	AdviceSell = "sell"
)

// AdvisorRow is the advice for one player on the user's roster.
type AdvisorRow struct {
	Player  *Player
	Advice  string
	Reasons []string
	Score   float64
}

// AdvisorOf gives Keep / Shop / Sell for every player on the user's roster,
// best player first, and a one-sentence summary. Advice only.
func (lg *League) AdvisorOf() ([]AdvisorRow, string) {
	user := lg.UserTeamID
	tm := lg.Teams[user]
	t := &AdvisorTuning
	cf := lg.contendFactors()
	status := lg.TeamStatusOf(user)
	rebuild := status.Key == "rebuilding" || status.Key == "seller"
	pms := tm.EffectiveMinutes()
	byValue := append([]*Player(nil), tm.Roster...)
	sort.SliceStable(byValue, func(i, j int) bool {
		vi, vj := lg.value(byValue[i], user, cf), lg.value(byValue[j], user, cf)
		if vi != vj {
			return vi > vj
		}
		return byValue[i].ID < byValue[j].ID
	})
	valueRank := map[int]int{}
	for i, p := range byValue {
		valueRank[p.ID] = i
	}

	var rows []AdvisorRow
	for _, p := range tm.Roster {
		type part struct {
			pts    float64
			reason string
		}
		var parts []part
		add := func(pts float64, reason string) {
			if pts != 0 {
				parts = append(parts, part{pts, reason})
			}
		}
		rank := rankOnTeam(tm, p)
		ovr := p.Overall()
		switch {
		case rank < t.StarterRank:
			add(t.StarterPts, "One of your five best players")
		case rank < t.RotationRank:
			add(t.RotationPts, "Part of your rotation")
		case rank >= t.DeepRank:
			add(t.DeepPts, "Deep on the bench")
		}
		// direction
		if rebuild {
			if p.Age > t.OldAge {
				add(t.OldPerYear*float64(p.Age-t.OldAge), fmt.Sprintf("Age %d and your team is %s", p.Age, directionPhrase(status.Key)))
				if p.Contract.Salary >= t.BigSalary {
					add(t.BigSalaryPt, "Big contract that will not age well")
				}
			}
			if p.Age <= t.YoungAge && p.Potential > ovr+2 {
				add(t.YoungPts, "Young with room to grow")
			}
		} else {
			if p.Age > t.OldAge && ovr >= t.VetMinOvr {
				add(t.VetPts, "Productive veteran and your team is "+directionPhrase(status.Key))
			}
			if p.Age <= t.ChipMaxAge && ovr <= t.ChipMaxOvr && p.Potential > ovr+4 {
				add(t.ChipPts, "Prospect who will not help you now: a trade chip")
			}
		}
		// contract against market value
		if p.Contract.Salary > 0 && p.Contract.Years >= t.ContractMinY {
			market := float64(MarketSalary(p))
			pay := float64(p.Contract.Salary)
			rel := (market - pay) / max(market, pay)
			switch {
			case rel <= -t.ValueGap:
				add(t.OverpaidPts, fmt.Sprintf("Overpaid: %s against a market value of %s", fmtM(p.Contract.Salary), fmtM(int(market))))
			case rel >= t.ValueGap:
				add(t.BargainPts, fmt.Sprintf("Bargain: paid %s, worth about %s", fmtM(p.Contract.Salary), fmtM(int(market))))
			}
		}
		// expiring contract and his stance
		st := lg.StanceOf(p)
		if p.Contract.Years == 1 && p.Contract.NextYears == 0 {
			switch st.Key {
			case "test":
				add(t.WalkPts, "Contract ends soon and he will test free agency")
			case "open":
				add(t.OpenPts, "Contract ends soon and he is not sure to re-sign")
			}
		}
		// mood
		switch {
		case p.TradeRequest > 0:
			add(t.RequestPts, "Has asked to be traded")
		case p.Morale <= MoraleTuning.Unhappy:
			add(t.UnhappyPts, "Unhappy with the team")
		case p.Morale >= MoraleTuning.Happy:
			add(t.HappyPts, "Happy here")
		}
		// system fit (a balanced system fits everyone the same: no signal)
		if sys := tm.systemOf(); !sys.isBalanced() {
			switch lg.PlayerFit(tm, p).Grade() {
			case "A":
				add(t.FitAPts, "Fits your system well")
			case "D":
				add(t.FitDPts, "Poor fit with your system")
			}
		}
		// minutes against rank
		if _, _, short := lg.rolePts(tm, p, pms); short && !p.Injured() {
			add(t.ShortPts, "Playing fewer minutes than his rank deserves")
		}
		if rank >= t.DeepRank && p.Age > t.YoungAge {
			add(t.NoMinPts, "Out of the rotation")
		}

		score := 0.0
		for _, x := range parts {
			score += x.pts
		}
		advice := AdviceShop
		switch {
		case score >= t.KeepAt:
			advice = AdviceKeep
		case score <= t.SellAt:
			advice = AdviceSell
		}
		var corner string
		if valueRank[p.ID] < t.CornerRank && p.Age <= t.CornerMaxAge && p.TradeRequest == 0 {
			advice = AdviceKeep
			corner = "Franchise cornerstone: top value and young"
			parts = append(parts, part{100, corner})
		}
		if advice == AdviceShop && len(parts) == 0 {
			parts = append(parts, part{0, "Nothing strong either way; fine to move for the right offer"})
		}
		// reasons: the pulls that agree with the advice first, biggest first
		sort.SliceStable(parts, func(i, j int) bool {
			a, b := parts[i].pts, parts[j].pts
			switch advice {
			case AdviceSell:
				return a < b
			case AdviceKeep:
				return a > b
			}
			if a < 0 {
				a = -a
			}
			if b < 0 {
				b = -b
			}
			return a > b
		})
		var rs []string
		for _, x := range parts {
			if len(rs) < 2 {
				rs = append(rs, x.reason)
			}
		}
		if corner != "" && p.Contract.Years == 1 && st.Key == "test" && len(rs) < 3 {
			rs = append(rs, "Extend him soon: he may test free agency")
		}
		rows = append(rows, AdvisorRow{Player: p, Advice: advice, Reasons: rs, Score: score})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i].Player, rows[j].Player
		if a.Overall() != b.Overall() {
			return a.Overall() > b.Overall()
		}
		return a.ID < b.ID
	})

	var keep, shop, sell []string
	for _, r := range rows {
		switch r.Advice {
		case AdviceKeep:
			keep = append(keep, r.Player.Name)
		case AdviceShop:
			shop = append(shop, r.Player.Name)
		default:
			sell = append(sell, r.Player.Name)
		}
	}
	first := func(xs []string, n int) []string { return xs[:min(n, len(xs))] }
	s := fmt.Sprintf("Your team is %s: %d to keep, %d to shop, %d to sell.", strings.ToLower(status.Label), len(keep), len(shop), len(sell))
	if len(sell) > 0 {
		s += " Best to sell: " + joinList(first(sell, 3)) + "."
	}
	if len(keep) > 0 {
		s += " Build around: " + joinList(first(keep, 2)) + "."
	}
	return rows, s
}
