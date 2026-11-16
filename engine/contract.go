package engine

import (
	"fmt"
	"math"
	"sort"
)

// Cap holds every contract and salary-cap setting in one place. Money is in
// thousands of dollars per season.
var Cap = struct {
	SalaryCap       int     // soft cap: cap-room signings and trades must fit under it
	TaxLine         int     // payroll above this pays luxury tax
	HardCeiling     int     // payroll may never go above this (except draft picks)
	MinSalary       int     // smallest contract
	MaxSalary       int     // largest contract (35% of the cap)
	TaxRate         float64 // tax dollars per dollar above the tax line
	TradeMatchPct   int     // over the cap, incoming salary may be this % of outgoing...
	TradeMatchExtra int     // ...plus this much
	RookieTop       int     // salary of the first pick
	RookieLastFirst int     // salary of the last first-round pick (slides linearly)
	// second-round picks get MinSalary

	RookieYearsRound1 int
	RookieYearsRound2 int
	MaxContractYears  int

	// Market salary curve (visible overall and age only):
	// salary = Min + (Max-Min) * x^MarketCurve, x = (effective ovr - MarketFloorOvr)
	// / (MarketTopOvr - MarketFloorOvr), clamped to 0..1.
	MarketFloorOvr float64
	MarketTopOvr   float64
	MarketCurve    float64
	// Young players with potential at least MarketYoungGap above their overall
	// are paid as if rated up to MarketYoungMax higher (a quarter of the gap).
	MarketYoungAge int
	MarketYoungGap int
	MarketYoungMax float64
	// From age MarketOldAge on, salary drops by MarketOldStep per year past 30 (floor MarketOldMin).
	MarketOldAge  int
	MarketOldStep float64
	MarketOldMin  float64

	// Contract effect on trade value (see contractAdjust): the value changes by
	// up to +-ContractSwing (fraction) for a bargain or overpay lasting
	// ContractFullYears or more; shorter deals count proportionally.
	ContractSwing     float64
	ContractFullYears int
	// An expiring player (no years left) is worth this much less to a fully
	// rebuilding team (scaled down to nothing for a contender).
	ExpiringRebuildCut float64

	// Computer teams re-sign an expiring player only if his trade value to
	// them is at least this (about a 62-rated player).
	ReSignMinValue float64

	// Extensions (see ExtensionAsk): in his last contract year a player may
	// extend. The ask is his market salary (resign stance), times
	// ExtensionOpenMult for an "open" stance; it may be at most
	// ExtensionMaxRaise times his current salary (the NBA 140% rule).
	ExtensionMaxRaise float64
	ExtensionOpenMult float64

	// Mid-level exception: once per offseason a team may sign one free agent
	// without cap room. Up to MidLevel for MidLevelYears if the payroll stays
	// at or under the tax line; otherwise the smaller taxpayer version
	// (TaxMidLevel, TaxMidLevelYears), which still obeys the hard ceiling.
	MidLevel         int
	MidLevelYears    int
	TaxMidLevel      int
	TaxMidLevelYears int
}{
	SalaryCap: 140000, TaxLine: 170000, HardCeiling: 190000,
	MinSalary: 1200, MaxSalary: 49000,
	TaxRate: 1.5, TradeMatchPct: 125, TradeMatchExtra: 250,
	RookieTop: 10000, RookieLastFirst: 2200,
	RookieYearsRound1: 4, RookieYearsRound2: 2, MaxContractYears: 4,

	MarketFloorOvr: 47, MarketTopOvr: 92, MarketCurve: 1.8,
	MarketYoungAge: 24, MarketYoungGap: 5, MarketYoungMax: 4,
	MarketOldAge: 31, MarketOldStep: 0.05, MarketOldMin: 0.6,

	ContractSwing: 0.30, ContractFullYears: 4, ExpiringRebuildCut: 0.08,
	ReSignMinValue:    4.5,
	ExtensionMaxRaise: 1.40, ExtensionOpenMult: 1.10,
	MidLevel: 12800, MidLevelYears: 4, TaxMidLevel: 5200, TaxMidLevelYears: 2,
}

