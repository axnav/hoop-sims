package engine

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
)

// The assistant GM: rules (no AI, no randomness) that look at the user's team
// and list suggestions with reasons and a "Do it" action, and, when the owner
// switches an area to auto, do the routine ones themselves and log what they
// did. Areas: lineup, training, contracts, free agency, draft and trades.
// TRADES ARE NEVER DONE AUTOMATICALLY: a trade suggestion is only executed by
// GMApply, which the owner's click calls; the trade switch only alerts.
//
// Everything is built from existing engine functions and from information the
// owner can see. Player.Impact, TrueOverall and a prospect's true ratings are
// never read: draft suggestions use the scouting ranges (ScoutView) only.
//
// Saved state is League.GM (a pointer so that a league that never used the
// assistant saves exactly the same bytes as before). The zero state is
// "everything off", which changes nothing in the game, including random
// streams: no code here draws a random number.

// GMTuning holds the numbers behind the suggestions. Edit here.
var GMTuning = struct {
	// Most suggestions listed per area.
	MaxTraining, MaxContracts, MaxFA, MaxTrades int
	// Trades: most "sell" suggestions among MaxTrades, players of the advisor
	// tried, players tried for an upgrade.
	MaxSells, SellCandidates, UpgradeCandidates int
	// A sell offer must bring back at least this share of the player's value.
	SellMinReturn float64
	// An upgrade must lift the mean overall of the top eight by this much.
	UpgradeMinGain float64

	// Lineup: a healthy player outside the custom starting five who is at
	// least this many overall points better than a starter who would be
	// dropped makes the reset worth suggesting.
	LineupGap int

	// Contracts: the re-sign and extension rule is the computer teams' one
	// (trade value at least Cap.ReSignMinValue, payroll at or under the tax
	// line), tilted by the plan. Win now: veterans of WinNowVetAge or older
	// count WinNowVetBoost times their value and the tax line is lifted by
	// WinNowTaxRoom. Rebuild: players older than RebuildKeepAge are kept only
	// when worth RebuildStarMult times the usual minimum.
	WinNowVetAge    int
	WinNowVetBoost  float64
	WinNowTaxRoom   int
	RebuildKeepAge  int
	RebuildStarMult float64
	ReleaseMaxYears int // release: most years left on his contract
	ReleaseMaxDead  int // release: most total dead money (salary x years, thousands)
	ReleaseMinRank  int // release: his rank on the roster by overall (0 = best) must be at least this

	// Stars and the tax line. A player among the StarRank best of the roster
	// (by overall, 1 = best), aged StarMaxAge or younger, may be re-signed or
	// extended up to the tax line plus StarTaxRoom (a star walking for nothing
	// costs more than a little tax). Anyone else who would take the payroll
	// within NearTaxRoom of the tax line must be worth NearTaxNeedMult times
	// the usual minimum: near the line money goes to value per dollar.
	StarRank        int
	StarMaxAge      int
	StarTaxRoom     int
	NearTaxRoom     int
	NearTaxNeedMult float64
	// An extension for a player of OldYearsAge or older runs at most
	// OldMaxYears (older players decline; a long deal blocks the roster).
	OldYearsAge, OldMaxYears int

	// Free agency: a target is worth at least FAMinValue (trade value to the
	// user); each need he fills adds LackBonus (at most two count); a skill
	// need is filled by a rating of SkillRating or a badge. Rebuild only
	// bids for players of RebuildFAMaxAge or younger.
	FAMinValue      float64
	LackBonus       float64
	SkillRating     int
	RebuildFAMaxAge int

	// Draft: weight of the overall midpoint (the rest goes to potential) by plan.
	DraftWinNowOvr, DraftRebuildOvr float64

	// Upgrade trades ("trade:get"). Only for plan win_now, or balanced when the
	// team is a contender or a buyer; never for rebuild nor for a seller or
	// rebuilding team. The package may hold at most UpgradeMaxCore of the
	// user's five best players by value and at most UpgradeMaxFirsts first-round
	// picks (UpgradeWinNowMaxFirsts for win now); the target is at most
	// UpgradeMaxAge years old (UpgradeWinNowMaxAge for win now).
	UpgradeMaxCore         int
	UpgradeMaxFirsts       int
	UpgradeWinNowMaxFirsts int
	UpgradeMaxAge          int
	UpgradeWinNowMaxAge    int

	// Trade alerts: look every this many days.
	AlertEveryDays int

	// Log: entries kept.
	LogKeep int
}{
	MaxTraining: 5, MaxContracts: 5, MaxFA: 3, MaxTrades: 3,
	MaxSells: 2, SellCandidates: 4, UpgradeCandidates: 6,
	SellMinReturn: 0.75, UpgradeMinGain: 0.5,
	LineupGap:    3,
	WinNowVetAge: 28, WinNowVetBoost: 1.15, WinNowTaxRoom: 10000,
	RebuildKeepAge: 25, RebuildStarMult: 2,
	ReleaseMaxYears: 2, ReleaseMaxDead: 6000, ReleaseMinRank: 10,
	StarRank: 3, StarMaxAge: 30, StarTaxRoom: 15000, NearTaxRoom: 15000, NearTaxNeedMult: 1.4,
	OldYearsAge: 31, OldMaxYears: 2,
	UpgradeMaxCore: 2, UpgradeMaxFirsts: 1, UpgradeWinNowMaxFirsts: 2, UpgradeMaxAge: 30, UpgradeWinNowMaxAge: 34,
	FAMinValue: 3, LackBonus: 1, SkillRating: 70, RebuildFAMaxAge: 25,
	DraftWinNowOvr: 0.7, DraftRebuildOvr: 0.2,
	AlertEveryDays: 3,
	LogKeep:        200,
}

// GM plans (the keys go out in the API).
const (
	GMPlanBalanced = "balanced"
	GMPlanWinNow   = "win_now"
	GMPlanRebuild  = "rebuild"
)

// GM areas (the keys go out in the API).
const (
	GMAreaLineup     = "lineup"
	GMAreaTraining   = "training"
	GMAreaContracts  = "contracts"
	GMAreaFreeAgency = "free_agency"
	GMAreaDraft      = "draft"
	GMAreaTrades     = "trades"
)

// GMAuto says which areas the assistant runs by itself. TradeAlerts only
// tells the owner about trade ideas while simming; trades are never made
// without a click.
type GMAuto struct {
	Lineup, Training, Contracts, FreeAgency, Draft, TradeAlerts bool
}

// GMLogEntry is one thing the assistant did (Auto) or the owner had it do.
type GMLogEntry struct {
	Year, Day int // Day is the engine's 0-based day (the API shows it 1-based)
	Area      string
	Text      string
	Auto      bool
}

// GMState is the saved assistant state. The zero value is everything off.
type GMState struct {
	Plan string `json:",omitempty"` // "" = balanced, "win_now", "rebuild"
	Auto GMAuto
	Log  []GMLogEntry `json:",omitempty"` // oldest first, newest GMTuning.LogKeep kept
	// Seen holds the trade ideas already alerted this season ("year|id").
	Seen map[string]bool `json:",omitempty"`
}

func (g *GMState) isZero() bool {
	return g.Plan == "" && g.Auto == (GMAuto{}) && len(g.Log) == 0 && len(g.Seen) == 0
}

// GMSuggestion is one thing the assistant proposes.
type GMSuggestion struct {
	ID       string
	Area     string
	Title    string
	Reasons  []string
	Action   string
	PlayerID int    // -1 when none (player 0 is a real player)
	Link     string // optional front-end route to look first
}

