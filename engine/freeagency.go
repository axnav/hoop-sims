package engine

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// FATuning holds every free-agency number in one place.
var FATuning = struct {
	Days            int     // length of the free-agency period, in days
	SoftenTo        float64 // asking salary on the last day, as a fraction of day 0
	AcceptPct       float64 // a player takes an offer only if salary >= ask * this
	OfferLow        float64 // computer offers are the ask times a factor in OfferLow..OfferHigh
	OfferHigh       float64
	YearBonus       float64 // appeal bonus per contract year beyond the first
	TeamSwing       float64 // team factor is 1 +- TeamSwing (contending teams are better)
	RoleBonus       float64 // appeal bonus if he would be among the team's best eight
	MaxNewOffers    int     // most new offers one computer team makes per day
	SecondOfferPct  float64 // chance that a team with room makes a second offer in a day
	MinOffersAfter  int     // computer teams start offering minimum deals from this day on
	WantMinValue    float64 // a computer team bids above the minimum only for players worth at least this to it
	CandidatesPerTm int     // how many top-valued free agents a team looks at per day
	BestEffortShare float64 // a team short of cap room may offer all its room if that is at least this share of the current ask
	LateDays        int     // in the last this-many days a player settles for less (see LateShare)
	LateShare       float64 // late in the period he takes his best offer if it is at least this share of his ORIGINAL ask
	SettleAskMult   int     // after the last day, players whose original ask is at least this many times the minimum take their best real offer
	UpgradeMargin   float64 // a full computer team bids for a free agent whose keep score beats its worst player's by this much, and cuts that player on signing

	// Restricted free agency: a player whose contract ran out with at most
	// RFAMaxExperience seasons played is restricted. His old team holds the
	// rights: another team's offer is an offer sheet (at least OfferSheetMinYears
	// years) that the rights team may match.
	RFAMaxExperience   int
	OfferSheetMinYears int
}{
	Days: 10, SoftenTo: 0.70, AcceptPct: 0.95,
	OfferLow: 0.92, OfferHigh: 1.12, YearBonus: 0.02, TeamSwing: 0.08, RoleBonus: 0.03,
	MaxNewOffers: 2, SecondOfferPct: 0.4, MinOffersAfter: 5,
	WantMinValue: 2.5, CandidatesPerTm: 14, UpgradeMargin: 4,
	BestEffortShare: 0.60, LateDays: 3, LateShare: 0.50, SettleAskMult: 4,
	RFAMaxExperience: 4, OfferSheetMinYears: 2,
}

// freeAgencyOffset is the RNG stream for computer offers (20000 proposals,
// 30000 injuries, 40000 contracts), so game results never change.
const freeAgencyOffset = 50000

// FAOffer is one team's standing offer to a free agent. Day is the day it was made.
type FAOffer struct {
	ID       int
	Team     int
	PlayerID int
	Salary   int
	Years    int
	Day      int
	// Kind is how the offer would be signed: SignException, SignOwn (the rights
	// team keeping its restricted player) or SignMinimum. The zero value
	// (SignCapRoom) is the old behaviour: cap room, or minimum if the salary is.
	Kind SignKind `json:",omitempty"`
}

// FASigning is one signing during free agency.
type FASigning struct {
	Day      int
	PlayerID int
	Name     string
	Team     int
	Salary   int
	Years    int
	Pos      string
	Overall  int
	// Matched: the rights team matched an offer sheet from SheetFrom (Team is
	// then the rights team). Via is "room", "exception", "minimum" or "own"
	// (empty in old saves).
	Matched   bool   `json:",omitempty"`
	SheetFrom int    `json:",omitempty"`
	Via       string `json:",omitempty"`
}

// OfferSheet is another team's offer to a restricted player whose rights the
// user holds, waiting for the user to match it or let him go.
type OfferSheet struct {
	PlayerID int
	Team     int // the offering team
	Salary   int
	Years    int
	Day      int
}

// FreeAgency is the saved state of the free-agency period (nil outside it).
type FreeAgency struct {
	Day         int // days already resolved
	Days        int // total days
	NextOfferID int
	Offers      []FAOffer
	Log         []FASigning
	// Rights maps a restricted player to the team holding his rights.
	Rights map[int]int `json:",omitempty"`
	// Prev maps a player released into this free agency to the team he
	// played for (his Loyalty bonus in playersDecide).
	Prev map[int]int `json:",omitempty"`
	// Sheets are the pending offer sheets against the user's rights.
	Sheets []OfferSheet `json:",omitempty"`
}

// DaysLeft is how many days remain.
func (fa *FreeAgency) DaysLeft() int { return max(0, fa.Days-fa.Day) }