// contractOffset is the RNG stream for contract randomness (initial contract
// lengths). 20000 is proposals, 30000 injuries; this one is separate so game
// results for a seed never change.
const contractOffset = 40000

// Contract is a player's deal. Salary is thousands per season; Years is the
// seasons left, including the current one (0 = none, or expiring in the
// offseason).
type Contract struct {
	Salary int
	Years  int
	// NextSalary and NextYears are an extension signed in the last contract
	// year (see Extend). Zero NextYears = none. BeginOffseason makes them the
	// contract when the current one runs out.
	NextSalary int `json:",omitempty"`
	NextYears  int `json:",omitempty"`
}

// counted is what the contract adds to a payroll: nothing once it has run out.
func (c Contract) counted() int {
	if c.Years < 1 {
		return 0
	}
	return c.Salary
}

// MarketSalary is what a player is worth on the open market, from visible
// overall and age only, rounded to the nearest 100.
func MarketSalary(p *Player) int {
	c := &Cap
	eff := float64(p.Overall())
	if gap := p.Potential - p.Overall(); p.Age <= c.MarketYoungAge && gap >= c.MarketYoungGap {
		eff += math.Min(c.MarketYoungMax, float64(gap)/4)
	}
	x := math.Max(0, math.Min(1, (eff-c.MarketFloorOvr)/(c.MarketTopOvr-c.MarketFloorOvr)))
	sal := float64(c.MinSalary) + float64(c.MaxSalary-c.MinSalary)*math.Pow(x, c.MarketCurve)
	if p.Age >= c.MarketOldAge {
		sal *= math.Max(c.MarketOldMin, 1-c.MarketOldStep*float64(p.Age-30))
	}
	s := int(math.Round(sal/100)) * 100
	return max(c.MinSalary, min(c.MaxSalary, s))
}

// contractAdjust is the trade-value multiplier from a player's contract: a
// bargain (market salary above what he is paid) raises it, an overpay lowers
// it, by at most +-ContractSwing, scaled by the years left. A player with no
// years left (expiring) has no surplus and is worth a little less to a
// rebuilding team. Players with no contract at all are unaffected.
func contractAdjust(p *Player, contend float64) float64 {
	c := &Cap
	if p.Contract.Salary == 0 {
		return 1
	}
	if p.Contract.Years < 1 {
		return 1 - c.ExpiringRebuildCut*(1-contend)
	}
	market, pay := float64(MarketSalary(p)), float64(p.Contract.Salary)
	rel := (market - pay) / math.Max(market, pay)
	yw := math.Min(1, float64(p.Contract.Years)/float64(c.ContractFullYears))
	return 1 + c.ContractSwing*rel*yw
}

// RookieContract is the contract for a draft pick by overall pick number
// (1-based) and round.
func RookieContract(pick, round int) Contract {
	c := &Cap
	if round >= 2 {
		return Contract{Salary: c.MinSalary, Years: c.RookieYearsRound2}
	}
	f := float64(pick-1) / 29
	if f > 1 {
		f = 1
	}
	s := float64(c.RookieTop) + f*float64(c.RookieLastFirst-c.RookieTop)
	return Contract{Salary: int(math.Round(s/100)) * 100, Years: c.RookieYearsRound1}
}

// Payroll is the total of a team's contracts that are still running.
func (lg *League) Payroll(teamID int) int {
	n := 0
	for _, p := range lg.Teams[teamID].Roster {
		n += p.Contract.counted()
	}
	return n
}

// CapRoom is the salary cap minus the payroll (negative when over the cap).
func (lg *League) CapRoom(teamID int) int { return Cap.SalaryCap - lg.Payroll(teamID) }

// TaxBill is the luxury tax a team owes at its current payroll.
func (lg *League) TaxBill(teamID int) int {
	over := lg.Payroll(teamID) - Cap.TaxLine
	if over <= 0 {
		return 0
	}
	return int(float64(over) * Cap.TaxRate)
}

// SignKind says which rule a signing falls under.
type SignKind int