// ErrGMBadID is wrapped by GMApply for an id that is not a suggestion id at all.
var ErrGMBadID = errors.New("That is not a valid suggestion.")

// GMPlans lists the plans as key, label, blurb.
func GMPlans() [][3]string {
	return [][3]string{
		{GMPlanBalanced, "Balanced", "Keep the team competitive and the books healthy."},
		{GMPlanWinNow, "Win now", "Favour veterans and the best players available today."},
		{GMPlanRebuild, "Rebuild", "Favour young players with potential and sell older ones."},
	}
}

// GMAreas lists the areas as key, label, blurb.
func GMAreas() [][3]string {
	return [][3]string{
		{GMAreaLineup, "Lineup", "Keeps your starting five and minutes sensible."},
		{GMAreaTraining, "Training", "Gives each player the training focus with the most room to grow."},
		{GMAreaContracts, "Contracts", "Re-signs and extends players who are worth their ask."},
		{GMAreaFreeAgency, "Free agency", "Bids for free agents that fit your needs and your money."},
		{GMAreaDraft, "Draft", "Picks the best prospect by your own scouting."},
		{GMAreaTrades, "Trades", "Trade ideas are only ever suggestions: nothing is traded without your click."},
	}
}

// ValidGMPlan says whether key is a plan ("" counts as balanced).
func ValidGMPlan(key string) bool {
	switch key {
	case "", GMPlanBalanced, GMPlanWinNow, GMPlanRebuild:
		return true
	}
	return false
}

// GMPlan is the current plan key (never empty).
func (lg *League) GMPlan() string {
	if lg.GM == nil || lg.GM.Plan == "" {
		return GMPlanBalanced
	}
	return lg.GM.Plan
}

// GMAutoSettings is the current auto switches.
func (lg *League) GMAutoSettings() GMAuto {
	if lg.GM == nil {
		return GMAuto{}
	}
	return lg.GM.Auto
}

// GMLogEntries is the log, oldest first (a copy).
func (lg *League) GMLogEntries() []GMLogEntry {
	if lg.GM == nil {
		return nil
	}
	return append([]GMLogEntry(nil), lg.GM.Log...)
}

// SetGMPlan sets the plan. Balanced is stored as "".
func (lg *League) SetGMPlan(plan string) error {
	if !ValidGMPlan(plan) {
		return fmt.Errorf("Unknown plan %q.", plan)
	}
	if plan == GMPlanBalanced {
		plan = ""
	}
	g := lg.gmState()
	g.Plan = plan
	lg.gmTidy()
	return nil
}

// SetGMAuto sets the auto switches.
func (lg *League) SetGMAuto(a GMAuto) {
	g := lg.gmState()
	g.Auto = a
	lg.gmTidy()
}

// gmState is the state to write to (created on first use).
func (lg *League) gmState() *GMState {
	if lg.GM == nil {
		lg.GM = &GMState{}
	}
	return lg.GM
}

// gmTidy drops an all-zero state so the league saves as if it never used it.
func (lg *League) gmTidy() {
	if lg.GM != nil && lg.GM.isZero() {
		lg.GM = nil
	}
}

func (lg *League) gmLog(area, text string, auto bool) {
	g := lg.gmState()
	g.Log = append(g.Log, GMLogEntry{Year: lg.Year, Day: lg.currentDay(), Area: area, Text: text, Auto: auto})
	if n := len(g.Log); n > GMTuning.LogKeep {
		g.Log = append([]GMLogEntry(nil), g.Log[n-GMTuning.LogKeep:]...)
	}
}

// gmCand is a suggestion with the call that carries it out.
type gmCand struct {
	s  GMSuggestion
	do func() (string, error)
	// manual marks a step auto mode never takes by itself (owner 2026-11-17:
	// a release always needs the owner's click).
	manual bool
}