// CanBeginFreeAgency says whether free agency may begin now (the draft is done).
func (lg *League) CanBeginFreeAgency(reportSeen bool) error {
	if lg.FA != nil {
		return errors.New("free agency has already begun")
	}
	if lg.Phase(reportSeen) != PhaseDraftDone {
		return errors.New("the draft is not finished")
	}
	return nil
}

// BeginFreeAgency ends the re-sign window: computer teams keep who they want
// (and are trimmed to 13), then every player still expiring, the user's too,
// is released to the pool and the free-agency period opens.
func (lg *League) BeginFreeAgency() error {
	if lg.FA != nil {
		return errors.New("free agency has already begun")
	}
	if lg.Draft == nil || !lg.DraftDone() {
		return errors.New("the draft is not finished")
	}
	lg.gmAutoResign() // assistant GM: auto re-sign before the expiring are released (gm.go)
	lg.reSignComputerTeams()
	// Restricted players: the old team keeps his rights (a computer team only
	// if it would have kept him; the user always does).
	var rights map[int]int
	cf := lg.contendFactors()
	for _, t := range lg.Teams {
		for _, p := range lg.Expiring(t.ID) {
			if !lg.RestrictedEligible(p) {
				continue
			}
			if t.ID != lg.UserTeamID && lg.value(p, t.ID, cf) < Cap.ReSignMinValue {
				continue
			}
			if rights == nil {
				rights = map[int]int{}
			}
			rights[p.ID] = t.ID
		}
	}
	prev := map[int]int{}
	for _, t := range lg.Teams {
		for _, p := range lg.Expiring(t.ID) {
			prev[p.ID] = t.ID
		}
	}
	lg.releaseExpiring()
	for _, t := range lg.Teams {
		if t.ID != lg.UserTeamID {
			if len(t.Roster) > RosterSize {
				for _, p := range t.Roster {
					prev[p.ID] = t.ID // the weakest are trimmed below
				}
			}
			lg.trimRoster(t)
		}
	}
	lg.FA = &FreeAgency{Days: FATuning.Days, Rights: rights, Prev: prev}
	return nil
}

// RestrictedEligible says whether a player whose contract expires now is
// restricted (few enough seasons played, counted after this offseason's progression).
func (lg *League) RestrictedEligible(p *Player) bool {
	return p != nil && p.Experience <= FATuning.RFAMaxExperience
}

// RightsTeam is the team holding the rights to a restricted free agent.
func (lg *League) RightsTeam(playerID int) (teamID int, ok bool) {
	if lg.FA == nil {
		return 0, false
	}
	teamID, ok = lg.FA.Rights[playerID]
	return teamID, ok
}

// MinYearsFor is the shortest contract the team may offer the player: 2 for
// another team's restricted player, else 1.
func (lg *League) MinYearsFor(teamID, playerID int) int {
	if rt, ok := lg.RightsTeam(playerID); ok && rt != teamID {
		return max(1, FATuning.OfferSheetMinYears)
	}
	return 1
}

// trimRoster releases the lowest keepScore players above 13.
func (lg *League) trimRoster(t *Team) {
	sort.SliceStable(t.Roster, func(i, j int) bool { return keepScore(t.Roster[i]) > keepScore(t.Roster[j]) })
	if len(t.Roster) > RosterSize {
		for _, p := range t.Roster[RosterSize:] {
			lg.releaseToFA(p)
		}
		t.Roster = t.Roster[:RosterSize]
	}
}

// soften is the asking-salary multiplier on a day (1 on day 0, SoftenTo on the last).
func soften(day, days int) float64 {
	if days <= 1 {
		return 1
	}
	d := max(0, min(day, days-1))
	return 1 - (1-FATuning.SoftenTo)*float64(d)/float64(days-1)
}

// FreeAgentAsk is what a free agent asks today: his asking contract, with the
// salary softening as the days pass (never below the minimum).
func (lg *League) FreeAgentAsk(p *Player) (salary, years int) {
	salary, years = AskingContract(p)
	if lg.FA != nil {
		s := int(math.Round(float64(salary)*soften(lg.FA.Day, lg.FA.Days)/100)) * 100
		salary = max(Cap.MinSalary, min(salary, s))
	}
	return salary, years
}