const (
	SignCapRoom   SignKind = iota // needs cap room
	SignOwn                       // re-signing one's own expiring player
	SignMinimum                   // minimum contract
	SignDraft                     // a drafted rookie
	SignException                 // the once-a-year mid-level exception (see ExceptionFor)
)

// fmtM writes thousands of dollars as millions, e.g. 18200 -> "$18.2M".
func fmtM(k int) string { return fmt.Sprintf("$%.1fM", float64(k)/1000) }

// CanSign says whether the team may sign a contract of this salary under the
// kind's rule (nil if yes). Years are checked in Sign.
func (lg *League) CanSign(teamID int, salary int, kind SignKind) error {
	c := &Cap
	if teamID < 0 || teamID >= len(lg.Teams) {
		return fmt.Errorf("That is not a valid team.")
	}
	if salary < c.MinSalary || salary > c.MaxSalary {
		return fmt.Errorf("A contract must be between %s and %s.", fmtM(c.MinSalary), fmtM(c.MaxSalary))
	}
	name := lg.Teams[teamID].FullName()
	after := lg.Payroll(teamID) + salary
	switch kind {
	case SignCapRoom:
		if after > c.SalaryCap {
			return fmt.Errorf("%s have %s of cap room; this contract is %s.", name, fmtM(max(0, lg.CapRoom(teamID))), fmtM(salary))
		}
	case SignMinimum:
		if salary != c.MinSalary {
			return fmt.Errorf("A minimum contract must be exactly %s.", fmtM(c.MinSalary))
		}
	case SignOwn:
	case SignException:
		if err := lg.exceptionProblem(teamID, salary, 0); err != nil {
			return err
		}
	case SignDraft:
		// Draft picks always sign, even past the hard ceiling (the rookie scale
		// is small and the roster is trimmed afterwards), so no ceiling check.
		return nil
	default:
		return fmt.Errorf("Unknown signing type.")
	}
	if after > c.HardCeiling {
		return fmt.Errorf("%s would be at %s, over the hard ceiling of %s.", name, fmtM(after), fmtM(c.HardCeiling))
	}
	return nil
}

// Sign gives the player a contract with the team and puts him on its roster
// (taking him out of free agency) if he is not already there.
func (lg *League) Sign(teamID int, p *Player, salary, years int, kind SignKind) error {
	if teamID < 0 || teamID >= len(lg.Teams) {
		return fmt.Errorf("That is not a valid team.")
	}
	if years < 1 || years > Cap.MaxContractYears {
		return fmt.Errorf("A contract must run 1 to %d years.", Cap.MaxContractYears)
	}
	t := lg.Teams[teamID]
	onTeam := lg.findRosterPlayer(teamID, p.ID) != nil
	switch {
	case kind == SignOwn && !onTeam && p.TeamID < 0 && lg.FA != nil && lg.FA.Rights[p.ID] == teamID:
		// a restricted free agent signing with the team that holds his rights
	case kind == SignOwn:
		if !onTeam || p.Contract.Years > 0 {
			return fmt.Errorf("%s is not an expiring player of %s.", p.Name, t.FullName())
		}
	case kind == SignDraft:
	case p.TeamID >= 0:
		return fmt.Errorf("%s is already under contract with a team.", p.Name)
	}
	if !onTeam && len(t.Roster) >= RosterSize {
		return fmt.Errorf("%s already have a full roster of %d.", t.FullName(), RosterSize)
	}
	if err := lg.CanSign(teamID, salary, kind); err != nil {
		return err
	}
	if kind == SignException {
		if err := lg.exceptionProblem(teamID, salary, years); err != nil {
			return err
		}
		t.ExceptionYear = lg.Year
	}
	lg.forceSign(teamID, p, salary, years)
	return nil
}

// ExceptionInfo describes the mid-level exception a team could use right now.
// Kind is "mid", "taxpayer" or "none" (Reason says why). MaxSalary and
// MaxYears are for a signing at the largest allowed amount.
type ExceptionInfo struct {
	Kind      string
	Label     string
	MaxSalary int
	MaxYears  int
	Used      bool
	Reason    string
}

