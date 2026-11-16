package engine

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// Owner goals and team finances (the user's team only).
//
// The owner OBSERVES the game: nothing in this file draws a game random
// number, and no computer team or game result depends on it. It reads only
// visible information (never Player.Impact, TrueOverall or TrueCeiling).
//
// Saved: League.Owner (nil = not started; created for a new league and
// lazily at the next StartSeason for an old save). Derived and never saved:
// each team's owner name, personality and market size (OwnerOf, hashed from
// the seed and team ID on stream ownerOffset).

// ownerOffset is the hashed stream for owner names, personalities and market
// sizes (RULES.md section 3). Nothing else uses it.
const ownerOffset = 140000

// ownerOff is a test hook: when true no owner state is ever created, so a
// league behaves exactly as it did before this feature existed.
var ownerOff bool

// Personalities (the keys go out in the API).
const (
	OwnerWinNow  = "win_now"
	OwnerPatient = "patient"
	OwnerCheap   = "cheap"
)

// Market sizes (the keys go out in the API).
const (
	MarketSmall  = "small"
	MarketMedium = "medium"
	MarketLarge  = "large"
)

// OwnerTuning holds every number of the owner feature. Money is in thousands
// of dollars, scaled so a medium-market team at the salary cap with 41 wins
// roughly breaks even.
var OwnerTuning = struct {
	StartTrust int // trust of a new owner (0..100)

	// Revenue = MarketBase[size] + PerHomeWin * home wins + PerPlayoffHomeGame * playoff home games.
	MarketBase         [3]int // small, medium, large
	PerHomeWin         int
	PerPlayoffHomeGame int
	NeutralHomeWins    int // home wins assumed for a first-season budget (no last season yet)
	HomeGames          int // regular-season home games (revenue is projected to this)

	// Budget = revenue basis times the personality's factor, never below
	// BudgetFloor; cheap and patient never above the tax line; win-now may go
	// WinNowTaxRoom above it when the roster is a contender or buyer.
	BudgetFloor         int
	CheapFactor         float64
	PatientFactor       float64
	WinNowFactor        float64
	WinNowTaxRoom       int
	AssistantOverBudget int // the assistant GM may plan this much above the budget (0 = not at all)

	// Goals.
	ExpWinsTop     float64 // wins a rank-1 roster usually gets
	ExpWinsStep    float64 // wins lost per rank
	WinsSlack      int     // a wins goal is this many below the usual for the rank
	MinWinsGoal    int
	ConfFinalsRank int    // rank at or better: asked for the conference finals
	RoundRank      int    // rank at or better: asked to win a playoff round
	PlayoffsRank   int    // rank at or better: asked to make the playoffs
	ImproveGain    int    // overall a young player must gain over the season
	YoungAge       int    // "young" for the improve goal
	RebuildAge     int    // young for the rebuild goal
	PaceWins       [3]int // pace line (wins) for playoffs, round, conf finals goals

	// Trust.
	GoalGain        int     // trust per weight point for a met goal
	GoalLoss        int     // trust per weight point for a missed goal
	PlayoffBonus    int     // per playoff round won beyond the results goal
	WinsPivot       int     // wins that move trust neither way
	WinsTrustUp     float64 // trust per win above the pivot, times the personality speed
	WinsTrustDown   float64 // trust per win below the pivot, times the personality speed
	SpeedWinNow     float64 // how fast trust moves per personality
	SpeedPatient    float64
	SpeedCheap      float64
	OverWeekPenalty int // trust lost each week over budget
	PaceDay         int // games played before the pace warning
	PaceSlack       int // wins below the line the pace may project before the warning
	PacePenalty     int
	LosingWins      int    // a season with fewer wins than this is a losing season
	LosingStep      int    // trust lost per losing season in a row, times the streak and the speed (8, 16, 24...)
	LosingStreakMax int    // the streak stops counting here
	MaxSeasonDrop   int    // most trust one season can cost, from its start to its end
	RebuildGrace    [3]int // losing seasons forgiven when a real rebuild met its building goal: win_now, patient, cheap
	ProgressWins    int    // after a losing season the wins goal is at least last season's wins plus this
	MaxWinsGoal     int
	SpendFloor      float64 // payroll (share of the cap) below which the money goal asks for a real team
	SpendFloorCheap float64 // the same for a cheap owner
	ProspectOvr     int     // a "real prospect" is this overall or better...
	ProspectPot     int     // ...or has this potential or more
	ProspectTop     int     // and is among the user's this many best players
	BadSeasons      int     // results goals missed in a row...
	BadSeasonsTrust int     // ...with trust below this: fired

	LedgerKeep  int
	MessageKeep int
	JobCount    int
}{
	StartTrust: 60,

	MarketBase:         [3]int{105000, 120000, 140000},
	PerHomeWin:         1000,
	PerPlayoffHomeGame: 2500,
	NeutralHomeWins:    20,
	HomeGames:          41,

	BudgetFloor:         140000, // never below the salary cap: a team at the cap is never "over budget"
	CheapFactor:         0.90,
	PatientFactor:       0.95,
	WinNowFactor:        1.02,
	WinNowTaxRoom:       12000,
	AssistantOverBudget: 0,

	ExpWinsTop: 56, ExpWinsStep: 1.05, WinsSlack: 3, MinWinsGoal: 29,
	ConfFinalsRank: 3, RoundRank: 8, PlayoffsRank: 14,
	ImproveGain: 2, YoungAge: 24, RebuildAge: 23,
	PaceWins: [3]int{36, 44, 50},

	GoalGain: 3, GoalLoss: 4, PlayoffBonus: 3, WinsPivot: 38, WinsTrustUp: 0.5, WinsTrustDown: 0.6,
	SpeedWinNow: 1.4, SpeedPatient: 0.8, SpeedCheap: 1.1,
	OverWeekPenalty: 1, PaceDay: 30, PaceSlack: 5, PacePenalty: 2,
	LosingWins: 32, LosingStep: 6, LosingStreakMax: 5, MaxSeasonDrop: 30, RebuildGrace: [3]int{1, 2, 1}, ProgressWins: 4, MaxWinsGoal: 48,
	SpendFloor: 0.88, SpendFloorCheap: 0.72, ProspectOvr: 62, ProspectPot: 74, ProspectTop: 10,
	BadSeasons: 3, BadSeasonsTrust: 25,

	LedgerKeep: 30, MessageKeep: 60, JobCount: 3,
}