func (lg *League) faPlayer(id int) *Player {
	for _, p := range lg.FreeAgents {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// pendingAbove is the money in the team's outstanding offers above the minimum,
// leaving out any offer for the given player (which a new offer replaces).
func (lg *League) pendingAbove(teamID, exceptPlayer int) int {
	n := 0
	for _, o := range lg.FA.Offers {
		if o.Team == teamID && o.PlayerID != exceptPlayer && o.Salary > Cap.MinSalary && o.Kind != SignException && o.Kind != SignOwn {
			n += o.Salary
		}
	}
	return n
}

// pendingOutsideRoom is the money in the team's offers that do not use cap
// room (exception and own-rights offers), leaving out one player's.
func (lg *League) pendingOutsideRoom(teamID, exceptPlayer int) int {
	n := 0
	for _, o := range lg.FA.Offers {
		if o.Team == teamID && o.PlayerID != exceptPlayer && (o.Kind == SignException || o.Kind == SignOwn) {
			n += o.Salary
		}
	}
	return n
}

// hasPendingException says whether the team has an exception offer out for
// a player other than the given one.
func (lg *League) hasPendingException(teamID, exceptPlayer int) bool {
	for _, o := range lg.FA.Offers {
		if o.Team == teamID && o.PlayerID != exceptPlayer && o.Kind == SignException {
			return true
		}
	}
	return false
}

// offerKind decides how an offer would be signed and checks the money rules:
// the rights team's own player (SignOwn), else cap room (or the minimum),
// else the mid-level exception if the team has one free.
func (lg *League) offerKind(teamID, playerID, salary, years int) (SignKind, error) {
	if rt, ok := lg.RightsTeam(playerID); ok && rt == teamID {
		if err := lg.CanSign(teamID, salary, SignOwn); err != nil {
			return 0, err
		}
		return SignOwn, nil
	}
	err := lg.checkOfferMoney(teamID, playerID, salary)
	if err == nil {
		if salary == Cap.MinSalary {
			return SignMinimum, nil
		}
		return SignCapRoom, nil
	}
	if salary == Cap.MinSalary || lg.hasPendingException(teamID, playerID) {
		return 0, err
	}
	exc := lg.CanSign(teamID, salary, SignException)
	if exc == nil {
		exc = lg.exceptionProblem(teamID, salary, years)
	}
	if exc == nil {
		return SignException, nil
	}
	if info := lg.ExceptionFor(teamID); info.Kind != "none" || info.Used {
		return 0, fmt.Errorf("%v %v", err, exc)
	}
	return 0, err
}

// checkOfferMoney applies the cap-room rules to a possible offer.
func (lg *League) checkOfferMoney(teamID, playerID, salary int) error {
	if salary == Cap.MinSalary {
		return lg.CanSign(teamID, salary, SignMinimum)
	}
	if err := lg.CanSign(teamID, salary, SignCapRoom); err != nil {
		return err
	}
	if pend := lg.pendingAbove(teamID, playerID); pend > 0 {
		if lg.Payroll(teamID)+pend+salary > Cap.SalaryCap {
			room := max(0, lg.CapRoom(teamID)-pend)
			return fmt.Errorf("%s have %s in cap room after their other offers; this offer needs %s.", lg.Teams[teamID].FullName(), fmtM(room), fmtM(salary))
		}
	}
	return nil
}

// MakeOffer puts a standing offer to a free agent. It replaces the team's
// earlier offer for the same player. It can be accepted from the next day on.
func (lg *League) MakeOffer(teamID, playerID, salary, years int) (FAOffer, error) {
	return lg.makeOffer(teamID, playerID, salary, years, false)
}

// makeOffer is MakeOffer; allowFull lets a full computer team bid for an
// upgrade (it cuts its worst player if the player signs).
func (lg *League) makeOffer(teamID, playerID, salary, years int, allowFull bool) (FAOffer, error) {
	if lg.FA == nil || lg.FA.Day >= lg.FA.Days {
		return FAOffer{}, errors.New("Free agency is not open.")
	}
	if teamID < 0 || teamID >= len(lg.Teams) {
		return FAOffer{}, errors.New("That is not a valid team.")
	}
	p := lg.faPlayer(playerID)
	if p == nil {
		return FAOffer{}, errors.New("That player is not a free agent.")
	}
	if years < 1 || years > Cap.MaxContractYears {
		return FAOffer{}, fmt.Errorf("A contract must run 1 to %d years.", Cap.MaxContractYears)
	}
	t := lg.Teams[teamID]
	if len(t.Roster) >= RosterSize && !allowFull {
		return FAOffer{}, fmt.Errorf("%s already have a full roster of %d; release someone first.", t.FullName(), RosterSize)
	}
	if sh := lg.sheetFor(playerID); sh != nil && sh.Team == teamID {
		return FAOffer{}, errors.New("That offer is held as an offer sheet and cannot be changed.")
	}
	if minY := lg.MinYearsFor(teamID, playerID); years < minY {
		return FAOffer{}, fmt.Errorf("%s is a restricted free agent; an offer from another team must run at least %d years.", p.Name, minY)
	}
	kind, err := lg.offerKind(teamID, playerID, salary, years)
	if err != nil {
		return FAOffer{}, err
	}
	lg.dropOffer(teamID, playerID)
	lg.FA.NextOfferID++
	o := FAOffer{ID: lg.FA.NextOfferID, Team: teamID, PlayerID: playerID, Salary: salary, Years: years, Day: lg.FA.Day, Kind: kind}
	lg.FA.Offers = append(lg.FA.Offers, o)
	return o, nil
}

func (lg *League) dropOffer(teamID, playerID int) {
	keep := lg.FA.Offers[:0]
	for _, o := range lg.FA.Offers {
		if !(o.Team == teamID && o.PlayerID == playerID) {
			keep = append(keep, o)
		}
	}
	lg.FA.Offers = keep
}

// WithdrawOffer cancels one of the team's offers.
func (lg *League) WithdrawOffer(teamID, offerID int) error {
	if lg.FA == nil {
		return errors.New("Free agency is not open.")
	}
	for i, o := range lg.FA.Offers {
		if o.ID == offerID && o.Team == teamID {
			if sh := lg.sheetFor(o.PlayerID); sh != nil && sh.Team == teamID {
				return errors.New("That offer is held as an offer sheet and cannot be withdrawn.")
			}
			lg.FA.Offers = append(lg.FA.Offers[:i], lg.FA.Offers[i+1:]...)
			return nil
		}
	}
	return errors.New("That offer does not exist.")
}

// OffersBy lists the team's outstanding offers.
func (lg *League) OffersBy(teamID int) []FAOffer {
	var out []FAOffer
	if lg.FA == nil {
		return out
	}
	for _, o := range lg.FA.Offers {
		if o.Team == teamID {
			out = append(out, o)
		}
	}
	return out
}

// OffersFor counts the offers out for a player (no details: rival bids stay hidden).
func (lg *League) OffersFor(playerID int) int {
	n := 0
	if lg.FA == nil {
		return 0
	}
	for _, o := range lg.FA.Offers {
		if o.PlayerID == playerID {
			n++
		}
	}
	return n
}

// sortedPool returns the free agents best overall first (ties by ID).
func (lg *League) sortedPool() []*Player {
	ps := append([]*Player(nil), lg.FreeAgents...)
	sort.SliceStable(ps, func(i, j int) bool {
		if ps[i].Overall() != ps[j].Overall() {
			return ps[i].Overall() > ps[j].Overall()
		}
		return ps[i].ID < ps[j].ID
	})
	return ps
}

// AdvanceFreeAgency resolves one day and returns that day's signings. Computer
// teams bid first, then players choose among offers made on earlier days.
// After the last day it returns an error. Screens must check
// CanAdvanceFreeAgency first, which refuses while an offer sheet against the
// user waits; called anyway, the day still resolves and the sheet's player
// simply stays out of the market until it is answered (FinishOffseason
// answers it with the computer-team rule).
func (lg *League) AdvanceFreeAgency() ([]FASigning, error) {
	return lg.advanceDay(false, false)
}

// advanceDay is AdvanceFreeAgency; auto answers any offer sheet against the
// user at once (for paths with no screens): declined, or with the
// computer-team match rule if matchUser.
func (lg *League) advanceDay(auto, matchUser bool) ([]FASigning, error) {
	fa := lg.FA
	if fa == nil {
		return nil, errors.New("Free agency is not open.")
	}
	if fa.Day >= fa.Days {
		return nil, errors.New("Free agency is over; start the new season.")
	}
	lg.gmAutoFreeAgency() // assistant GM: auto offers (gm.go)
	cf := lg.contendFactors()
	lg.computerOffers(cf)
	signed := lg.playersDecide(cf, false)
	fa.Day++
	if fa.Day >= fa.Days {
		// Period over: players with a real offer on the table take the best one.
		signed = append(signed, lg.playersDecide(cf, true)...)
	}
	if auto {
		signed = append(signed, lg.autoAnswerSheets(cf, matchUser)...)
	}
	return signed, nil
}

// sheetFor is the pending offer sheet for a player, or nil.
func (lg *League) sheetFor(playerID int) *OfferSheet {
	if lg.FA == nil {
		return nil
	}
	for i := range lg.FA.Sheets {
		if lg.FA.Sheets[i].PlayerID == playerID {
			return &lg.FA.Sheets[i]
		}
	}
	return nil
}

// PendingSheets lists the offer sheets waiting for the user's answer.
func (lg *League) PendingSheets() []OfferSheet {
	if lg.FA == nil {
		return nil
	}
	return append([]OfferSheet(nil), lg.FA.Sheets...)
}

// sheetBlock is the refusal while an offer sheet waits (nil if none).
func (lg *League) sheetBlock() error {
	if lg.FA == nil || len(lg.FA.Sheets) == 0 {
		return nil
	}
	name := "a player"
	for _, p := range lg.FreeAgents {
		if p.ID == lg.FA.Sheets[0].PlayerID {
			name = p.Name
		}
	}
	return fmt.Errorf("Answer the offer sheet for %s first.", name)
}

// sheetSpots counts the roster spots the team's pending offer sheets hold.
func (lg *League) sheetSpots(teamID, exceptPlayer int) int {
	n := 0
	if lg.FA != nil {
		for _, sh := range lg.FA.Sheets {
			if sh.Team == teamID && sh.PlayerID != exceptPlayer {
				n++
			}
		}
	}
	return n
}

// removeSheet deletes the player's pending sheet.
func (lg *League) removeSheet(playerID int) {
	keep := lg.FA.Sheets[:0]
	for _, sh := range lg.FA.Sheets {
		if sh.PlayerID != playerID {
			keep = append(keep, sh)
		}
	}
	lg.FA.Sheets = keep
}

// offerVia names how a signing was made, for the log.
func offerVia(kind SignKind, salary int) string {
	switch {
	case kind == SignOwn:
		return "own"
	case kind == SignException:
		return "exception"
	case kind == SignMinimum || salary == Cap.MinSalary:
		return "minimum"
	}
	return "room"
}

// offerSignKind is the kind an offer is signed under.
func offerSignKind(o FAOffer) SignKind {
	if o.Kind == SignCapRoom && o.Salary == Cap.MinSalary {
		return SignMinimum
	}
	return o.Kind
}

// signWithCut signs the player with the team. A full computer team signing a
// clear upgrade first cuts its worst player (undone if the signing fails).
func (lg *League) signWithCut(teamID int, p *Player, salary, years int, kind SignKind) error {
	var cut *Player
	var cutContract Contract
	var cutDead []DeadContract
	if tm := lg.Teams[teamID]; len(tm.Roster) >= RosterSize && teamID != lg.UserTeamID {
		if w := lg.worstKeep(tm); keepScore(p) >= keepScore(w)+FATuning.UpgradeMargin {
			cut, cutContract, cutDead = w, w.Contract, tm.DeadMoney
			for i, q := range tm.Roster {
				if q == w {
					tm.Roster = append(tm.Roster[:i], tm.Roster[i+1:]...)
					break
				}
			}
			lg.releaseToFA(w)
		}
	}
	if err := lg.Sign(teamID, p, salary, years, kind); err != nil {
		if cut != nil { // undo the cut
			lg.FreeAgents = lg.FreeAgents[:len(lg.FreeAgents)-1]
			cut.Contract, cut.TeamID = cutContract, teamID
			lg.Teams[teamID].DeadMoney = cutDead
			lg.Teams[teamID].Roster = append(lg.Teams[teamID].Roster, cut)
		}
		return err
	}
	return nil
}

// offerFeasible checks, without signing, that the offer could be signed now.
func (lg *League) offerFeasible(p *Player, o FAOffer) error {
	tm := lg.Teams[o.Team]
	if len(tm.Roster) >= RosterSize {
		if o.Team == lg.UserTeamID || keepScore(p) < keepScore(lg.worstKeep(tm))+FATuning.UpgradeMargin {
			return fmt.Errorf("%s already have a full roster of %d.", tm.FullName(), RosterSize)
		}
	}
	kind := offerSignKind(o)
	if err := lg.CanSign(o.Team, o.Salary, kind); err != nil {
		return err
	}
	if kind == SignException {
		return lg.exceptionProblem(o.Team, o.Salary, o.Years)
	}
	return nil
}

// dropPlayerOffers removes every offer for the player and his rights entry.
func (lg *League) dropPlayerOffers(playerID int) {
	keep := lg.FA.Offers[:0]
	for _, x := range lg.FA.Offers {
		if x.PlayerID != playerID {
			keep = append(keep, x)
		}
	}
	lg.FA.Offers = keep
	delete(lg.FA.Rights, playerID)
}

// wouldMatch is the computer-team rule for matching an offer sheet: the
// player is worth keeping (the re-signing test), the sheet keeps its payroll
// at or under the tax line, the contract is legal for it and it has (or, for a
// computer team, can make) a roster spot. The user's team never cuts anyone.
func (lg *League) wouldMatch(rights int, p *Player, salary, years int, cf []float64) bool {
	if lg.value(p, rights, cf) < Cap.ReSignMinValue {
		return false
	}
	if lg.Payroll(rights)+salary > Cap.TaxLine || lg.CanSign(rights, salary, SignOwn) != nil {
		return false
	}
	tm := lg.Teams[rights]
	if len(tm.Roster) >= RosterSize {
		return rights != lg.UserTeamID && keepScore(p) >= keepScore(lg.worstKeep(tm))+FATuning.UpgradeMargin
	}
	return true
}

// logSigning records a signing in the free-agency log.
func (lg *League) logSigning(day int, p *Player, team, salary, years int, via string, sheetFrom int, matched bool) FASigning {
	g := FASigning{Day: day, PlayerID: p.ID, Name: p.Name, Team: team, Salary: salary, Years: years,
		Pos: p.Pos.String(), Overall: p.Overall(), Via: via}
	if matched {
		g.Matched, g.SheetFrom = true, sheetFrom
	}
	lg.FA.Log = append(lg.FA.Log, g)
	return g
}

// AnswerSheet answers a pending offer sheet against the user's rights: match
// signs the player with the user at the sheet's terms (over the cap is fine,
// the hard ceiling and roster size still apply); otherwise he signs with the
// offering team. If that team can no longer sign him the sheet is void and he
// returns to the pool (the returned signing is then empty, Name == "").
func (lg *League) AnswerSheet(playerID int, match bool) (FASigning, error) {
	sh := lg.sheetFor(playerID)
	if sh == nil {
		return FASigning{}, errors.New("There is no offer sheet for that player.")
	}
	p := lg.faPlayer(playerID)
	if p == nil {
		lg.removeSheet(playerID)
		return FASigning{}, errors.New("That player is not a free agent.")
	}
	s := *sh
	u := lg.UserTeamID
	if match {
		if len(lg.Teams[u].Roster) >= RosterSize {
			return FASigning{}, fmt.Errorf("Your roster is full (%d); release someone first to match the offer for %s.", RosterSize, p.Name)
		}
		if err := lg.Sign(u, p, s.Salary, s.Years, SignOwn); err != nil {
			return FASigning{}, err
		}
		lg.removeSheet(playerID)
		lg.dropPlayerOffers(playerID)
		return lg.logSigning(s.Day, p, u, s.Salary, s.Years, "own", s.Team, true), nil
	}
	var offer *FAOffer
	for i := range lg.FA.Offers {
		if o := &lg.FA.Offers[i]; o.PlayerID == playerID && o.Team == s.Team {
			offer = o
		}
	}
	lg.removeSheet(playerID)
	if offer == nil {
		return FASigning{}, nil
	}
	o := *offer
	if err := lg.signWithCut(o.Team, p, o.Salary, o.Years, offerSignKind(o)); err != nil {
		lg.dropOffer(o.Team, playerID) // sheet void, he is back in the pool with his rights kept
		return FASigning{}, nil
	}
	lg.dropPlayerOffers(playerID)
	return lg.logSigning(s.Day, p, o.Team, o.Salary, o.Years, offerVia(offerSignKind(o), o.Salary), 0, false), nil
}

// autoAnswerSheets answers every pending sheet: declined, or (matchUser) by
// the computer-team match rule.
func (lg *League) autoAnswerSheets(cf []float64, matchUser bool) []FASigning {
	var out []FASigning
	for len(lg.FA.Sheets) > 0 {
		sh := lg.FA.Sheets[0]
		match := false
		if p := lg.faPlayer(sh.PlayerID); p != nil {
			match = matchUser && lg.wouldMatch(lg.UserTeamID, p, sh.Salary, sh.Years, cf)
		}
		g, err := lg.AnswerSheet(sh.PlayerID, match)
		if err != nil {
			lg.removeSheet(sh.PlayerID)
			continue
		}
		if g.Name != "" {
			out = append(out, g)
		}
	}
	return out
}

// computerOffers lets each computer team with open roster spots bid.
func (lg *League) computerOffers(cf []float64) {
	fa := lg.FA
	r := lg.dayRNG(lg.Year, freeAgencyOffset+fa.Day)
	pool := lg.sortedPool()
	if len(pool) == 0 {
		return
	}
	soft := soften(fa.Day, fa.Days)
	for _, t := range lg.Teams {
		if t.ID == lg.UserTeamID {
			continue
		}
		spots := RosterSize - len(t.Roster)
		upgrade := spots <= 0 // full roster: only a clear upgrade is worth a bid
		limit := spots + 1    // one backup offer in case a player turns it down or signs elsewhere
		worst := 0.0
		if upgrade {
			limit = 1
			worst = keepScore(lg.worstKeep(t))
		}
		if len(lg.OffersBy(t.ID)) >= limit {
			continue
		}
		type cand struct {
			p *Player
			v float64
		}
		cands := make([]cand, 0, len(pool))
		for _, p := range pool {
			cands = append(cands, cand{p, lg.value(p, t.ID, cf)})
		}
		sort.SliceStable(cands, func(i, j int) bool {
			if cands[i].v != cands[j].v {
				return cands[i].v > cands[j].v
			}
			return cands[i].p.ID < cands[j].p.ID
		})
		if len(cands) > FATuning.CandidatesPerTm {
			cands = cands[:FATuning.CandidatesPerTm]
		}
		want := 1
		if FATuning.MaxNewOffers > 1 && r.Chance(FATuning.SecondOfferPct) {
			want = FATuning.MaxNewOffers
		}
		made := 0
		for _, c := range cands {
			if made >= want || len(lg.OffersBy(t.ID)) >= limit {
				break
			}
			if c.v < FATuning.WantMinValue || (upgrade && keepScore(c.p) < worst+FATuning.UpgradeMargin) {
				continue
			}
			already := false
			for _, o := range lg.OffersBy(t.ID) {
				if o.PlayerID == c.p.ID {
					already = true
				}
			}
			if already {
				continue
			}
			ask, years := AskingContract(c.p)
			sal := int(math.Round(float64(ask)*soft*(FATuning.OfferLow+r.Float64()*(FATuning.OfferHigh-FATuning.OfferLow))/100)) * 100
			sal = max(Cap.MinSalary, min(Cap.MaxSalary, sal))
			years = clamp(years+r.Between(-1, 1)*b2i(r.Chance(0.3)), 1, Cap.MaxContractYears)
			years = max(years, lg.MinYearsFor(t.ID, c.p.ID))
			if rt, ok := lg.RightsTeam(c.p.ID); ok && rt == t.ID && lg.Payroll(t.ID)+lg.pendingOutsideRoom(t.ID, c.p.ID)+sal > Cap.TaxLine {
				continue // keeping his own restricted player must not take it over the tax line
			}
			if !lg.computerOfferOK(t, c.p, sal, years, upgrade) {
				// "Best we can do": offer all the usable cap room if it is a credible share of the ask.
				softAsk := max(Cap.MinSalary, int(math.Round(float64(ask)*soft/100))*100)
				room := (Cap.SalaryCap - lg.Payroll(t.ID) - lg.pendingAbove(t.ID, c.p.ID)) / 100 * 100
				if room <= Cap.MinSalary || float64(room) < FATuning.BestEffortShare*float64(softAsk) {
					continue
				}
				sal = min(sal, room)
				if lg.checkOfferMoney(t.ID, c.p.ID, sal) != nil {
					continue
				}
			}
			if _, err := lg.makeOffer(t.ID, c.p.ID, sal, years, upgrade); err == nil {
				made++
			}
		}
		// Late in the period, fill open spots with minimum offers to players
		// who would take one.
		if made == 0 && !upgrade && fa.Day >= FATuning.MinOffersAfter && len(lg.OffersBy(t.ID)) < limit {
			for _, c := range cands {
				if c.v < 1 {
					continue
				}
				ask, _ := AskingContract(c.p)
				if float64(Cap.MinSalary) < FATuning.AcceptPct*math.Max(float64(Cap.MinSalary), float64(ask)*soft) {
					continue
				}
				if lg.OffersBy(t.ID) != nil && hasOffer(lg.OffersBy(t.ID), c.p.ID) {
					continue
				}
				if _, err := lg.MakeOffer(t.ID, c.p.ID, Cap.MinSalary, 1); err == nil {
					break
				}
			}
		}
	}
}

// computerOfferOK is the money check for a computer offer: the usual rules,
// plus its habits: the exception only for an open roster spot, and never
// taking payroll over the tax line by exception or by keeping its own player.
func (lg *League) computerOfferOK(t *Team, p *Player, sal, years int, upgrade bool) bool {
	kind, err := lg.offerKind(t.ID, p.ID, sal, years)
	if err != nil {
		return false
	}
	switch kind {
	case SignException:
		if upgrade {
			return false
		}
		fallthrough
	case SignOwn:
		return lg.Payroll(t.ID)+lg.pendingOutsideRoom(t.ID, p.ID)+sal <= Cap.TaxLine
	}
	return true
}

func hasOffer(os []FAOffer, pid int) bool {
	for _, o := range os {
		if o.PlayerID == pid {
			return true
		}
	}
	return false
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// playersDecide lets every free agent with an offer made before today pick
// the most appealing acceptable one. A restricted player picking another
// team's offer makes it an offer sheet: a computer rights team matches it or
// not at once; if the user holds the rights, a pending sheet is created.
func (lg *League) playersDecide(cf []float64, final bool) []FASigning {
	fa := lg.FA
	var out []FASigning
	for _, p := range lg.sortedPool() {
		if lg.sheetFor(p.ID) != nil {
			continue
		}
		var offs []FAOffer
		for _, o := range fa.Offers {
			if o.PlayerID == p.ID && (o.Day < fa.Day || final) {
				offs = append(offs, o)
			}
		}
		if len(offs) == 0 {
			continue
		}
		ask, _ := lg.FreeAgentAsk(p)
		need := FATuning.AcceptPct * float64(ask)
		orig, _ := AskingContract(p)
		if fa.Day >= fa.Days-FATuning.LateDays {
			need = math.Min(need, FATuning.LateShare*float64(orig))
		}
		if final {
			if orig < FATuning.SettleAskMult*Cap.MinSalary {
				continue
			}
			need = float64(Cap.MinSalary + 1)
		}
		mot := lg.MotivationOf(p)
		mw := &MotivationTuning.FAWeight
		appeal := func(o FAOffer) float64 {
			// Motivation scales how far each part moves the base salary:
			// Money the salary and years part, Winning the team part, Role the
			// top-eight part, Loyalty a bonus for his previous team.
			a := math.Pow(float64(o.Salary)/float64(max(1, orig)), mw[mot.Money]) * (1 + FATuning.YearBonus*float64(o.Years-1))
			a *= 1 - FATuning.TeamSwing*mw[mot.Winning] + 2*FATuning.TeamSwing*mw[mot.Winning]*cf[o.Team]
			better := 0
			for _, q := range lg.Teams[o.Team].Roster {
				if q.Overall() > p.Overall() {
					better++
				}
			}
			if better < 8 {
				a *= 1 + FATuning.RoleBonus*mw[mot.Role]
			}
			if prev, ok := fa.Prev[p.ID]; ok && prev == o.Team {
				a *= 1 + MotivationTuning.LoyaltyBonus[mot.Loyalty]
			}
			return a
		}
		sort.SliceStable(offs, func(i, j int) bool {
			ai, aj := appeal(offs[i]), appeal(offs[j])
			if ai != aj {
				return ai > aj
			}
			return offs[i].Team < offs[j].Team
		})
		for _, o := range offs {
			if float64(o.Salary) < need {
				continue
			}
			day := min(fa.Day, fa.Days-1)
			kind := offerSignKind(o)
			tm := lg.Teams[o.Team]
			// A roster spot held for a pending offer sheet is not free for others.
			if len(tm.Roster) < RosterSize && len(tm.Roster)+1+lg.sheetSpots(o.Team, p.ID) > RosterSize {
				continue
			}
			rt, restricted := lg.RightsTeam(p.ID)
			if restricted && rt != o.Team {
				if err := lg.offerFeasible(p, o); err != nil {
					lg.dropOffer(o.Team, p.ID)
					continue
				}
				if rt == lg.UserTeamID {
					fa.Sheets = append(fa.Sheets, OfferSheet{PlayerID: p.ID, Team: o.Team, Salary: o.Salary, Years: o.Years, Day: day})
					break
				}
				if lg.wouldMatch(rt, p, o.Salary, o.Years, cf) && lg.signWithCut(rt, p, o.Salary, o.Years, SignOwn) == nil {
					lg.dropPlayerOffers(p.ID)
					out = append(out, lg.logSigning(day, p, rt, o.Salary, o.Years, "own", o.Team, true))
					break
				}
			}
			if err := lg.signWithCut(o.Team, p, o.Salary, o.Years, kind); err != nil {
				lg.dropOffer(o.Team, p.ID)
				continue
			}
			lg.dropPlayerOffers(p.ID)
			out = append(out, lg.logSigning(day, p, o.Team, o.Salary, o.Years, offerVia(kind, o.Salary), 0, false))
			break
		}
	}
	return out
}

// worstKeep is the team's player with the lowest keep score (ties: higher ID).
func (lg *League) worstKeep(t *Team) *Player {
	var w *Player
	for _, p := range t.Roster {
		if w == nil || keepScore(p) < keepScore(w) || (keepScore(p) == keepScore(w) && p.ID > w.ID) {
			w = p
		}
	}
	return w
}

// fillRosters is the end-of-period fill: every team short of 13 signs the best
// remaining free agents to one-year minimum deals (users included).
func (lg *League) fillRosters(order []int) {
	if len(order) != len(lg.Teams) {
		order = order[:0]
		for i := range lg.Teams {
			order = append(order, i)
		}
	}
	for _, tid := range order {
		t := lg.Teams[tid]
		for len(t.Roster) < RosterSize && len(lg.FreeAgents) > 0 {
			p := lg.sortedPool()[0]
			if lg.Sign(tid, p, Cap.MinSalary, 1, SignMinimum) != nil {
				lg.forceSign(tid, p, Cap.MinSalary, 1)
			}
		}
	}
}