// exceptionUsed is true once the team has used its exception this offseason.
func (lg *League) exceptionUsed(teamID int) bool {
	return lg.Teams[teamID].ExceptionYear == lg.Year
}

// exceptionProblem checks a signing under the exception (years 0 = skip the
// years check). It does not check the hard ceiling (CanSign does).
func (lg *League) exceptionProblem(teamID, salary, years int) error {
	c := &Cap
	name := lg.Teams[teamID].FullName()
	if lg.exceptionUsed(teamID) {
		return fmt.Errorf("%s have already used their exception this offseason.", name)
	}
	if lg.Payroll(teamID)+salary <= c.TaxLine {
		if salary > c.MidLevel {
			return fmt.Errorf("The mid-level exception is at most %s.", fmtM(c.MidLevel))
		}
		if years > c.MidLevelYears {
			return fmt.Errorf("The mid-level exception allows at most %d years.", c.MidLevelYears)
		}
		return nil
	}
	if salary > c.TaxMidLevel {
		return fmt.Errorf("Over the tax line only the taxpayer exception works: at most %s for %d years.", fmtM(c.TaxMidLevel), c.TaxMidLevelYears)
	}
	if years > c.TaxMidLevelYears {
		return fmt.Errorf("The taxpayer exception allows at most %d years.", c.TaxMidLevelYears)
	}
	return nil
}

// ExceptionFor says which exception, if any, the team can use now. It is
// "none" when it was used this offseason, when cap room is at least as big as
// the exception (use the room), or when even the taxpayer amount would break
// the hard ceiling.
func (lg *League) ExceptionFor(teamID int) ExceptionInfo {
	c := &Cap
	if teamID < 0 || teamID >= len(lg.Teams) {
		return ExceptionInfo{Kind: "none", Label: "No exception", Reason: "That is not a valid team."}
	}
	pay := lg.Payroll(teamID)
	none := func(reason string, used bool) ExceptionInfo {
		return ExceptionInfo{Kind: "none", Label: "No exception", Used: used, Reason: reason}
	}
	if lg.exceptionUsed(teamID) {
		return none("Already used this offseason.", true)
	}
	var info ExceptionInfo
	switch {
	case pay+c.MidLevel <= c.TaxLine:
		info = ExceptionInfo{Kind: "mid", Label: "Mid-level exception", MaxSalary: c.MidLevel, MaxYears: c.MidLevelYears}
	case pay+c.TaxMidLevel <= c.HardCeiling:
		info = ExceptionInfo{Kind: "taxpayer", Label: "Taxpayer mid-level exception", MaxSalary: c.TaxMidLevel, MaxYears: c.TaxMidLevelYears}
	default:
		return none("Payroll is too close to the hard ceiling for any exception.", false)
	}
	if lg.CapRoom(teamID) >= info.MaxSalary {
		return none("Cap room is at least as big as the exception; sign with cap room.", false)
	}
	return info
}

// forceSign does the signing without any rule check.
func (lg *League) forceSign(teamID int, p *Player, salary, years int) {
	for i, q := range lg.FreeAgents {
		if q == p {
			lg.FreeAgents = append(lg.FreeAgents[:i], lg.FreeAgents[i+1:]...)
			break
		}
	}
	p.Contract = Contract{Salary: salary, Years: years}
	if lg.findRosterPlayer(teamID, p.ID) != nil {
		lg.moraleReSigned(p) // re-signed with his own team
	}
	if lg.findRosterPlayer(teamID, p.ID) == nil {
		lg.moraleNewTeam(p, -1, false)
		p.TeamID = teamID
		p.TeamSince = lg.Year
		lg.Teams[teamID].Roster = append(lg.Teams[teamID].Roster, p)
	}
}

// Expiring lists the team's players whose contracts ran out this offseason.
func (lg *League) Expiring(teamID int) []*Player {
	var out []*Player
	for _, p := range lg.Teams[teamID].Roster {
		if p.Contract.Years < 1 {
			out = append(out, p)
		}
	}
	return out
}