// ownerWeights are the goal weights (results, money, building) per personality.
var ownerWeights = map[string][3]int{
	OwnerWinNow:  {3, 1, 2},
	OwnerCheap:   {2, 3, 1},
	OwnerPatient: {2, 1, 3},
}

// OwnerGoal is one thing the owner asks for this season.
type OwnerGoal struct {
	Key    string // wins, playoffs, round, conf_finals, budget, tax, profit, improve, keep_star, rebuild
	Kind   string // results, money, building
	Title  string
	Detail string
	Weight int // 1..3
	Target int // wins line, overall gain, or player ID (keep_star)
	Met    bool
	Done   bool
}

// OwnerGoalResult is one goal's outcome in a ledger row.
type OwnerGoalResult struct {
	Title string
	Met   bool
}

// OwnerSeason is one finished season in the ledger.
type OwnerSeason struct {
	Year, Wins, Losses int
	Playoffs           string
	Revenue, Costs     int
	Profit             int
	TrustStart         int
	TrustEnd           int
	Goals              []OwnerGoalResult
}

// OwnerMessage is one line the owner said. Tone is good, warn, bad or info.
type OwnerMessage struct {
	Year, Day int
	Text      string
	Tone      string
}

// OwnerState is the saved owner state of the user's team.
type OwnerState struct {
	Trust      int // 0..100
	LastChange int // trust moved since the season began (the whole season once it is judged)
	Goals      []OwnerGoal
	Budget     int // payroll budget for this season, thousands
	Ledger     []OwnerSeason
	Messages   []OwnerMessage // newest last
	Fired      bool
	FiredText  string
	BadSeasons int // seasons in a row with the results goal missed
	OverWeeks  int // weeks over budget this season

	SeasonStartTrust int         `json:",omitempty"`
	PaceWarned       bool        `json:",omitempty"`
	OverNoted        bool        `json:",omitempty"`
	StartOvr         map[int]int `json:",omitempty"` // overall of each player 24 or under at the season start
	StartDir         string      `json:",omitempty"` // the team's direction at the season start (TeamStatus key)
	LosingStreak     int         `json:",omitempty"` // losing seasons in a row (a better season takes one off)
	GraceUsed        int         `json:",omitempty"` // losing seasons forgiven for a real rebuild
	PrevWins         int         `json:",omitempty"` // wins of the season before (0 = none)
	PrevLosing       bool        `json:",omitempty"`
}

// OwnerInfo is the derived (never saved) description of a team's owner.
type OwnerInfo struct {
	Name        string
	Personality string // OwnerWinNow, OwnerPatient or OwnerCheap
	Market      string // MarketSmall, MarketMedium or MarketLarge
}

// OwnerPersonalities lists key, label and blurb of each personality.
func OwnerPersonalities() [][3]string {
	return [][3]string{
		{OwnerWinNow, "Win-now", "Wants results this year and will spend for them. Trust moves fast, and results matter most."},
		{OwnerPatient, "Patient", "Thinks long term and forgives a slow start. Trust moves slowly, and building matters most."},
		{OwnerCheap, "Cheap", "Watches every dollar. Money matters most, and he hates the luxury tax."},
	}
}

// OwnerPersonalityText is the label and blurb for a personality key.
func OwnerPersonalityText(key string) (label, blurb string) {
	for _, p := range OwnerPersonalities() {
		if p[0] == key {
			return p[1], p[2]
		}
	}
	return "", ""
}

// OwnerMarketLabel is the display label of a market key.
func OwnerMarketLabel(key string) string {
	switch key {
	case MarketSmall:
		return "Small market"
	case MarketMedium:
		return "Medium market"
	case MarketLarge:
		return "Large market"
	}
	return ""
}

func marketIndex(key string) int {
	switch key {
	case MarketSmall:
		return 0
	case MarketLarge:
		return 2
	}
	return 1
}