func yearsText(n int) string {
	if n == 1 {
		return "1 year"
	}
	return fmt.Sprintf("%d years", n)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// ---- entry points ----

// GMSuggestions lists every suggestion right now, area by area. Each one can
// be carried out immediately with GMApply.
func (lg *League) GMSuggestions() []GMSuggestion {
	var out []GMSuggestion
	for _, a := range GMAreas() {
		for _, c := range lg.gmArea(a[0]) {
			out = append(out, c.s)
		}
	}
	return out
}

func (lg *League) gmArea(area string) []gmCand {
	switch area {
	case GMAreaLineup:
		return lg.gmLineup()
	case GMAreaTraining:
		return lg.gmTraining()
	case GMAreaContracts:
		return lg.gmContracts()
	case GMAreaFreeAgency:
		return lg.gmFreeAgency()
	case GMAreaDraft:
		return lg.gmDraft()
	case GMAreaTrades:
		return lg.gmTrades()
	}
	return nil
}

// gmParseID checks the shape of an id and says which area it belongs to.
func gmParseID(id string) (string, bool) {
	parts := strings.Split(id, ":")
	isInt := func(s string) bool { _, err := strconv.Atoi(s); return err == nil }
	isHex := func(s string) bool { _, err := strconv.ParseUint(s, 16, 32); return err == nil && len(s) > 0 }
	n := len(parts)
	switch parts[0] {
	case "lineup":
		return GMAreaLineup, id == "lineup:auto"
	case "focus":
		if n == 3 && isInt(parts[1]) {
			_, ok := FocusFromKey(parts[2])
			return GMAreaTraining, ok
		}
	case "resign", "extend", "release":
		return GMAreaContracts, n == 2 && isInt(parts[1])
	case "fa":
		return GMAreaFreeAgency, n == 2 && isInt(parts[1])
	case "draft":
		return GMAreaDraft, n == 2 && isInt(parts[1])
	case "trade":
		if n == 5 && parts[1] == "sell" {
			return GMAreaTrades, isInt(parts[2]) && isInt(parts[3]) && isHex(parts[4])
		}
		if n == 4 && parts[1] == "get" {
			return GMAreaTrades, isInt(parts[2]) && isHex(parts[3])
		}
	}
	return "", false
}

// GMApply carries out one suggestion by id and logs it. The error is
// ErrGMBadID (wrapped) for an id that is not a suggestion at all, and a plain
// sentence when it no longer applies. For a trade the owner's click is the
// approval: the package is worked out again and executed only if it is still
// the one that was shown.
func (lg *League) GMApply(id string) (string, error) {
	area, ok := gmParseID(id)
	if !ok {
		return "", fmt.Errorf("%w", ErrGMBadID)
	}
	for _, c := range lg.gmArea(area) {
		if c.s.ID != id {
			continue
		}
		msg, err := c.do()
		if err != nil {
			return "", err
		}
		lg.gmLog(area, msg, false)
		return msg, nil
	}
	if area == GMAreaTrades {
		return "", errors.New("That trade idea has changed or is gone. Open the trade screen to see what is possible now.")
	}
	return "", errors.New("That suggestion no longer applies.")
}

// ---- lineup ----

func (lg *League) gmLineup() []gmCand {
	user := lg.UserTeamID
	t := lg.Teams[user]
	l := t.Lineup
	if l == nil {
		return nil
	}
	custom := map[int]bool{}
	var reasons []string
	gone := 0
	for _, id := range l.Starters {
		p := t.player(id)
		if p == nil {
			gone++
			continue
		}
		custom[id] = true
		if p.Injured() {
			reasons = append(reasons, fmt.Sprintf("%s is injured and still in your starting five.", p.Name))
		}
	}
	if gone > 0 {
		reasons = append(reasons, "A starter in your lineup is no longer on the team.")
	}
	auto := lg.AutoLineup(user)
	inAuto := map[int]bool{}
	for _, id := range auto.Starters {
		inAuto[id] = true
	}
	var bestNew, worstOld *Player
	for _, id := range auto.Starters {
		if p := t.player(id); p != nil && !custom[id] && (bestNew == nil || p.Overall() > bestNew.Overall()) {
			bestNew = p
		}
	}
	for id := range custom {
		if p := t.player(id); p != nil && !inAuto[id] && !p.Injured() && (worstOld == nil || p.Overall() < worstOld.Overall() || (p.Overall() == worstOld.Overall() && p.ID > worstOld.ID)) {
			worstOld = p
		}
	}
	if bestNew != nil && worstOld != nil && bestNew.Overall()-worstOld.Overall() >= GMTuning.LineupGap {
		reasons = append(reasons, fmt.Sprintf("%s (%d) would start in the automatic lineup ahead of %s (%d).", bestNew.Name, bestNew.Overall(), worstOld.Name, worstOld.Overall()))
	}
	if len(reasons) == 0 {
		return nil
	}
	if len(reasons) > 3 {
		reasons = reasons[:3]
	}
	return []gmCand{{
		s: GMSuggestion{ID: "lineup:auto", Area: GMAreaLineup, Title: "Reset your lineup to the best available five.",
			Reasons: reasons, Action: "Reset the lineup", PlayerID: -1, Link: "/lineup"},
		do: func() (string, error) {
			lg.ClearLineup(user)
			return "Your lineup is automatic again.", nil
		},
	}}
}

// ---- training ----

// focusGap is the average room to 99 in the open ratings a focus favours.
func focusGap(p *Player, f Focus) float64 {
	rt := p.Ratings.array()
	sum, n := 0.0, 0
	for _, i := range focusRatings[f] {
		if windowOpen(p.Age, i) {
			sum += float64(99 - rt[i])
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func (lg *League) gmTraining() []gmCand {
	user := lg.UserTeamID
	ps := append([]*Player(nil), lg.Teams[user].Roster...)
	sort.SliceStable(ps, func(i, j int) bool {
		if a, b := ps[i].Overall(), ps[j].Overall(); a != b {
			return a > b
		}
		return ps[i].ID < ps[j].ID
	})
	var out []gmCand
	for _, p := range ps {
		want := autoFocus(p)
		if p.Focus == want || len(out) >= GMTuning.MaxTraining {
			continue
		}
		p, want := p, want
		var reason string
		if want == FocusBalanced {
			reason = fmt.Sprintf("At %d his ratings change little, so a specific focus gains nothing; balanced is the safe choice.", p.Age)
		} else {
			reason = fmt.Sprintf("His %s ratings have the most room to grow: about %d points to 99 on average.", lowerFirst(want.Label()), int(focusGap(p, want)+0.5))
		}
		out = append(out, gmCand{
			s: GMSuggestion{ID: fmt.Sprintf("focus:%d:%s", p.ID, want.Key()), Area: GMAreaTraining,
				Title:   fmt.Sprintf("Set %s's training focus to %s.", p.Name, want.Label()),
				Reasons: []string{reason, fmt.Sprintf("His focus is %s now.", p.Focus.Label())},
				Action:  "Set the focus", PlayerID: p.ID},
			do: func() (string, error) {
				if err := lg.SetFocus(user, p.ID, want); err != nil {
					return "", err
				}
				return fmt.Sprintf("Set %s's training focus to %s.", p.Name, want.Label()), nil
			},
		})
	}
	return out
}

// ---- contracts ----

// gmWorth is the re-sign and extension rule: the computer teams' rule (trade
// value to the user at least Cap.ReSignMinValue), tilted by the plan. The
// string is the plan's part of the reasons ("" when the plan did not matter).
func (lg *League) gmWorth(p *Player, plan string, cf []float64, needMult float64) (bool, string) {
	v := lg.value(p, lg.UserTeamID, cf)
	need := Cap.ReSignMinValue * needMult
	note := ""
	switch plan {
	case GMPlanWinNow:
		if p.Age >= GMTuning.WinNowVetAge {
			v *= GMTuning.WinNowVetBoost
			note = "Win now: a veteran who helps today."
		}
	case GMPlanRebuild:
		if p.Age > GMTuning.RebuildKeepAge {
			need *= GMTuning.RebuildStarMult
			note = fmt.Sprintf("Rebuild: only a clear star is kept past age %d.", GMTuning.RebuildKeepAge)
		} else {
			note = "Rebuild: young enough to grow with the team."
		}
	}
	return v >= need, note
}

// gmPayLimit is the payroll the assistant will not pass when it signs.
func gmPayLimit(plan string) int {
	if plan == GMPlanWinNow {
		return min(Cap.HardCeiling, Cap.TaxLine+GMTuning.WinNowTaxRoom)
	}
	return Cap.TaxLine
}

// gmIsStar says whether p is one of the StarRank best players of the user's
// roster (by overall) and young enough to be worth keeping at any price near
// the tax line.
func (lg *League) gmIsStar(p *Player) bool {
	return p.Age <= GMTuning.StarMaxAge && rankOnTeam(lg.Teams[lg.UserTeamID], p) < GMTuning.StarRank
}

// gmResignLimit is the payroll limit for re-signing or extending p: the plan's
// limit, lifted by StarTaxRoom for a star (never past the hard ceiling).
func (lg *League) gmResignLimit(plan string, p *Player) int {
	limit, _ := lg.gmResignLimitOwner(plan, p)
	return limit
}

// gmResignLimitOwner is gmResignLimit also held to the owner's budget (owner.go;
// a star may pass it only for a win-now owner). bound says the budget is what
// sets the limit.
func (lg *League) gmResignLimitOwner(plan string, p *Player) (limit int, bound bool) {
	limit = gmPayLimit(plan)
	star := plan != GMPlanRebuild && lg.gmIsStar(p)
	if star {
		limit = max(limit, min(Cap.HardCeiling, Cap.TaxLine+GMTuning.StarTaxRoom))
	}
	if c, ok := lg.ownerCapFor(star); ok && c < limit {
		return c, true
	}
	return limit, false
}

// gmNeedMult is the multiple of the usual minimum value a player must reach
// when taking his salary would put the payroll within NearTaxRoom of the tax
// line (1 for a star or when there is room).
func (lg *League) gmNeedMult(p *Player, payAfter int) float64 {
	if lg.gmIsStar(p) || payAfter <= Cap.TaxLine-GMTuning.NearTaxRoom {
		return 1
	}
	return GMTuning.NearTaxNeedMult
}

// gmSellIDs is the set of the user's players the advisor says to sell. For
// plans balanced and rebuild the assistant decides "sell" over "extend": such
// a player gets no re-sign or extension suggestion. Under win now the plan
// decides the other way round (see gmTrades, which then leaves out of its sell
// ideas anybody the contract rules would keep).
func (lg *League) gmSellIDs(plan string) map[int]bool {
	out := map[int]bool{}
	if plan == GMPlanWinNow {
		return out
	}
	rows, _ := lg.AdvisorOf()
	for _, r := range rows {
		if r.Advice == AdviceSell {
			out[r.Player.ID] = true
		}
	}
	return out
}

// gmKeepIDs is the set of players the contract rules would re-sign or extend
// right now (no release).
func (lg *League) gmKeepIDs() map[int]bool {
	out := map[int]bool{}
	for _, c := range lg.gmContracts() {
		if strings.HasPrefix(c.s.ID, "resign:") || strings.HasPrefix(c.s.ID, "extend:") {
			out[c.s.PlayerID] = true
		}
	}
	return out
}

func (lg *League) gmContracts() []gmCand {
	user := lg.UserTeamID
	t := lg.Teams[user]
	plan := lg.GMPlan()
	cf := lg.contendFactors()
	sells := lg.gmSellIDs(plan)
	var out []gmCand
	add := func(c gmCand) {
		if len(out) < GMTuning.MaxContracts {
			out = append(out, c)
		}
	}
	byValue := func(ps []*Player) {
		sort.SliceStable(ps, func(i, j int) bool {
			vi, vj := lg.value(ps[i], user, cf), lg.value(ps[j], user, cf)
			if vi != vj {
				return vi > vj
			}
			return ps[i].ID < ps[j].ID
		})
	}

	// Re-sign: expiring players, before free agency.
	if lg.FA == nil && lg.Draft != nil {
		exp := lg.Expiring(user)
		byValue(exp)
		pay := lg.Payroll(user)
		for _, p := range exp {
			p := p
			if lg.CanReSign(p) != nil || sells[p.ID] {
				continue
			}
			salary, years := AskingContract(p)
			ok, note := lg.gmWorth(p, plan, cf, lg.gmNeedMult(p, pay+salary))
			if !ok || pay+salary > lg.gmResignLimit(plan, p) || lg.CanSign(user, salary, SignOwn) != nil {
				continue
			}
			pay += salary
			reasons := []string{fmt.Sprintf("He is %d years old and rated %d, and his ask is fair for that.", p.Age, p.Overall()),
				fmt.Sprintf("He says: %s.", lowerFirst(lg.StanceOf(p).Label))}
			if note != "" {
				reasons = append(reasons, note)
			}
			if pay+salary > gmPayLimit(plan) {
				reasons = append(reasons, "He is one of your best players, so a little luxury tax is worth it.")
			}
			if _, bound := lg.gmResignLimitOwner(plan, p); bound {
				reasons = append(reasons, "It keeps you inside the owner's budget.")
			}
			add(gmCand{
				s: GMSuggestion{ID: fmt.Sprintf("resign:%d", p.ID), Area: GMAreaContracts,
					Title:   fmt.Sprintf("Re-sign %s for %s over %s.", p.Name, fmtM(salary), yearsText(years)),
					Reasons: reasons, Action: "Re-sign him", PlayerID: p.ID, Link: ""},
				do: func() (string, error) {
					salary, years := AskingContract(p)
					if err := lg.ReSign(user, p.ID); err != nil {
						return "", err
					}
					return fmt.Sprintf("Re-signed %s for %s over %s.", p.Name, fmtM(salary), yearsText(years)), nil
				},
			})
		}
	}

	// Extend: players in the last year of the contract.
	var last []*Player
	for _, p := range t.Roster {
		if p.Contract.Years == 1 && p.Contract.NextYears == 0 {
			if _, _, err := lg.ExtensionAsk(user, p.ID); err == nil {
				last = append(last, p)
			}
		}
	}
	byValue(last)
	for _, p := range last {
		p := p
		if sells[p.ID] {
			continue
		}
		salary, maxYears, _ := lg.ExtensionAsk(user, p.ID)
		_, askYears := AskingContract(p)
		years := min(askYears, maxYears)
		if p.Age >= GMTuning.OldYearsAge {
			years = min(years, GMTuning.OldMaxYears)
		}
		next := salary
		for _, q := range t.Roster {
			switch {
			case q == p:
			case q.Contract.Years > 1:
				next += q.Contract.Salary
			case q.Contract.NextYears > 0:
				next += q.Contract.NextSalary
			}
		}
		ok, note := lg.gmWorth(p, plan, cf, lg.gmNeedMult(p, next))
		if !ok || next > lg.gmResignLimit(plan, p) {
			continue
		}
		reasons := []string{fmt.Sprintf("He is in the last year of his deal, %d years old and rated %d.", p.Age, p.Overall()),
			fmt.Sprintf("He says: %s.", lowerFirst(lg.StanceOf(p).Label))}
		if note != "" {
			reasons = append(reasons, note)
		}
		if next > gmPayLimit(plan) {
			reasons = append(reasons, "He is one of your best players, so a little luxury tax is worth it.")
		}
		if _, bound := lg.gmResignLimitOwner(plan, p); bound {
			reasons = append(reasons, "It keeps you inside the owner's budget.")
		}
		if years < min(askYears, maxYears) {
			reasons = append(reasons, fmt.Sprintf("At %d the deal is kept to %s.", p.Age, yearsText(years)))
		}
		add(gmCand{
			s: GMSuggestion{ID: fmt.Sprintf("extend:%d", p.ID), Area: GMAreaContracts,
				Title:   fmt.Sprintf("Extend %s for %s over %s.", p.Name, fmtM(salary), yearsText(years)),
				Reasons: reasons, Action: "Extend him", PlayerID: p.ID, Link: ""},
			do: func() (string, error) {
				if err := lg.Extend(user, p.ID, salary, years); err != nil {
					return "", err
				}
				return fmt.Sprintf("Extended %s for %s over %s.", p.Name, fmtM(salary), yearsText(years)), nil
			},
		})
	}

	// Release: only to open a roster spot for a better free agent, and only a
	// cheap player outside the rotation.
	if lg.FA != nil && lg.FA.Day < lg.FA.Days && len(t.Roster) >= RosterSize {
		if targets := lg.gmFATargets(plan, cf); len(targets) > 0 {
			tg := targets[0]
			w := lg.worstKeep(t)
			if w != nil && rankOnTeam(t, w) >= GMTuning.ReleaseMinRank && w.Contract.Years <= GMTuning.ReleaseMaxYears &&
				w.Contract.Salary*max(1, w.Contract.Years) <= GMTuning.ReleaseMaxDead && w.Contract.NextYears == 0 &&
				w.Overall() < tg.p.Overall() && lg.ReleasePreview(user, w.ID).Allowed {
				dead := 0
				if w.Contract.Years >= 1 {
					dead = w.Contract.Salary
				}
				add(gmCand{
					s: GMSuggestion{ID: fmt.Sprintf("release:%d", w.ID), Area: GMAreaContracts,
						Title: fmt.Sprintf("Release %s to make room for %s.", w.Name, tg.p.Name),
						Reasons: []string{
							fmt.Sprintf("Your roster is full and %s is your %s best player (%d).", w.Name, ordinal(rankOnTeam(t, w)+1), w.Overall()),
							fmt.Sprintf("You would still pay %s this season in dead money, but %s (%d) is a clear step up.", fmtM(dead), tg.p.Name, tg.p.Overall()),
						},
						Action: "Release him", PlayerID: w.ID},
					do: func() (string, error) {
						if err := lg.Release(user, w.ID); err != nil {
							return "", err
						}
						return fmt.Sprintf("Released %s to open a roster spot.", w.Name), nil
					},
				})
			}
		}
	}
	return out
}

func ordinal(n int) string {
	suf := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suf = "st"
		case 2:
			suf = "nd"
		case 3:
			suf = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suf)
}

// ---- free agency ----

// gmLack is one of the user's needs as a position or a skill.
type gmLack struct {
	pos, skill int // -1 when not that kind
	text       string
}

// gmParseLacks turns NeedsOf's sentences back into positions and skills.
func gmParseLacks(lacks []string) []gmLack {
	var out []gmLack
	for _, l := range lacks {
		g := gmLack{pos: -1, skill: -1, text: l}
		for si, d := range skillDefs {
			if l == d.none || l == d.weak {
				g.skill = si
			}
		}
		if g.skill < 0 {
			low := strings.ToLower(l)
			for i, n := range posNames {
				if strings.Contains(low, n) {
					g.pos = i
				}
			}
		}
		if g.pos >= 0 || g.skill >= 0 {
			out = append(out, g)
		}
	}
	return out
}

func (lg *League) gmFills(p *Player, l gmLack) bool {
	if l.pos >= 0 {
		return p.CanPlay(Position(l.pos))
	}
	d := skillDefs[l.skill]
	if p.Ratings.array()[d.rating] >= GMTuning.SkillRating {
		return true
	}
	set := lg.badgeSet(p)
	for _, k := range d.badges {
		if idx := badgeIdx(k); idx >= 0 && set.Tier[idx] > 0 {
			return true
		}
	}
	return false
}

// gmFAT is a free agent worth an offer, with the offer's terms.
type gmFAT struct {
	p             *Player
	salary, years int
	kind          SignKind
	score         float64
	fills         []string
	ownerBound    bool // the owner's budget set the pay limit
}

// gmFATargets lists the free agents worth an offer, best first, each with
// terms the money rules accept right now (the roster spot is the caller's
// business). It uses the same checks MakeOffer uses (offerKind).
func (lg *League) gmFATargets(plan string, cf []float64) []gmFAT {
	if lg.FA == nil || lg.FA.Day >= lg.FA.Days {
		return nil
	}
	user := lg.UserTeamID
	t := lg.Teams[user]
	limit := gmPayLimit(plan)
	ownerBound := false
	if c, ok := lg.ownerCapFor(false); ok && c < limit {
		limit, ownerBound = c, true // the owner's budget (owner.go)
	}
	lacks := gmParseLacks(lg.NeedsOf(user).Lacks)
	mine := map[int]bool{}
	for _, o := range lg.OffersBy(user) {
		mine[o.PlayerID] = true
	}
	// The overall a newcomer must beat once the roster is deep enough.
	tenth := 0
	if rot := rotationOf(t, 10); len(rot) >= 10 {
		tenth = rot[9].Overall()
	}
	var out []gmFAT
	for _, p := range lg.sortedPool() {
		if mine[p.ID] || lg.sheetFor(p.ID) != nil {
			continue
		}
		if rt, ok := lg.RightsTeam(p.ID); ok && rt != user {
			continue // another team's restricted player: an offer sheet, left to the owner
		}
		salary, years := lg.FreeAgentAsk(p)
		years = max(years, lg.MinYearsFor(user, p.ID))
		kind, err := lg.offerKind(user, p.ID, salary, years)
		if err != nil {
			continue
		}
		if lg.Payroll(user)+lg.pendingAbove(user, p.ID)+lg.pendingOutsideRoom(user, p.ID)+salary > limit {
			continue
		}
		v := lg.value(p, user, cf)
		if v < GMTuning.FAMinValue || p.Overall() <= tenth {
			continue
		}
		var fills []string
		for _, l := range lacks {
			if lg.gmFills(p, l) {
				fills = append(fills, l.text)
			}
		}
		bonus := GMTuning.LackBonus * float64(min(2, len(fills)))
		var score float64
		switch plan {
		case GMPlanWinNow:
			score = 0.5*v + 0.35*float64(p.Overall()-55) + bonus
		case GMPlanRebuild:
			if p.Age > GMTuning.RebuildFAMaxAge {
				continue
			}
			score = v + 0.2*float64(p.Potential-p.Overall()) + bonus
		default:
			score = v + bonus
		}
		out = append(out, gmFAT{p, salary, years, kind, score, fills, ownerBound})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].p.ID < out[j].p.ID
	})
	return out
}

// gmCutFor is the player a full roster would give up for the free agent p, or
// nil. It is exactly the engine's own rule for a full computer team that signs
// an upgrade (signWithCut): p's keep score beats the worst player's by
// FATuning.UpgradeMargin. On top of that the assistant stays conservative with
// the user's money: the worst player must be outside the top ten, on a short,
// cheap deal with no extension agreed, and his release must be allowed.
func (lg *League) gmCutFor(t *Team, p *Player) *Player {
	w := lg.worstKeep(t)
	if w == nil || keepScore(p) < keepScore(w)+FATuning.UpgradeMargin {
		return nil
	}
	if rankOnTeam(t, w) < GMTuning.ReleaseMinRank || w.Contract.Years > GMTuning.ReleaseMaxYears ||
		w.Contract.NextYears != 0 || w.Contract.Salary*max(1, w.Contract.Years) > GMTuning.ReleaseMaxDead ||
		!lg.ReleasePreview(t.ID, w.ID).Allowed {
		return nil
	}
	return w
}

func (lg *League) gmFreeAgency() []gmCand {
	if lg.FA == nil || lg.FA.Day >= lg.FA.Days {
		return nil
	}
	user := lg.UserTeamID
	t := lg.Teams[user]
	offers := len(lg.OffersBy(user))
	spots := RosterSize - len(t.Roster) - offers
	// A full roster: the engine lets a full computer team bid for a clear
	// upgrade and cut its worst player when he signs. The user's roster is not
	// cut by the engine, so the assistant releases that same player first and
	// makes the offer in one step, once at a time (never past 13).
	cutting := spots <= 0 && len(t.Roster) == RosterSize && offers == 0
	if spots <= 0 && !cutting {
		return nil
	}
	plan := lg.GMPlan()
	cf := lg.contendFactors()
	var out []gmCand
	for _, tg := range lg.gmFATargets(plan, cf) {
		if len(out) >= min(GMTuning.MaxFA, max(spots, 1)) {
			break
		}
		tg := tg
		p := tg.p
		var cut *Player
		if cutting {
			if cut = lg.gmCutFor(t, p); cut == nil {
				continue
			}
		}
		var reasons []string
		if cut != nil {
			dead := 0
			if cut.Contract.Years >= 1 {
				dead = cut.Contract.Salary
			}
			reasons = append(reasons, fmt.Sprintf("Your roster is full. %s (%d, age %d) is your weakest player and %s (%d) is a clear step up; this is the cut a computer team makes.",
				cut.Name, cut.Overall(), cut.Age, p.Name, p.Overall()))
			if dead > 0 {
				reasons = append(reasons, fmt.Sprintf("You still pay %s of his contract this season as dead money.", fmtM(dead)))
			}
		}
		if len(tg.fills) > 0 {
			reasons = append(reasons, "Fills a need: "+lowerFirst(tg.fills[0])+".")
		}
		if cut == nil {
			reasons = append(reasons, fmt.Sprintf("He is %d years old, rated %d with %d potential.", p.Age, p.Overall(), p.Potential))
		}
		if tg.ownerBound {
			reasons = append(reasons, "It keeps you inside the owner's budget.")
		}
		switch tg.kind {
		case SignException:
			reasons = append(reasons, "It uses your once-a-year exception, so no cap room is needed.")
		case SignMinimum:
			reasons = append(reasons, "He takes the minimum contract.")
		default:
			reasons = append(reasons, "It fits under your cap room and the tax line.")
		}
		switch plan {
		case GMPlanWinNow:
			reasons = append(reasons[:len(reasons)-1], "Win now: one of the best players available today.")
		case GMPlanRebuild:
			reasons = append(reasons[:len(reasons)-1], "Rebuild: young with room to grow.")
		}
		if len(reasons) > 3 {
			reasons = reasons[:3]
		}
		title := fmt.Sprintf("Offer %s %s a year for %s.", p.Name, fmtM(tg.salary), yearsText(tg.years))
		action := "Make the offer"
		if cut != nil {
			title = fmt.Sprintf("Offer %s %s a year for %s and release %s to make room.", p.Name, fmtM(tg.salary), yearsText(tg.years), cut.Name)
			action = "Release and offer"
		}
		cutID := -1
		if cut != nil {
			cutID = cut.ID
		}
		out = append(out, gmCand{
			manual: cut != nil,
			s: GMSuggestion{ID: fmt.Sprintf("fa:%d", p.ID), Area: GMAreaFreeAgency,
				Title: title, Reasons: reasons, Action: action, PlayerID: p.ID, Link: "/free-agency"},
			do: func() (string, error) {
				if cutID < 0 {
					if _, err := lg.MakeOffer(user, p.ID, tg.salary, tg.years); err != nil {
						return "", err
					}
					return fmt.Sprintf("Offered %s %s a year for %s. He answers as the days pass.", p.Name, fmtM(tg.salary), yearsText(tg.years)), nil
				}
				w := lg.findRosterPlayer(user, cutID)
				if w == nil || len(lg.Teams[user].Roster) != RosterSize || lg.gmCutFor(lg.Teams[user], p) != w {
					return "", errors.New("That suggestion no longer applies.")
				}
				// Check the money first so a failed offer never follows a release
				// (the release keeps the payroll the same: dead money takes the
				// salary's place).
				if _, err := lg.offerKind(user, p.ID, tg.salary, tg.years); err != nil {
					return "", err
				}
				if err := lg.Release(user, w.ID); err != nil {
					return "", err
				}
				if _, err := lg.MakeOffer(user, p.ID, tg.salary, tg.years); err != nil {
					lg.gmLog(GMAreaFreeAgency, fmt.Sprintf("Released %s but the offer to %s failed: %v", w.Name, p.Name, err), true)
					return "", err
				}
				return fmt.Sprintf("Released %s and offered %s %s a year for %s. He answers as the days pass.", w.Name, p.Name, fmtM(tg.salary), yearsText(tg.years)), nil
			},
		})
		if cutting {
			break
		}
	}
	return out
}

// ---- draft ----

func (lg *League) gmDraft() []gmCand {
	d := lg.Draft
	user := lg.UserTeamID
	if d == nil || lg.DraftDone() || lg.DraftOnClock() != user {
		return nil
	}
	plan := lg.GMPlan()
	wo := Progression.DraftOvrWeight
	switch plan {
	case GMPlanWinNow:
		wo = GMTuning.DraftWinNowOvr
	case GMPlanRebuild:
		wo = GMTuning.DraftRebuildOvr
	}
	var best *Player
	var bv ScoutView
	bestScore := 0.0
	for _, p := range d.Pool {
		v, ok := lg.ScoutView(p.ID)
		if !ok {
			continue
		}
		// The midpoints the Draft screen shows.
		s := wo*float64((v.OvrLo+v.OvrHi+1)/2) + (1-wo)*float64((v.PotLo+v.PotHi+1)/2)
		if best == nil || s > bestScore {
			best, bv, bestScore = p, v, s
		}
	}
	if best == nil {
		return nil
	}
	p := best
	reasons := []string{
		fmt.Sprintf("Your scouting puts him at %d to %d overall and %d to %d potential.", bv.OvrLo, bv.OvrHi, bv.PotLo, bv.PotHi),
		"He is the best blend of overall and potential among the prospects left, by your scouting.",
	}
	switch plan {
	case GMPlanWinNow:
		reasons[1] = "Win now: the blend leans toward who can play soon."
	case GMPlanRebuild:
		reasons[1] = "Rebuild: the blend leans toward potential."
	}
	return []gmCand{{
		s: GMSuggestion{ID: fmt.Sprintf("draft:%d", p.ID), Area: GMAreaDraft,
			Title:   fmt.Sprintf("Draft %s (%s, age %d) with pick #%d.", p.Name, p.PosLabel(), p.Age, d.Made+1),
			Reasons: reasons, Action: "Draft him", PlayerID: p.ID, Link: "/draft"},
		do: func() (string, error) {
			n := d.Made + 1
			if err := lg.DraftPick(p.ID); err != nil {
				return "", err
			}
			return fmt.Sprintf("Drafted %s with pick #%d.", p.Name, n), nil
		},
	}}
}

// ---- trades ----

// gmHash is a short stable code for a set of assets, so a trade suggestion
// can tell whether the package is still the one that was shown.
func gmHash(assets []Asset) string {
	ks := make([]string, len(assets))
	for i, a := range assets {
		ks[i] = fmt.Sprintf("%d/%d/%d/%d/%d", a.Kind, a.PlayerID, a.Year, a.Round, a.Original)
	}
	sort.Strings(ks)
	h := fnv.New32a()
	h.Write([]byte(strings.Join(ks, ",")))
	return fmt.Sprintf("%06x", h.Sum32()&0xffffff)
}

// gmAssetText names an asset of team owner for a sentence.
func (lg *League) gmAssetText(a Asset, owner int) string {
	if a.Kind == PlayerAsset {
		if p := lg.findRosterPlayer(owner, a.PlayerID); p != nil {
			return p.Name
		}
		if p := lg.FindPlayer(a.PlayerID); p != nil {
			return p.Name
		}
		return "a player"
	}
	rd := "first"
	if a.Round == 2 {
		rd = "second"
	}
	return fmt.Sprintf("the %s %d %s-round pick", lg.Teams[a.Original].Name, a.Year, rd)
}

func (lg *League) gmAssetsText(as []Asset, owner int) string {
	xs := make([]string, len(as))
	for i, a := range as {
		xs[i] = lg.gmAssetText(a, owner)
	}
	return joinList(xs)
}

// rotationMean8 is the mean overall of the eight best of ps.
func rotationMean8(ps []*Player) float64 {
	v := make([]float64, len(ps))
	for i, p := range ps {
		v[i] = float64(p.Overall())
	}
	return topMean(v, 8)
}

func (lg *League) gmTrades() []gmCand {
	if ok, _ := TradeWindowOpen(lg); !ok {
		return nil
	}
	user := lg.UserTeamID
	plan := lg.GMPlan()
	var out []gmCand

	// (a) sell: players the advisor would move, with the best offer for each.
	// A player the contract rules would re-sign or extend is not sold (and a
	// player sold is not extended: see gmSellIDs); a valuable player in his last
	// year who refuses an extension and will test free agency is sold before he
	// walks for nothing (except under win now, which keeps him for the run).
	rows, _ := lg.AdvisorOf()
	keep := lg.gmKeepIDs()
	want := "any"
	if plan == GMPlanRebuild {
		want = "young"
	}
	cf := lg.contendFactors()
	tried := 0
	for _, r := range rows {
		if len(out) >= GMTuning.MaxSells || tried >= GMTuning.SellCandidates {
			break
		}
		walks := false
		if plan != GMPlanWinNow && r.Advice != AdviceSell && r.Player.Contract.Years == 1 && r.Player.Contract.NextYears == 0 &&
			lg.StanceOf(r.Player).Key == "test" && lg.value(r.Player, user, cf) >= Cap.ReSignMinValue {
			walks = true
		}
		if r.Advice != AdviceSell && !(r.Advice == AdviceShop && plan == GMPlanRebuild) && !walks {
			continue
		}
		if keep[r.Player.ID] {
			continue
		}
		tried++
		p := r.Player
		give := []Asset{PlayerOf(p.ID)}
		res, err := lg.FindOffers(give, want)
		if (err != nil || !res.Possible || len(res.Offers) == 0) && want != "any" {
			res, err = lg.FindOffers(give, "any")
		}
		if err != nil || !res.Possible || len(res.Offers) == 0 {
			continue
		}
		o := res.Offers[0]
		if o.Value < GMTuning.SellMinReturn*res.GiveValue {
			continue
		}
		hash := gmHash(o.Receive)
		tid := o.Team
		recv := lg.gmAssetsText(o.Receive, tid)
		reasons := append([]string(nil), r.Reasons...)
		if walks {
			reasons = []string{"He will test free agency and cannot be extended, so he would leave for nothing."}
		}
		if o.Note != "" {
			reasons = append(reasons, o.Note)
		}
		if len(reasons) > 2 {
			reasons = reasons[:2]
		}
		reasons = append(reasons, "So he is sold, not extended or re-signed.")
		wantNow := want
		out = append(out, gmCand{
			s: GMSuggestion{ID: fmt.Sprintf("trade:sell:%d:%d:%s", p.ID, tid, hash), Area: GMAreaTrades,
				Title:   fmt.Sprintf("Trade %s to the %s for %s.", p.Name, lg.Teams[tid].Name, recv),
				Reasons: reasons, Action: "Open the trade", PlayerID: p.ID, Link: fmt.Sprintf("/trade?find=%d", p.ID)},
			do: func() (string, error) {
				res, err := lg.FindOffers(give, wantNow)
				if err != nil {
					return "", err
				}
				for _, o := range res.Offers {
					if o.Team != tid {
						continue
					}
					if gmHash(o.Receive) != hash {
						break
					}
					if _, err := ExecuteDeal(lg, o.Deal(user, give), user); err != nil {
						return "", err
					}
					return fmt.Sprintf("Traded %s to the %s for %s.", p.Name, lg.Teams[tid].Name, lg.gmAssetsText(o.Receive, user)), nil
				}
				return "", errors.New("That offer has changed or is gone. Open the trade screen to see what is possible now.")
			},
		})
	}

	// (b) upgrade at the biggest weak spot, from a team that is selling.
	if lg.gmUpgradeAllowed(plan) && len(out) < GMTuning.MaxTrades {
		if c, ok := lg.gmUpgrade(plan); ok {
			out = append(out, c)
		}
	}
	if len(out) > GMTuning.MaxTrades {
		out = out[:GMTuning.MaxTrades]
	}
	return out
}

// gmUpgradeAllowed says whether the assistant may propose a trade that adds a
// better player at the cost of assets: only for plan win now, or for balanced
// when the team is a contender or a buyer; never for rebuild, and never for a
// seller or a rebuilding team whatever the plan.
func (lg *League) gmUpgradeAllowed(plan string) bool {
	dir := lg.TeamStatusOf(lg.UserTeamID).Key
	if dir != "contender" && dir != "buyer" {
		return false
	}
	return plan == GMPlanWinNow || plan == GMPlanBalanced
}

// gmUpgrade looks for one player at the user's biggest weak spot on a selling
// or rebuilding team and a package the real rules accept. The package may not
// hold a player the contract rules would re-sign or extend, more than
// UpgradeMaxCore of the user's five best players by value, or more first-round
// picks than the plan allows; the target's age must fit the plan.
func (lg *League) gmUpgrade(plan string) (gmCand, bool) {
	user := lg.UserTeamID
	need := lg.NeedsOf(user)
	lacks := gmParseLacks(need.Lacks)
	if len(lacks) == 0 {
		return gmCand{}, false
	}
	lack := lacks[0]
	mine := lg.Teams[user].Roster
	floor := 0
	if rot := rotationOf(lg.Teams[user], 6); len(rot) >= 6 {
		floor = rot[5].Overall()
	}
	sts := lg.TeamStatuses()
	maxAge, maxFirsts := GMTuning.UpgradeMaxAge, GMTuning.UpgradeMaxFirsts
	if plan == GMPlanWinNow {
		maxAge, maxFirsts = GMTuning.UpgradeWinNowMaxAge, GMTuning.UpgradeWinNowMaxFirsts
	}
	keep := lg.gmKeepIDs()
	cf := lg.contendFactors()
	byVal := append([]*Player(nil), mine...)
	sort.SliceStable(byVal, func(i, j int) bool {
		vi, vj := lg.value(byVal[i], user, cf), lg.value(byVal[j], user, cf)
		if vi != vj {
			return vi > vj
		}
		return byVal[i].ID < byVal[j].ID
	})
	core := map[int]bool{}
	for i, p := range byVal {
		if i < 5 {
			core[p.ID] = true
		}
	}
	var cands []*Player
	for tid, t := range lg.Teams {
		if tid == user || (sts[tid].Key != "seller" && sts[tid].Key != "rebuilding") {
			continue
		}
		for _, p := range t.Roster {
			if p.Overall() >= floor && p.Age <= maxAge && lg.gmFills(p, lack) {
				cands = append(cands, p)
			}
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if a, b := cands[i].Overall(), cands[j].Overall(); a != b {
			return a > b
		}
		return cands[i].ID < cands[j].ID
	})
	if len(cands) > GMTuning.UpgradeCandidates {
		cands = cands[:GMTuning.UpgradeCandidates]
	}
	before := rotationMean8(mine)
	for _, target := range cands {
		res, err := lg.WhatWouldItTake(target.ID)
		if err != nil || !res.Possible || len(res.Packages) == 0 {
			continue
		}
		// the cheapest package first, then the others, whichever obeys the limits
		ordered := append([]TakePackage(nil), res.Packages...)
		sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Cheapest && !ordered[j].Cheapest })
		var pk TakePackage
		found := false
		for _, x := range ordered {
			nCore, firsts, kept := 0, 0, false
			for _, a := range x.Give {
				switch {
				case a.Kind == PlayerAsset && keep[a.PlayerID]:
					kept = true
				case a.Kind == PlayerAsset && core[a.PlayerID]:
					nCore++
				case a.Kind != PlayerAsset && a.Round == 1:
					firsts++
				}
			}
			if !kept && nCore <= GMTuning.UpgradeMaxCore && firsts <= maxFirsts {
				pk, found = x, true
				break
			}
		}
		if !found {
			continue
		}
		gone := map[int]bool{}
		for _, a := range pk.Give {
			if a.Kind == PlayerAsset {
				gone[a.PlayerID] = true
			}
		}
		after := []*Player{target}
		for _, p := range mine {
			if !gone[p.ID] {
				after = append(after, p)
			}
		}
		gain := rotationMean8(after) - before
		if gain < GMTuning.UpgradeMinGain {
			continue
		}
		tid := res.Team
		hash := gmHash(pk.Give)
		tp := target
		reasons := []string{
			"Your weak spot: " + lowerFirst(lack.text) + ".",
			fmt.Sprintf("Your top eight would average %.1f instead of %.1f.", before+gain, before),
		}
		if pk.Note != "" {
			reasons = append(reasons, pk.Note)
		}
		cand := gmCand{
			s: GMSuggestion{ID: fmt.Sprintf("trade:get:%d:%s", tp.ID, hash), Area: GMAreaTrades,
				Title: fmt.Sprintf("Trade for %s (%d, %s) from the %s, sending %s.", tp.Name, tp.Overall(), tp.PosLabel(), lg.Teams[tid].Name,
					lg.gmAssetsText(pk.Give, user)),
				Reasons: reasons, Action: "Open the trade", PlayerID: tp.ID, Link: fmt.Sprintf("/trade?team=%d&want=%d", tid, tp.ID)},
			do: func() (string, error) {
				res, err := lg.WhatWouldItTake(tp.ID)
				if err != nil {
					return "", err
				}
				for _, x := range res.Packages {
					if gmHash(x.Give) != hash {
						continue
					}
					if _, err := ExecuteDeal(lg, x.Deal(user, tid), user); err != nil {
						return "", err
					}
					return fmt.Sprintf("Traded %s to the %s for %s.", lg.gmAssetsText(x.Give, user), lg.Teams[tid].Name, tp.Name), nil
				}
				return "", errors.New("That package has changed or is gone. Open the trade screen to see what is possible now.")
			},
		}
		return cand, true
	}
	return gmCand{}, false
}