// AskingContract is what a player wants to re-sign: his market salary now
// (after this offseason's progression), for longer if he is young.
func AskingContract(p *Player) (salary, years int) {
	switch {
	case p.Age <= 25:
		years = 4
	case p.Age <= 28:
		years = 3
	case p.Age <= 31:
		years = 2
	default:
		years = 1
	}
	return MarketSalary(p), min(years, Cap.MaxContractYears)
}

// CanReSign says why the team cannot re-sign the expiring player now (nil if
// it can, as far as his stance goes).
func (lg *League) CanReSign(p *Player) error {
	if lg.StanceOf(p).Key == "test" {
		return errTestsFA
	}
	return nil
}

// ReSign re-signs one of the team's expiring players at his ask.
func (lg *League) ReSign(teamID, playerID int) error {
	if lg.FA != nil {
		return fmt.Errorf("Free agency has begun, so re-signing is closed. Bid for him as a free agent.")
	}
	if teamID < 0 || teamID >= len(lg.Teams) {
		return fmt.Errorf("That is not a valid team.")
	}
	p := lg.findRosterPlayer(teamID, playerID)
	if p == nil || p.Contract.Years > 0 {
		return fmt.Errorf("That player is not an expiring player on your team.")
	}
	if lg.StanceOf(p).Key == "test" {
		return errTestsFA
	}
	salary, years := AskingContract(p)
	return lg.Sign(teamID, p, salary, years, SignOwn)
}

// inOffseason is true from the end of a season until the next one starts.
func (lg *League) inOffseason() bool {
	s := lg.Season
	return lg.Draft != nil || s == nil || s.Year < lg.Year
}

// Release sends a player to free agency and ends his contract. Simplification:
// there is no dead money, the whole contract just disappears. Only allowed in
// the offseason.
func (lg *League) Release(teamID, playerID int) error {
	if teamID < 0 || teamID >= len(lg.Teams) {
		return fmt.Errorf("That is not a valid team.")
	}
	if !lg.inOffseason() {
		return fmt.Errorf("Players can only be released in the offseason.")
	}
	t := lg.Teams[teamID]
	for i, p := range t.Roster {
		if p.ID == playerID {
			t.Roster = append(t.Roster[:i], t.Roster[i+1:]...)
			lg.releaseToFA(p)
			return nil
		}
	}
	return fmt.Errorf("That player is not on the team.")
}

// releaseToFA moves a player (already off his roster) to free agency, contract ended.
func (lg *League) releaseToFA(p *Player) {
	lg.moraleNewTeam(p, -1, false)
	p.TeamID = -1
	p.Contract = Contract{}
	lg.FreeAgents = append(lg.FreeAgents, p)
}

// salaryProblem checks the trade salary rules for one team that sends out and
// takes in the given players: legal if after the deal it is at or under the
// cap, or it takes back at most TradeMatchPct of what it sends plus
// TradeMatchExtra; and in every case it stays at or under the hard ceiling.
func (lg *League) salaryProblem(tid int, out, in []*Player) error {
	var o, i int
	for _, p := range out {
		o += p.Contract.counted()
	}
	for _, p := range in {
		i += p.Contract.counted()
	}
	name := lg.Teams[tid].FullName()
	after := lg.Payroll(tid) - o + i
	if after > Cap.HardCeiling {
		return fmt.Errorf("%s would be at %s after the deal, over the hard ceiling of %s.", name, fmtM(after), fmtM(Cap.HardCeiling))
	}
	if after <= Cap.SalaryCap {
		return nil
	}
	limit := o*Cap.TradeMatchPct/100 + Cap.TradeMatchExtra
	if i > limit {
		return fmt.Errorf("%s would take back %s for %s; over the cap they can take back at most %s.", name, fmtM(i), fmtM(o), fmtM(limit))
	}
	return nil
}