// OwnerOf is a team's owner: a fictional name, a personality and the market
// size, all hashed from the seed and the team ID (stream ownerOffset). Market
// sizes are spread over the 30 teams as 10 small, 12 medium and 8 large.
func (lg *League) OwnerOf(teamID int) OwnerInfo {
	h := func(team, k int) uint64 { return mix64(lg.Seed, ownerOffset, uint64(team), uint64(k)) }
	// Market: rank the teams by a hash; the lowest ten are small, the top eight large.
	ids := make([]int, len(lg.Teams))
	for i := range ids {
		ids[i] = i
	}
	sort.SliceStable(ids, func(a, b int) bool { return h(ids[a], 1) < h(ids[b], 1) })
	market := MarketMedium
	for pos, id := range ids {
		if id != teamID {
			continue
		}
		switch {
		case pos < 10:
			market = MarketSmall
		case pos >= 22:
			market = MarketLarge
		}
	}
	pers := []string{OwnerWinNow, OwnerPatient, OwnerCheap}[h(teamID, 2)%3]
	name := firstNames[h(teamID, 3)%uint64(len(firstNames))] + " " + lastNames[h(teamID, 4)%uint64(len(lastNames))]
	return OwnerInfo{Name: name, Personality: pers, Market: market}
}

// OwnerTrustLabel names a trust score.
func OwnerTrustLabel(score int) string {
	switch {
	case score >= 75:
		return "Secure"
	case score >= 50:
		return "Steady"
	case score >= 25:
		return "Uneasy"
	}
	return "Hot seat"
}

// OwnerTrend is "up", "flat" or "down" for the trust change this season.
func OwnerTrend(change int) string {
	switch {
	case change >= 3:
		return "up"
	case change <= -3:
		return "down"
	}
	return "flat"
}

func newOwnerState() *OwnerState {
	return &OwnerState{Trust: OwnerTuning.StartTrust}
}

// ---- money ----

// OwnerMoney is the user's season in money (thousands).
type OwnerMoney struct {
	Market, Wins, Playoffs, Revenue int
	Salaries, DeadMoney, Tax, Costs int
	Profit                          int
	Projected                       bool // true while the regular season is not over (revenue is extrapolated)
}

// ownerMoneyOf is team's season s in money: revenue from the games played so
// far (regular-season home wins extrapolated to a full schedule while the
// season is on), costs from the team's payroll right now.
func (lg *League) ownerMoneyOf(team int, s *Season) OwnerMoney {
	t := &OwnerTuning
	var m OwnerMoney
	m.Market = t.MarketBase[marketIndex(lg.OwnerOf(team).Market)]
	homeWins := t.NeutralHomeWins
	if s != nil {
		hg, hw := 0, 0
		for i := range s.Games {
			g := &s.Games[i]
			if g.Played && g.Home == team {
				hg++
				if g.HomeScore > g.AwayScore {
					hw++
				}
			}
		}
		switch {
		case s.Done():
			homeWins = hw
		case hg > 0:
			homeWins = int(math.Round(float64(hw) * float64(t.HomeGames) / float64(hg)))
			m.Projected = true
		default:
			m.Projected = true
		}
		po := 0
		for _, sr := range s.Series {
			for _, g := range sr.Games {
				if g.Home == team {
					po++
				}
			}
		}
		m.Playoffs = po * t.PerPlayoffHomeGame
	} else {
		m.Projected = true
	}
	m.Wins = homeWins * t.PerHomeWin
	m.Revenue = m.Market + m.Wins + m.Playoffs
	dead := 0
	for _, d := range lg.Teams[team].DeadMoney {
		dead += d.Salary
	}
	m.DeadMoney = dead
	m.Salaries = lg.Payroll(team) - dead
	m.Tax = lg.TaxBill(team)
	m.Costs = m.Salaries + m.DeadMoney + m.Tax
	m.Profit = m.Revenue - m.Costs
	return m
}

// OwnerFinances is the user's season so far, projected to a full schedule.
func (lg *League) OwnerFinances() OwnerMoney { return lg.ownerMoneyOf(lg.UserTeamID, lg.Season) }

// ownerBudgetFor is the payroll budget the owner of team sets from last
// season's revenue (lastRev <= 0: no last season yet, use the market base
// plus a typical number of home wins).
func (lg *League) ownerBudgetFor(team, lastRev int) int {
	t := &OwnerTuning
	info := lg.OwnerOf(team)
	base := float64(lastRev)
	if lastRev <= 0 {
		base = float64(t.MarketBase[marketIndex(info.Market)] + t.NeutralHomeWins*t.PerHomeWin)
	}
	b := 0
	switch info.Personality {
	case OwnerCheap:
		b = min(Cap.TaxLine, int(base*t.CheapFactor))
	case OwnerWinNow:
		limit := Cap.TaxLine
		if key := lg.TeamStatusOf(team).Key; key == "contender" || key == "buyer" {
			limit = min(Cap.HardCeiling, Cap.TaxLine+t.WinNowTaxRoom)
		}
		b = min(limit, int(base*t.WinNowFactor))
	default:
		b = min(Cap.TaxLine, int(base*t.PatientFactor))
	}
	b = max(b, t.BudgetFloor)
	return b / 100 * 100
}

// ownerBudgetNow is the budget the assistant and the screens should use right
// now: this season's budget, or in the offseason the one the owner will set for
// the coming season from the season just finished.
func (lg *League) ownerBudgetNow() int {
	o := lg.Owner
	if o == nil {
		return 0
	}
	if lg.inOffseason() && lg.Season != nil && lg.Season.Done() {
		rev := lg.ownerMoneyOf(lg.UserTeamID, lg.Season).Revenue
		return lg.ownerBudgetFor(lg.UserTeamID, rev)
	}
	return o.Budget
}