// ---- auto mode (hooks called from the normal flow) ----

// gmAutoRun applies an area's suggestions one by one (recomputing after each,
// since each changes the picture) until none is left that carries out. Only
// ids with one of the prefixes are used. Everything it does is logged as auto.
func (lg *League) gmAutoRun(area string, prefixes ...string) int {
	done := 0
	for iter := 0; iter < 40; iter++ {
		applied := false
		for _, c := range lg.gmArea(area) {
			ok := len(prefixes) == 0
			for _, p := range prefixes {
				ok = ok || strings.HasPrefix(c.s.ID, p)
			}
			if !ok || c.manual {
				continue
			}
			msg, err := c.do()
			if err != nil {
				continue
			}
			lg.gmLog(area, msg, true)
			applied, done = true, done+1
			break
		}
		if !applied {
			break
		}
	}
	return done
}

func (lg *League) gmAuto() GMAuto {
	if lg.GM == nil {
		return GMAuto{}
	}
	return lg.GM.Auto
}

// gmAutoLineup runs before each game day (regular season and playoffs).
func (lg *League) gmAutoLineup() {
	if lg.gmAuto().Lineup {
		lg.gmAutoRun(GMAreaLineup)
	}
}

// gmAutoTraining runs at the end of the offseason, where computer teams get
// their focus.
func (lg *League) gmAutoTraining() {
	if lg.gmAuto().Training {
		lg.gmAutoRun(GMAreaTraining)
	}
}