// assignInitialContracts gives every rostered player a market-salary contract
// of 1 to 4 years (lengths from the contract RNG stream), then scales down
// any team above the tax line so no league starts illegal.
func (lg *League) assignInitialContracts() {
	r := lg.dayRNG(lg.Year, contractOffset)
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			w := []float64{1, 1, 1, 1}
			if p.Age <= 26 {
				w[3] += 1.5
				w[2] += 0.5
			}
			if p.Age >= 31 {
				w[0] += 2
				w[1] += 1
				w[3] -= 0.5
			}
			if p.Overall() >= 66 {
				w[3] += 1
				w[2] += 0.5
			}
			if p.Overall() < 58 {
				w[0] += 1
			}
			p.Contract = Contract{Salary: MarketSalary(p), Years: 1 + r.PickWeighted(w)}
		}
		lg.scaleToTax(t)
	}
}

// scaleToTax shrinks the part of every salary above the minimum by one
// factor, rounding down to 100, until the team is at or under the tax line.
func (lg *League) scaleToTax(t *Team) {
	pay := lg.Payroll(t.ID)
	if pay <= Cap.TaxLine {
		return
	}
	floor := 0
	for _, p := range t.Roster {
		if p.Contract.counted() > 0 {
			floor += Cap.MinSalary
		}
	}
	f := float64(Cap.TaxLine-floor) / float64(pay-floor)
	for _, p := range t.Roster {
		if s := p.Contract.counted(); s > 0 {
			ns := Cap.MinSalary + int(float64(s-Cap.MinSalary)*f)/100*100
			p.Contract.Salary = max(Cap.MinSalary, min(s, ns))
		}
	}
}

// reSignComputerTeams lets every computer team keep the expiring players worth
// their ask, best first, while staying at or under the tax line. (No roster limit
// here: the trim to 13 afterwards drops the weakest, drafted rookies included.) Deterministic. Players it does not keep stay expiring.
func (lg *League) reSignComputerTeams() {
	cf := lg.contendFactors()
	for _, t := range lg.Teams {
		if t.ID != lg.UserTeamID {
			lg.reSignTeam(t, cf)
		}
	}
}

// reSignTeam is the computer re-sign decision for one team.
func (lg *League) reSignTeam(t *Team, cf []float64) {
	exp := lg.Expiring(t.ID)
	vals := map[int]float64{}
	for _, p := range exp {
		vals[p.ID] = lg.value(p, t.ID, cf)
	}
	sort.SliceStable(exp, func(i, j int) bool {
		if vals[exp[i].ID] != vals[exp[j].ID] {
			return vals[exp[i].ID] > vals[exp[j].ID]
		}
		return exp[i].ID < exp[j].ID
	})
	for _, p := range exp {
		if vals[p.ID] < Cap.ReSignMinValue || lg.StanceOf(p).Key == "test" {
			continue
		}
		salary, years := AskingContract(p)
		if lg.Payroll(t.ID)+salary > Cap.TaxLine {
			continue
		}
		lg.Sign(t.ID, p, salary, years, SignOwn)
	}
}

// releaseExpiring sends every player whose contract is still expired to free agency.
func (lg *League) releaseExpiring() {
	for _, t := range lg.Teams {
		var keep []*Player
		for _, p := range t.Roster {
			if p.Contract.Years < 1 {
				lg.releaseToFA(p)
			} else {
				keep = append(keep, p)
			}
		}
		t.Roster = keep
	}
}

// signFromPool signs the best free agent for the team: at his ask if the team
// has the cap room, else a one-year minimum. A minimum deal that would pass
// the hard ceiling is still allowed so a roster can always be filled.
func (lg *League) signFromPool(tid int, p *Player) {
	salary, years := AskingContract(p)
	if lg.Sign(tid, p, salary, years, SignCapRoom) == nil {
		return
	}
	if lg.Sign(tid, p, Cap.MinSalary, 1, SignMinimum) == nil {
		return
	}
	lg.forceSign(tid, p, Cap.MinSalary, 1)
}

// recordTax stores each team's luxury tax for the season just finished.
func (lg *League) recordTax() {
	for _, t := range lg.Teams {
		if t.TaxPaid == nil {
			t.TaxPaid = map[int]int{}
		}
		t.TaxPaid[lg.Year] = lg.TaxBill(t.ID)
	}
}