// OwnerBudget is the payroll budget the owner holds the user to right now
// (0 when there is no owner state yet).
func (lg *League) OwnerBudget() int { return lg.ownerBudgetNow() }

// OwnerAlert is the newest warning or bad-news message from the current week
// ("" when there is none).
func (lg *League) OwnerAlert() string {
	o := lg.Owner
	if o == nil || lg.Season == nil {
		return ""
	}
	for i := len(o.Messages) - 1; i >= 0; i-- {
		m := o.Messages[i]
		if m.Year != lg.Year || m.Day < lg.Season.Day-6 {
			break
		}
		if m.Tone == "warn" || m.Tone == "bad" {
			return m.Text
		}
	}
	return ""
}

// ownerCapFor is the payroll the owner lets the assistant GM plan to, and
// whether such a limit applies. A star re-sign may go past it only for a
// win-now owner.
func (lg *League) ownerCapFor(star bool) (int, bool) {
	if lg.Owner == nil {
		return 0, false
	}
	if star && lg.OwnerOf(lg.UserTeamID).Personality == OwnerWinNow {
		return 0, false
	}
	return lg.ownerBudgetNow() + OwnerTuning.AssistantOverBudget, true
}

// ---- guards ----

// ErrOwnerFired is the sentence the offseason guards give while the user is fired.
var ErrOwnerFired = errors.New("You were let go. Choose your next job first.")

// ownerFiredBlock is non-nil while the user waits to choose a new job.
func (lg *League) ownerFiredBlock() error {
	if lg.Owner != nil && lg.Owner.Fired {
		return ErrOwnerFired
	}
	return nil
}

// OwnerFired says whether the user has been let go and must pick a job.
func (lg *League) OwnerFired() bool { return lg.Owner != nil && lg.Owner.Fired }

// ---- season start ----

// ownerPrevRevenue is the user's revenue in the season just finished, or 0
// when there is none (a new league). Call it before StartSeason replaces the season.
func (lg *League) ownerPrevRevenue() int {
	s := lg.Season
	if s == nil || !s.Done() || s.Year >= lg.Year {
		return 0
	}
	return lg.ownerMoneyOf(lg.UserTeamID, s).Revenue
}

// ownerRank is the 1-based rank of the team's top-eight average among all teams.
func (lg *League) ownerRank(team int) int {
	mine := lg.TeamTop8(team)
	rank := 1
	for i := range lg.Teams {
		if i != team && lg.TeamTop8(i) > mine {
			rank++
		}
	}
	return rank
}

func (lg *League) ownerSay(tone, text string) {
	o := lg.Owner
	day := 0
	if lg.Season != nil {
		day = lg.Season.Day
	}
	o.Messages = append(o.Messages, OwnerMessage{Year: lg.Year, Day: day, Text: text, Tone: tone})
	if n := len(o.Messages); n > OwnerTuning.MessageKeep {
		o.Messages = append([]OwnerMessage(nil), o.Messages[n-OwnerTuning.MessageKeep:]...)
	}
}

// ownerMove changes trust, keeping it in 0..100 (at least 1 when firing is off).
func (lg *League) ownerMove(delta int) {
	o := lg.Owner
	lo := 0
	if lg.Options.NoFiring {
		lo = 1
	}
	o.Trust = max(lo, min(100, o.Trust+delta))
	o.LastChange = o.Trust - o.SeasonStartTrust
}

func (lg *League) userYoungIDs(age int) map[int]int {
	out := map[int]int{}
	for _, p := range lg.Teams[lg.UserTeamID].Roster {
		if p.Age <= age {
			out[p.ID] = p.Overall()
		}
	}
	return out
}