// gmAutoExtend runs when the offseason begins, before contracts tick down.
func (lg *League) gmAutoExtend() {
	if lg.gmAuto().Contracts {
		lg.gmAutoRun(GMAreaContracts, "extend:")
	}
}

// gmAutoResign runs when free agency begins, before the expiring players
// are released. It never releases anyone.
func (lg *League) gmAutoResign() {
	if lg.gmAuto().Contracts {
		lg.gmAutoRun(GMAreaContracts, "resign:")
	}
}

// gmAutoFreeAgency runs at the start of each free-agency day.
func (lg *League) gmAutoFreeAgency() {
	if lg.gmAuto().FreeAgency {
		lg.gmAutoRun(GMAreaFreeAgency)
	}
}

// GMAutoDraftIfOnClock makes the user's pick by the assistant's suggestion
// when the draft switch is on and the user is on the clock. It reports
// whether a pick was made, so a draft loop can carry on instead of stopping.
func (lg *League) GMAutoDraftIfOnClock() bool {
	if !lg.gmAuto().Draft || lg.Draft == nil || lg.DraftDone() || lg.DraftOnClock() != lg.UserTeamID {
		return false
	}
	return lg.gmAutoRun(GMAreaDraft) > 0
}

// gmDayStart runs at the start of each regular-season day.
func (lg *League) gmDayStart() {
	lg.gmAlert = ""
	lg.gmAutoLineup()
}

// gmDayEnd runs at the end of each regular-season day: with the alert switch
// on it notes trade ideas it has not mentioned this season. It never trades.
func (lg *League) gmDayEnd() {
	g := lg.GM
	if g == nil || !g.Auto.TradeAlerts || lg.Season == nil {
		return
	}
	if every := GMTuning.AlertEveryDays; every > 1 && lg.Season.Day%every != 0 {
		return
	}
	cs := lg.gmTrades()
	prefix := fmt.Sprintf("%d|", lg.Year)
	for k := range g.Seen {
		if !strings.HasPrefix(k, prefix) {
			delete(g.Seen, k)
		}
	}
	for _, c := range cs {
		id := c.s.ID
		key := prefix + id[:strings.LastIndex(id, ":")] // the idea, not the exact package
		if g.Seen[key] {
			continue
		}
		if g.Seen == nil {
			g.Seen = map[string]bool{}
		}
		g.Seen[key] = true
		if lg.gmAlert == "" {
			lg.gmAlert = c.s.Title
		}
	}
}

// GMAlertLastDay is the title of a new trade idea found after the most recent
// SimDay ("" if none, or the alert switch is off). The sim loop in the web
// layer stops on it like on a new computer offer.
func (lg *League) GMAlertLastDay() string { return lg.gmAlert }