// ownerPickGoals chooses this season's three goals from the roster's standing
// (visible information only).
func (lg *League) ownerPickGoals(info OwnerInfo, budget int) []OwnerGoal {
	t := &OwnerTuning
	user := lg.UserTeamID
	rank := lg.ownerRank(user)
	status := lg.TeamStatusOf(user).Key
	w := ownerWeights[info.Personality]
	var goals []OwnerGoal

	// Results.
	expect := int(math.Round(t.ExpWinsTop - float64(rank-1)*t.ExpWinsStep))
	res := OwnerGoal{Kind: "results", Weight: w[0]}
	switch {
	case rank <= t.ConfFinalsRank:
		res.Key, res.Title, res.Target = "conf_finals", "Reach the conference finals", t.PaceWins[2]
	case rank <= t.RoundRank:
		res.Key, res.Title, res.Target = "round", "Win a playoff round", t.PaceWins[1]
	case rank <= t.PlayoffsRank:
		res.Key, res.Title, res.Target = "playoffs", "Make the playoffs", t.PaceWins[0]
	default:
		n := max(t.MinWinsGoal, expect-t.WinsSlack)
		if o := lg.Owner; o != nil && o.PrevLosing && o.PrevWins > 0 {
			n = max(n, min(t.MaxWinsGoal, o.PrevWins+t.ProgressWins)) // owners expect progress
		}
		res.Key, res.Title, res.Target = "wins", fmt.Sprintf("Win at least %d games", n), n
	}
	res.Detail = fmt.Sprintf("Your top eight players rank %s of %d in the league, so this is a fair ask.", ordinal(rank), len(lg.Teams))
	goals = append(goals, res)

	// Money: a stripped payroll must not be a free pass, a taxed one must come down.
	mon := OwnerGoal{Kind: "money", Weight: w[1]}
	pick := "budget"
	pay := lg.Payroll(user)
	floorShare := t.SpendFloor
	if info.Personality == OwnerCheap {
		floorShare = t.SpendFloorCheap
	}
	floor := int(floorShare*float64(Cap.SalaryCap)) / 100 * 100
	switch {
	case pay < floor:
		pick = "spend"
	case pay > Cap.TaxLine:
		pick = "tax"
	case pay > budget:
		pick = "budget"
	case info.Personality == OwnerCheap && info.Market != MarketSmall:
		pick = "profit"
	case info.Personality == OwnerWinNow && budget <= Cap.TaxLine:
		pick = "tax"
	}
	switch pick {
	case "spend":
		mon.Key, mon.Target = "spend", floor
		mon.Title = fmt.Sprintf("Put a real team on the floor: payroll of at least %s by the end of the season", fmtM(floor))
		mon.Detail = fmt.Sprintf("Your payroll is %s. I am not paying for a roster this empty; the budget is %s.", fmtM(pay), fmtM(budget))
	case "profit":
		mon.Key, mon.Title, mon.Detail = "profit", "Do not lose money this season", "Revenue has to cover salaries, dead money and any luxury tax."
	case "tax":
		mon.Key, mon.Title, mon.Detail = "tax", "Stay under the luxury tax", fmt.Sprintf("Keep the payroll at or under the tax line of %s.", fmtM(Cap.TaxLine))
		if pay > Cap.TaxLine {
			mon.Title = "Get under the luxury tax"
		}
	default:
		mon.Key, mon.Title, mon.Detail = "budget", fmt.Sprintf("Finish at or under the budget of %s", fmtM(budget)), "Payroll includes dead money. Each week over budget costs trust."
	}
	goals = append(goals, mon)

	// Building.
	bld := OwnerGoal{Kind: "building", Weight: w[2]}
	young := 0
	for _, p := range lg.Teams[user].Roster {
		if p.Age <= t.YoungAge {
			young++
		}
	}
	star := lg.ownerBestPlayer()
	switch {
	case status == "seller" || status == "rebuilding":
		bld.Key, bld.Title = "rebuild", "Build for the future"
		bld.Detail = lg.rebuildDetail()
	case star != nil && (young == 0 || lg.Options.NoInSeasonDev || (lg.Year+user)%2 == 0):
		bld.Key, bld.Title, bld.Target = "keep_star", "Keep "+star.Name, star.ID
		bld.Detail = "He is your best player. He has to be on your roster when the season ends."
	case young > 0:
		bld.Key, bld.Title, bld.Target = "improve", fmt.Sprintf("Help a young player improve by %d", t.ImproveGain), t.ImproveGain
		bld.Detail = fmt.Sprintf("A player aged %d or under must gain at least %d overall during the season.", t.YoungAge, t.ImproveGain)
	default:
		bld.Key, bld.Title = "rebuild", "Build for the future"
		bld.Detail = lg.rebuildDetail()
	}
	goals = append(goals, bld)
	return goals
}

// isProspect says whether a young player is a real prospect by visible numbers.
func isProspect(p *Player) bool {
	return p.Age <= OwnerTuning.RebuildAge && (p.Overall() >= OwnerTuning.ProspectOvr || p.Potential >= OwnerTuning.ProspectPot)
}

// prospectsTop counts the user's real prospects among his best ProspectTop players.
func (lg *League) prospectsTop() int {
	n := 0
	for _, p := range rotationOf(lg.Teams[lg.UserTeamID], OwnerTuning.ProspectTop) {
		if isProspect(p) {
			n++
		}
	}
	return n
}

func (lg *League) rebuildDetail() string {
	t := &OwnerTuning
	return fmt.Sprintf("Own your first-round pick in the coming draft and have at least two players aged %d or under among your %d best who are real prospects: overall %d or better, or potential %d or better.",
		t.RebuildAge, t.ProspectTop, t.ProspectOvr, t.ProspectPot)
}

// ownerBestPlayer is the user's best player by overall (lowest ID on a tie).
func (lg *League) ownerBestPlayer() *Player {
	var best *Player
	for _, p := range lg.Teams[lg.UserTeamID].Roster {
		if best == nil || p.Overall() > best.Overall() || (p.Overall() == best.Overall() && p.ID < best.ID) {
			best = p
		}
	}
	return best
}

// ownerStartSeason runs at the end of StartSeason: creates the owner state if
// it is missing (an old save), sets the budget and the three goals and says so.
func (lg *League) ownerStartSeason(prevRev int) {
	if ownerOff {
		return
	}
	if lg.Owner == nil {
		lg.Owner = newOwnerState()
	}
	o := lg.Owner
	user := lg.UserTeamID
	info := lg.OwnerOf(user)
	o.Budget = lg.ownerBudgetFor(user, prevRev)
	o.Goals = lg.ownerPickGoals(info, o.Budget)
	o.OverWeeks, o.PaceWarned, o.OverNoted = 0, false, false
	o.SeasonStartTrust = o.Trust
	o.LastChange = 0
	o.StartOvr = lg.userYoungIDs(OwnerTuning.YoungAge)
	o.StartDir = lg.TeamStatusOf(user).Key

	titles := ""
	for i, g := range o.Goals {
		if i > 0 {
			titles += "; "
		}
		titles += g.Title
	}
	lg.ownerSay("info", fmt.Sprintf("My goals for this season: %s.", titles))
	lg.ownerSay("info", fmt.Sprintf("The payroll budget is %s.", fmtM(o.Budget)))
	if pay := lg.Payroll(user); pay > o.Budget {
		lg.ownerSay("warn", fmt.Sprintf("Your payroll of %s is already over the budget of %s. Each week like that costs me trust in you.", fmtM(pay), fmtM(o.Budget)))
	}
}