// ---- count for the state object ----

// gmStamp is a cheap fingerprint of what the suggestions depend on, so the
// count shown on every screen is not recomputed until something changes.
func (lg *League) gmStamp() uint64 {
	h := fnv.New64a()
	put := func(vs ...int) {
		for _, v := range vs {
			var b [8]byte
			for i := range b {
				b[i] = byte(uint64(v) >> (8 * i))
			}
			h.Write(b[:])
		}
	}
	put(lg.Year, len(lg.Transactions), len(lg.FreeAgents), len(lg.Proposals), len(lg.Events))
	if s := lg.Season; s != nil {
		put(s.Year, s.Day, s.PlayoffDay, b2i(s.PlayoffsStarted), s.Champion)
	}
	if d := lg.Draft; d != nil {
		put(d.Made, 1)
	}
	if fa := lg.FA; fa != nil {
		put(fa.Day, len(fa.Offers), len(fa.Sheets), 1)
	}
	t := lg.Teams[lg.UserTeamID]
	put(b2i(t.Lineup != nil), len(t.Roster), len(t.DeadMoney))
	for _, p := range t.Roster {
		put(p.ID, int(p.Focus), p.Contract.Salary, p.Contract.Years, p.Contract.NextSalary, p.Contract.NextYears, b2i(p.Injured()), p.Morale, p.Overall(), p.Age)
	}
	g := lg.GM
	if g != nil {
		put(len(g.Plan), b2i(g.Auto.Lineup), b2i(g.Auto.Training), b2i(g.Auto.Contracts), b2i(g.Auto.FreeAgency), b2i(g.Auto.Draft))
		h.Write([]byte(g.Plan))
	}
	return h.Sum64()
}

// gmCache remembers the counts for one stamp (not saved).
type gmCache struct {
	stamp uint64
	total int
	area  map[string]int
}

// GMCounts is the number of suggestions in total and per area, cached until
// the team's situation changes (the screens ask on every refresh).
func (lg *League) GMCounts() (int, map[string]int) {
	st := lg.gmStamp()
	if c := lg.gmCounts; c != nil && c.stamp == st {
		return c.total, c.area
	}
	c := &gmCache{stamp: st, area: map[string]int{}}
	for _, a := range GMAreas() {
		n := len(lg.gmArea(a[0]))
		c.area[a[0]] = n
		c.total += n
	}
	lg.gmCounts = c
	return c.total, c.area
}