// ---- in-season ----

// ownerWeek runs once per sim week, beside the morale week.
func (lg *League) ownerWeek() {
	o := lg.Owner
	if o == nil || o.Fired || lg.Season == nil {
		return
	}
	s := lg.Season
	user := lg.UserTeamID
	if pay := lg.Payroll(user); o.Budget > 0 && pay > o.Budget {
		o.OverWeeks++
		lg.ownerMove(-OwnerTuning.OverWeekPenalty)
		if !o.OverNoted {
			o.OverNoted = true
			lg.ownerSay("warn", fmt.Sprintf("Your payroll of %s is over the budget of %s. Every week like this costs me trust in you.", fmtM(pay), fmtM(o.Budget)))
		}
	}
	played := s.Wins[user] + s.Losses[user]
	if !o.PaceWarned && played >= OwnerTuning.PaceDay {
		o.PaceWarned = true
		for _, g := range o.Goals {
			if g.Kind != "results" {
				continue
			}
			proj := float64(s.Wins[user]) * float64(SeasonGames) / float64(played)
			if proj < float64(g.Target-OwnerTuning.PaceSlack) {
				lg.ownerMove(-OwnerTuning.PacePenalty)
				lg.ownerSay("warn", fmt.Sprintf("You are %d-%d. At this pace I will not get what I asked for: %s.", s.Wins[user], s.Losses[user], lowerFirst(g.Title)))
			}
		}
	}
}

// ---- judging ----

// userRoundsWon is how many playoff series the user's team has won.
func (lg *League) userRoundsWon() int {
	n := 0
	for _, sr := range lg.Season.Series {
		if sr.Done && sr.Winner == lg.UserTeamID {
			n++
		}
	}
	return n
}

// ownerOwnsFirst says whether the user owns a first-round pick in this year's draft.
func (lg *League) ownerOwnsFirst() bool {
	for _, pk := range lg.Picks {
		if pk.Year == lg.Year && pk.Round == 1 && pk.Owner == lg.UserTeamID {
			return true
		}
	}
	return false
}

// youngTop8 counts players aged age or under among the user's best eight.
func (lg *League) youngTop8(age int) int {
	rot := rotationOf(lg.Teams[lg.UserTeamID], 8)
	n := 0
	for _, p := range rot {
		if p.Age <= age {
			n++
		}
	}
	return n
}

// ownerGoalState says how a goal stands right now: whether it is already met
// (met), whether it can no longer be met (lost), whether it is at risk, and a
// sentence about where it stands.
func (lg *League) ownerGoalState(g *OwnerGoal) (met, lost, risk bool, text string) {
	o := lg.Owner
	s := lg.Season
	user := lg.UserTeamID
	t := &OwnerTuning
	w, l := s.Wins[user], s.Losses[user]
	played := w + l
	proj := float64(w)
	if played > 0 {
		proj = float64(w) * float64(SeasonGames) / float64(played)
	}
	behind := played >= 10 && proj < float64(g.Target-t.PaceSlack)
	regularDone := s.Done()
	switch g.Key {
	case "wins":
		met = w >= g.Target
		lost = regularDone && !met
		risk = !met && behind
		text = fmt.Sprintf("You are %d-%d. The goal is %d wins.", w, l, g.Target)
	case "playoffs":
		rank := s.ConferenceRank(user)
		if regularDone {
			met, lost = s.MadePlayoffs(user), !s.MadePlayoffs(user)
		}
		risk = !met && (behind || (played >= 20 && rank > PlayoffSeeds))
		text = fmt.Sprintf("You are %d-%d, %s in the conference.", w, l, ordinal(rank))
		if regularDone {
			text = map[bool]string{true: "You made the playoffs.", false: "You missed the playoffs."}[met]
		}
	case "round", "conf_finals":
		need := 1
		if g.Key == "conf_finals" {
			need = 2
		}
		won := lg.userRoundsWon()
		met = won >= need
		if s.PlayoffsStarted {
			alive := lg.userAlive()
			lost = !met && !alive
			switch {
			case met:
				text = "You have won the rounds you needed."
			case lost:
				text = "Your playoff run is over."
			default:
				text = fmt.Sprintf("You have won %d of the %d rounds you need.", won, need)
			}
		} else {
			lost = regularDone && !s.MadePlayoffs(user)
			risk = behind
			text = fmt.Sprintf("You are %d-%d. The playoffs have not started.", w, l)
		}
	case "budget":
		pay := lg.Payroll(user)
		met = pay <= o.Budget
		risk = !met
		if met {
			text = fmt.Sprintf("Payroll %s, budget %s. You are inside it.", fmtM(pay), fmtM(o.Budget))
		} else {
			text = fmt.Sprintf("Payroll %s is %s over the budget of %s.", fmtM(pay), fmtM(pay-o.Budget), fmtM(o.Budget))
		}
	case "tax":
		bill := lg.TaxBill(user)
		met = bill == 0
		risk = !met
		if met {
			text = fmt.Sprintf("Payroll %s is under the tax line of %s.", fmtM(lg.Payroll(user)), fmtM(Cap.TaxLine))
		} else {
			text = fmt.Sprintf("You owe %s in luxury tax.", fmtM(bill))
		}
	case "profit":
		m := lg.OwnerFinances()
		met = m.Profit >= 0
		risk = !met
		if met {
			text = fmt.Sprintf("You are on course for a profit of %s.", fmtM(m.Profit))
		} else {
			text = fmt.Sprintf("You are on course for a loss of %s.", fmtM(-m.Profit))
		}
	case "improve":
		best, name := -100, ""
		for _, p := range lg.Teams[user].Roster {
			if base, ok := o.StartOvr[p.ID]; ok && p.Overall()-base > best {
				best, name = p.Overall()-base, p.Name
			}
		}
		met = best >= g.Target
		if name == "" {
			text = "None of your young players is on the roster any more."
			risk = true
		} else {
			text = fmt.Sprintf("%s is up %d overall this season; you need %d.", name, max(best, 0), g.Target)
			risk = !met && regularDone
		}
	case "keep_star":
		p := lg.findRosterPlayer(user, g.Target)
		met = p != nil
		risk = !met
		if met {
			text = fmt.Sprintf("%s is still on your roster.", p.Name)
		} else {
			text = "He is no longer on your roster."
			lost = true
		}
	case "rebuild":
		pick, young := lg.ownerOwnsFirst(), lg.prospectsTop()
		met = pick && young >= 2
		risk = !met && regularDone
		text = fmt.Sprintf("You %s your first-round pick in this draft and have %d real prospects (aged %d or under, overall %d+ or potential %d+) among your %d best; you need 2.",
			map[bool]string{true: "own", false: "do not own"}[pick], young, t.RebuildAge, t.ProspectOvr, t.ProspectPot, t.ProspectTop)
	case "spend":
		pay := lg.Payroll(user)
		met = pay >= g.Target
		risk = !met
		text = fmt.Sprintf("Payroll is %s; the owner wants at least %s.", fmtM(pay), fmtM(g.Target))
	}
	return
}

// userAlive says whether the user's team still has a playoff series to play.
func (lg *League) userAlive() bool {
	s := lg.Season
	u := lg.UserTeamID
	if s.Champion == u {
		return true
	}
	for _, sr := range s.Series {
		if (sr.High == u || sr.Low == u) && !sr.Done {
			return true
		}
	}
	return false
}

// OwnerGoalStatus is one goal's live status for the screens: its progress
// sentence and "on_track", "at_risk", "met" or "missed".
func (lg *League) OwnerGoalStatus(i int) (progress, status string) {
	o := lg.Owner
	g := &o.Goals[i]
	met, lost, risk, text := lg.ownerGoalState(g)
	if g.Done { // "met" and "missed" only once the goal was judged at season end
		if g.Met {
			return text, "met"
		}
		return text, "missed"
	}
	if lost || (risk && !met) {
		return text, "at_risk"
	}
	return text, "on_track"
}

// ownerSeasonEnd judges the season: called first thing in BeginOffseason,
// after the champion is known and before anything changes the rosters.
func (lg *League) ownerSeasonEnd() {
	o := lg.Owner
	if o == nil || lg.Season == nil {
		return
	}
	t := &OwnerTuning
	s := lg.Season
	user := lg.UserTeamID
	info := lg.OwnerOf(user)
	speed := map[string]float64{OwnerWinNow: t.SpeedWinNow, OwnerPatient: t.SpeedPatient, OwnerCheap: t.SpeedCheap}[info.Personality]
	startTrust := o.SeasonStartTrust

	metCount := 0
	delta := 0
	resultsMissed := false
	goalRounds := 0
	var rows []OwnerGoalResult
	for i := range o.Goals {
		g := &o.Goals[i]
		met, _, _, _ := lg.ownerGoalState(g)
		g.Met, g.Done = met, true
		per := t.GoalGain
		sign := 1
		if !met {
			per, sign = t.GoalLoss, -1
		} else {
			metCount++
		}
		delta += sign * int(math.Round(float64(per*g.Weight)*speed))
		if g.Kind == "results" {
			resultsMissed = !met
			switch g.Key {
			case "round":
				goalRounds = 1
			case "conf_finals":
				goalRounds = 2
			}
		}
		rows = append(rows, OwnerGoalResult{Title: g.Title, Met: met})
	}
	won := lg.userRoundsWon()
	bonus := 0
	if won > goalRounds {
		bonus = (won - goalRounds) * t.PlayoffBonus
		if s.Champion == user {
			bonus += t.PlayoffBonus
		}
	}
	wd := float64(s.Wins[user] - t.WinsPivot)
	if wd >= 0 {
		wd *= t.WinsTrustUp
	} else {
		wd *= t.WinsTrustDown
	}
	delta += int(math.Round((float64(bonus) + wd) * speed))
	// Losing patience, whatever the goals said.
	losing := s.Wins[user] < t.LosingWins
	buildMet := false
	for _, g := range o.Goals {
		if g.Kind == "building" && g.Met {
			buildMet = true
		}
	}
	forgiven := false
	if losing {
		grace := t.RebuildGrace[map[string]int{OwnerWinNow: 0, OwnerPatient: 1, OwnerCheap: 2}[info.Personality]]
		if buildMet && (o.StartDir == "seller" || o.StartDir == "rebuilding") && o.GraceUsed < grace {
			o.GraceUsed++
			forgiven = true
		} else {
			o.LosingStreak = min(t.LosingStreakMax, o.LosingStreak+1)
			delta -= int(math.Round(float64(t.LosingStep*o.LosingStreak) * speed))
		}
		if delta > 0 {
			delta = 0 // a losing season never raises trust
		}
	} else {
		o.LosingStreak = max(0, o.LosingStreak-1)
		o.GraceUsed = 0
	}
	o.PrevWins, o.PrevLosing = s.Wins[user], losing
	// One season can cost at most MaxSeasonDrop in all (weekly moves included),
	// so the owner is warned for a season before he lets anyone go.
	if floor := startTrust - t.MaxSeasonDrop; o.Trust+delta < floor {
		delta = min(0, floor-o.Trust)
	}
	lg.ownerMove(delta)
	o.LastChange = o.Trust - startTrust
	if resultsMissed {
		o.BadSeasons++
	} else {
		o.BadSeasons = 0
	}

	fin := lg.OwnerFinances()
	n := len(o.Goals)
	row := OwnerSeason{Year: lg.Year, Wins: s.Wins[user], Losses: s.Losses[user], Playoffs: lg.UserPlayoffResult(),
		Revenue: fin.Revenue, Costs: fin.Costs, Profit: fin.Profit, TrustStart: startTrust, TrustEnd: o.Trust, Goals: rows}
	o.Ledger = append(o.Ledger, row)
	if k := len(o.Ledger); k > t.LedgerKeep {
		o.Ledger = append([]OwnerSeason(nil), o.Ledger[k-t.LedgerKeep:]...)
	}

	switch {
	case losing && forgiven:
		lg.ownerSay("warn", fmt.Sprintf("%d wins is not good, but I can see you are building something. I will wait a little.", s.Wins[user]))
	case losing:
		lg.ownerSay("bad", fmt.Sprintf("%d wins is a losing season, and I have seen too many of them.", s.Wins[user]))
	}
	switch {
	case metCount == n:
		lg.ownerSay("good", fmt.Sprintf("You met all %d goals. I am very pleased.", n))
	case metCount*2 > n:
		lg.ownerSay("good", fmt.Sprintf("You met %d of %d goals. I am pleased.", metCount, n))
	case metCount > 0:
		lg.ownerSay("warn", fmt.Sprintf("You met %d of %d goals. My patience is running out.", metCount, n))
	default:
		lg.ownerSay("bad", fmt.Sprintf("You met none of the %d goals. My patience is running out.", n))
	}

	if !lg.Options.NoFiring && (o.Trust <= 0 || (o.BadSeasons >= t.BadSeasons && o.Trust < t.BadSeasonsTrust)) {
		o.Fired = true
		o.FiredText = fmt.Sprintf("%s has lost faith in you after the %d season and has let you go. Choose another team that is hiring.", info.Name, lg.Year)
		lg.ownerSay("bad", o.FiredText)
	}
}

// ---- losing and taking a job ----

// OwnerJobs are the teams hiring while the user is fired: the three worst
// records of the season just finished, not the user's old team. Empty when
// the user is not fired.
func (lg *League) OwnerJobs() []int {
	if !lg.OwnerFired() || lg.Season == nil {
		return []int{}
	}
	order := lg.Season.Standings()
	out := []int{}
	for i := len(order) - 1; i >= 0 && len(out) < OwnerTuning.JobCount; i-- {
		if order[i] != lg.UserTeamID {
			out = append(out, order[i])
		}
	}
	return out
}

// Errors from TakeJob.
var (
	ErrOwnerNotFired = errors.New("You have not been let go, so there is no new job to take.")
	ErrOwnerBadJob   = errors.New("That team is not hiring.")
)

// TakeJob moves the user to a team that is hiring. The old team goes back to
// the automatic lineup and its coach's system, pending offers are dropped, the
// assistant GM keeps its plan, scouting points stay, and the new owner starts
// with the starting trust. Returns a sentence for the screens.
func (lg *League) TakeJob(teamID int) (string, error) {
	o := lg.Owner
	if o == nil || !o.Fired {
		return "", ErrOwnerNotFired
	}
	ok := false
	for _, id := range lg.OwnerJobs() {
		if id == teamID {
			ok = true
		}
	}
	if !ok {
		return "", ErrOwnerBadJob
	}
	old := lg.Teams[lg.UserTeamID]
	old.Lineup, old.System = nil, ""
	lg.Proposals = nil
	if lg.Draft != nil {
		lg.Draft.OfferChecked = 0 // the offer flag belonged to the old team's pick
	}
	lg.UserTeamID = teamID
	lg.gmAlert = ""
	lg.gmCounts = nil
	o.Fired, o.FiredText = false, ""
	o.Trust, o.BadSeasons, o.LastChange = OwnerTuning.StartTrust, 0, 0
	o.LosingStreak, o.GraceUsed, o.PrevWins, o.PrevLosing = 0, 0, 0, false
	o.SeasonStartTrust = o.Trust
	o.Goals, o.OverWeeks = nil, 0
	t := lg.Teams[teamID]
	info := lg.OwnerOf(teamID)
	msg := fmt.Sprintf("You are now the GM of the %s. Your new owner is %s.", t.FullName(), info.Name)
	lg.ownerSay("info", msg)
	return msg, nil
}
