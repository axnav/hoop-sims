package engine

import (
	"fmt"
	"math"
	"sort"
)

// Morale and trade requests.
//
// Every player on a team has a mood (Player.Morale, saved, -50..+50, zero =
// content). Once a sim week (every 7 days of the regular season) it moves a
// share of the way toward a target built from five parts: his role (minutes
// against his rank), the team's winning, his pay against his market salary,
// his years with the team, and a Leader on the team. Each part is weighted by
// what the player cares about (MotivationOf). Injured players are not
// punished for the minutes they miss.
//
// Effects: a small make-chance nudge in games, measured against the league
// mean mood so the league as a whole plays as before; low mood pushes the
// stance toward "test" and raises the extension ask; a player who stays
// below the request line for a few weeks may ask to be traded. A request
// lowers his trade value, deepens his own mood and dents his teammates'
// moods each week, and computer teams move their own requesters more readily.
//
// Random stream offset 120000 (RULES.md section 3): every draw is hashed from
// the seed, the player and the week, so it never depends on order.

const moraleOffset = 120000

// weeksPerSeason is used to count weeks across a year change.
const weeksPerSeason = 12

// MoraleTuning holds every number behind morale. Edit here.
var MoraleTuning = struct {
	Min, Max int // the scale
	// Each sim week morale moves Share of the way toward its target (at least
	// one point while the gap is a point or more).
	Share float64
	// Target parts: the part's points (MotivationTuning values, win% and pay
	// ratio) times the scale times the player's trait weight, in mood points.
	RoleScale, WinScale, MoneyScale, LoyaltyScale float64
	WinPctPts                                     float64 // points for a win% of 100% above .500 (times 2)
	WinMinGames                                   int     // games played before the win% counts
	PayRatioScale                                 float64 // points per unit of salary / market salary away from 1
	PayMin, PayMax                                float64 // limits of the pay points
	PayMinExperience                              int     // rookie-scale deals (up to this many seasons) are not judged
	LeaderPts                                     float64 // mood points when a Leader plays on the team
	LeaderMinutes                                 int     // minutes a Leader needs to count
	// Trade request pressure.
	RequestStep, RequestCap float64 // extra minus on his own target per week since the request, and its limit
	MateStep, MateCap       float64 // minus on a teammate's target per requester on the team, and its limit
	// A request starts after RequestWeeks weeks in a row at or below
	// RequestBelow, each later week with chance RequestChance; it ends after
	// RecoverWeeks weeks in a row at or above RecoverAbove.
	RequestBelow, RecoverAbove int
	RequestWeeks, RecoverWeeks int
	RequestChance              float64
	// Computer teams: a requester's trade value is lowered by this share
	// (everyone knows he wants out), his own team accepts RequesterMargin
	// instead of 1 + TradeMargin + 0.05 on its side of a swap and may move him
	// even if he is a top player, and each sim week (trade window open) one
	// swap with a requester is tried with chance MoveChance.
	RequestDiscount, RequesterMargin, MoveChance float64
	// Offseason: a re-signed or extended player gains ReSignLift, a player who
	// changes team keeps NewTeamKeep of his mood, and everyone keeps
	// OffseasonKeep of it into the new season.
	ReSignLift                 int
	NewTeamKeep, OffseasonKeep float64
	// Game: make chance per mood point above the league mean (minutes
	// weighted), ceiling.
	EffectScale, EffectCap float64
	// Stance: points at mood -50 and +50 (scaled in between); only moods
	// beyond +-StanceFrom count.
	StanceNeg, StancePos float64
	StanceFrom           int
	// Extension: the ask is multiplied by 1 + AskUnhappy at mood -50 and by
	// 1 - AskHappy at +50.
	AskUnhappy, AskHappy float64
	// Labels: mood below Furious is "Furious", below Unhappy "Unhappy", below
	// Happy "Content", below Thrilled "Happy", else "Thrilled".
	Furious, Unhappy, Happy, Thrilled int
	// Trend: the target must be this far from the mood to say up or down.
	TrendGap float64
	// Events kept.
	EventsKept int
}{
	Min: -50, Max: 50,
	Share:     0.25,
	RoleScale: 20, WinScale: 20, MoneyScale: 20, LoyaltyScale: 20,
	WinPctPts: 1.0, WinMinGames: 8,
	PayRatioScale: 0.8, PayMin: -0.6, PayMax: 0.3, PayMinExperience: 3,
	LeaderPts: 4, LeaderMinutes: 15,
	RequestStep: 2, RequestCap: 16,
	MateStep: 1.5, MateCap: 6,
	RequestBelow: -28, RecoverAbove: -12,
	RequestWeeks: 3, RecoverWeeks: 3,
	RequestChance:   0.3,
	RequestDiscount: 0.12, RequesterMargin: 0.95, MoveChance: 0.5,
	ReSignLift: 10, NewTeamKeep: 0.5, OffseasonKeep: 0.75,
	EffectScale: 0.00008, EffectCap: 0.003,
	StanceNeg: 0.6, StancePos: 0.3, StanceFrom: 10,
	AskUnhappy: 0.08, AskHappy: 0.04,
	Furious: -30, Unhappy: -10, Happy: 10, Thrilled: 30,
	TrendGap:   2,
	EventsKept: 300,
}

// Event types in League.Events.
const (
	EvTradeRequest     = "trade_request"
	EvRequestWithdrawn = "request_withdrawn"
	EvRequestTraded    = "request_traded"
)

// LeagueEvent is one morale event (Day is the engine's 0-based day).
type LeagueEvent struct {
	Year, Day int
	Type      string
	PlayerID  int
	Name      string
	TeamID    int
}

func (lg *League) logEvent(typ string, p *Player, teamID int) {
	lg.Events = append(lg.Events, LeagueEvent{Year: lg.Year, Day: lg.currentDay(), Type: typ, PlayerID: p.ID, Name: p.Name, TeamID: teamID})
	lg.newsRequest(typ, p, teamID) // news.go
	if n := len(lg.Events); n > MoraleTuning.EventsKept {
		lg.Events = append([]LeagueEvent(nil), lg.Events[n-MoraleTuning.EventsKept:]...)
	}
}

// MoraleLabel is the word for a mood.
func MoraleLabel(morale int) string {
	t := &MoraleTuning
	switch {
	case morale < t.Furious:
		return "Furious"
	case morale < t.Unhappy:
		return "Unhappy"
	case morale < t.Happy:
		return "Content"
	case morale < t.Thrilled:
		return "Happy"
	}
	return "Thrilled"
}

// MoraleScore is the 0 to 100 score for a mood (50 = content).
func MoraleScore(morale int) int { return 50 + morale }

// ---- the target ----

type moralePart struct {
	pts    float64
	reason string
}

// moraleCalc is a player's target mood and what it is made of.
type moraleCalc struct {
	target float64
	parts  []moralePart
}

// moraleTeamCtx is what the target needs about one team, worked out once.
type moraleTeamCtx struct {
	tm      *Team
	pms     []PlayerMinutes
	status  TeamStatus
	leaders []*Player // Leaders who play
	askers  int       // players on the team with a trade request
}

func (lg *League) moraleCtx(tm *Team, statuses []TeamStatus) *moraleTeamCtx {
	t := &MoraleTuning
	c := &moraleTeamCtx{tm: tm, pms: tm.EffectiveMinutes()}
	if statuses == nil {
		statuses = lg.TeamStatuses()
	}
	c.status = statuses[tm.ID]
	for _, pm := range c.pms {
		if pm.Min >= t.LeaderMinutes && !pm.Player.Injured() {
			if l, _, _ := lg.personality(pm.Player); l {
				c.leaders = append(c.leaders, pm.Player)
			}
		}
	}
	for _, p := range tm.Roster {
		if p.TradeRequest > 0 {
			c.askers++
		}
	}
	return c
}

// weeksSince is the number of sim weeks since a request marker
// (year*100+week), at least 1 while it is active.
func (lg *League) weeksSince(marker int) int {
	cy, cw := lg.Year, 0
	if s := lg.Season; s != nil && s.Year == lg.Year {
		cw = s.Day / 7
	}
	w := (cy-marker/100)*weeksPerSeason + cw - marker%100
	return max(1, w)
}

func (lg *League) requestMarker() int {
	w := 0
	if s := lg.Season; s != nil && s.Year == lg.Year {
		w = s.Day / 7
	}
	return lg.Year*100 + w
}

// moraleTarget builds the target mood of a player of the team in ctx.
func (lg *League) moraleTarget(p *Player, c *moraleTeamCtx) moraleCalc {
	t := &MoraleTuning
	mt := &MotivationTuning
	m := lg.MotivationOf(p)
	w := func(l Level) float64 { return mt.TraitWeight[l] }
	var out moraleCalc
	add := func(pts float64, reason string) {
		out.target += pts
		if reason != "" && math.Abs(pts) >= 0.5 {
			out.parts = append(out.parts, moralePart{pts, reason})
		}
	}

	// Role: the stance logic. An injured player is not punished: only the
	// good side of the role counts.
	pts, _, short := lg.rolePts(c.tm, p, c.pms)
	if p.Injured() {
		pts = math.Max(0, pts)
	}
	switch {
	case pts > 0:
		add(pts*t.RoleScale*w(m.Role), "Happy in his role as a starter")
	case short:
		add(pts*t.RoleScale*w(m.Role), "Plays fewer minutes than he expects")
	default:
		add(pts*t.RoleScale*w(m.Role), "Wants a bigger role")
	}

	// Winning: the team's direction plus its record.
	var dir float64
	switch c.status.Key {
	case "contender":
		dir = mt.ContendPts
	case "buyer":
		dir = mt.BuyerPts
	case "seller":
		dir = mt.SellerPts
	default:
		dir = mt.RebuildPts
	}
	if s := lg.Season; s != nil && s.Year == lg.Year && c.tm.ID < len(s.Wins) {
		if g := s.Wins[c.tm.ID] + s.Losses[c.tm.ID]; g >= t.WinMinGames {
			dir += (float64(s.Wins[c.tm.ID])/float64(g) - 0.5) * 2 * t.WinPctPts * 0.5
		}
	}
	reason := "Likes that the team is winning"
	if dir < 0 {
		reason = "Frustrated by the team's losing"
	}
	add(dir*t.WinScale*w(m.Winning), reason)

	// Pay against his market salary (rookie-scale deals are not judged).
	if p.Contract.Salary > 0 && p.Experience > t.PayMinExperience {
		ratio := float64(p.Contract.Salary) / float64(MarketSalary(p))
		x := math.Max(t.PayMin, math.Min(t.PayMax, (ratio-1)*t.PayRatioScale))
		reason := "Happy with his pay"
		if x < 0 {
			reason = "Feels underpaid"
		}
		add(x*t.MoneyScale*w(m.Money), reason)
	}

	// Years with the team.
	if p.TeamSince > 0 {
		yrs := max(0, lg.Year-p.TeamSince)
		if yrs > mt.TenureFrom {
			f := math.Min(1, float64(min(yrs, mt.TenureFull)-mt.TenureFrom)/float64(mt.TenureFull-mt.TenureFrom))
			add(mt.TenurePts*f*t.LoyaltyScale*w(m.Loyalty), fmt.Sprintf("Has been here %d years", yrs))
		}
	}

	// A Leader who plays (another one, if he is a Leader himself).
	for _, l := range c.leaders {
		if l != p {
			add(t.LeaderPts, "Looks up to a Leader on the team")
			break
		}
	}

	// Trade request pressure: his own, and his teammates'.
	if p.TradeRequest > 0 {
		wk := float64(lg.weeksSince(p.TradeRequest))
		add(-math.Min(t.RequestCap, t.RequestStep*wk), "Is stuck on a team he asked to leave")
	}
	if n := c.askers; n > 0 {
		if p.TradeRequest > 0 {
			n--
		}
		if n > 0 {
			add(-math.Min(t.MateCap, t.MateStep*float64(n)), "A teammate wants out")
		}
	}

	out.target = math.Max(float64(t.Min), math.Min(float64(t.Max), out.target))
	return out
}

// moraleCalcOf is the target of one player (nil for a player without a team).
func (lg *League) moraleCalcOf(p *Player) *moraleCalc {
	if p == nil || p.TeamID < 0 || p.TeamID >= len(lg.Teams) || lg.IsProspect(p) {
		return nil
	}
	c := lg.moraleCtx(lg.Teams[p.TeamID], nil)
	mc := lg.moraleTarget(p, c)
	return &mc
}

// MoraleInfo is what the screens show for one player.
type MoraleInfo struct {
	Score        int
	Label        string
	Trend        string // up, down or steady
	Reasons      []string
	TradeRequest bool
}

// MoraleBrief is the short form for list rows (no reasons or trend): the
// score and label, ok false for a player without a team or a prospect.
func (lg *League) MoraleBrief(p *Player) (score int, label string, ok bool) {
	if p == nil || p.TeamID < 0 || p.TeamID >= len(lg.Teams) || lg.IsProspect(p) {
		return 0, "", false
	}
	return MoraleScore(p.Morale), MoraleLabel(p.Morale), true
}

// MoraleOf is a player's mood for the screens; ok is false for a player
// without a team (free agent, retired) or a draft prospect.
func (lg *League) MoraleOf(p *Player) (MoraleInfo, bool) {
	mc := lg.moraleCalcOf(p)
	if mc == nil {
		return MoraleInfo{}, false
	}
	t := &MoraleTuning
	info := MoraleInfo{Score: MoraleScore(p.Morale), Label: MoraleLabel(p.Morale), Trend: "steady",
		Reasons: []string{}, TradeRequest: p.TradeRequest > 0}
	switch gap := mc.target - float64(p.Morale); {
	case gap >= t.TrendGap:
		info.Trend = "up"
	case gap <= -t.TrendGap:
		info.Trend = "down"
	}
	parts := append([]moralePart(nil), mc.parts...)
	sort.SliceStable(parts, func(i, j int) bool { return math.Abs(parts[i].pts) > math.Abs(parts[j].pts) })
	for _, x := range parts {
		if len(info.Reasons) < 4 {
			info.Reasons = append(info.Reasons, x.reason)
		}
	}
	return info, true
}

// TeamMorale is the team's mood, minutes weighted, as a score and a label.
func (lg *League) TeamMorale(tm *Team) (score int, label string) {
	sum, w := 0.0, 0.0
	for _, pm := range tm.EffectiveMinutes() {
		sum += float64(pm.Min) * float64(pm.Player.Morale)
		w += float64(pm.Min)
	}
	m := 0.0
	if w > 0 {
		m = sum / w
	}
	return MoraleScore(int(math.Round(m))), MoraleLabel(int(math.Round(m)))
}

// ---- the weekly update ----

// moraleWeek runs once per sim week (SimDay calls it every 7th day): every
// player on a team moves toward his target, trade requests start and end,
// and computer teams may move a requester.
func (lg *League) moraleWeek() {
	t := &MoraleTuning
	week := lg.requestMarker() % 100
	statuses := lg.TeamStatuses()
	for _, tm := range lg.Teams {
		c := lg.moraleCtx(tm, statuses)
		// Targets first (they read the requests as they stood), then move.
		targets := make([]float64, len(tm.Roster))
		for i, p := range tm.Roster {
			targets[i] = lg.moraleTarget(p, c).target
		}
		for i, p := range tm.Roster {
			gap := targets[i] - float64(p.Morale)
			step := gap * t.Share
			if math.Abs(step) < 1 && math.Abs(gap) >= 1 {
				step = math.Copysign(1, gap)
			}
			p.Morale = clamp(p.Morale+int(math.Round(step)), t.Min, t.Max)
			lg.requestStep(p, tm.ID, week)
		}
	}
	lg.mmeans = nil
	lg.moveRequesters(week)
}

// requestStep updates one player's request state after his mood moved.
func (lg *League) requestStep(p *Player, teamID, week int) {
	t := &MoraleTuning
	if p.TradeRequest == 0 {
		if p.Morale > t.RequestBelow {
			p.MoraleStreak = 0
			return
		}
		p.MoraleStreak++
		if p.MoraleStreak < t.RequestWeeks {
			return
		}
		r := NewRNG(mix64(lg.Seed, moraleOffset, uint64(p.ID), uint64(lg.Year*100+week)))
		if r.Chance(t.RequestChance) {
			p.TradeRequest = lg.Year*100 + week
			p.MoraleStreak = 0
			lg.logEvent(EvTradeRequest, p, teamID)
		}
		return
	}
	if p.Morale < t.RecoverAbove {
		p.MoraleStreak = 0
		return
	}
	p.MoraleStreak++
	if p.MoraleStreak >= t.RecoverWeeks {
		p.TradeRequest, p.MoraleStreak = 0, 0
		lg.logEvent(EvRequestWithdrawn, p, teamID)
	}
}

// moveRequesters lets computer teams try one swap with a requester this week.
func (lg *League) moveRequesters(week int) {
	has := false
	for _, tm := range lg.Teams {
		if tm.ID == lg.UserTeamID {
			continue
		}
		for _, p := range tm.Roster {
			if p.TradeRequest > 0 {
				has = true
			}
		}
	}
	if !has {
		return
	}
	if ok, _ := TradeWindowOpen(lg); !ok {
		return
	}
	r := NewRNG(mix64(lg.Seed, moraleOffset, 1, uint64(lg.Year*100+week)))
	if !r.Chance(MoraleTuning.MoveChance) {
		return
	}
	lg.aiTradeOnce(r, map[int]bool{lg.UserTeamID: true}, map[int]bool{}, true)
}

// moveRequestersOffseason gives computer teams up to four swaps with
// requesters at the start of the new year (each tried with chance
// MoveChance*1.6), after the ordinary offseason trades.
func (lg *League) moveRequestersOffseason() {
	r := NewRNG(mix64(lg.Seed, moraleOffset, 2, uint64(lg.Year)))
	usedTeam, usedPlayer := map[int]bool{lg.UserTeamID: true}, map[int]bool{}
	for i := 0; i < 4; i++ {
		if !r.Chance(math.Min(1, MoraleTuning.MoveChance*1.6)) {
			continue
		}
		if _, ok := lg.aiTradeOnce(r, usedTeam, usedPlayer, true); !ok {
			break
		}
	}
}

// ---- joining and leaving ----

// moraleNewTeam resets a player who changes team (trade, signing, release):
// part of his mood is kept, any request ends. It logs the end of a request
// when he leaves a team on a trade.
func (lg *League) moraleNewTeam(p *Player, fromTeam int, traded bool) {
	if p.TradeRequest > 0 && traded && fromTeam >= 0 {
		lg.logEvent(EvRequestTraded, p, fromTeam)
	}
	p.Morale = int(math.Round(float64(p.Morale) * MoraleTuning.NewTeamKeep))
	p.TradeRequest, p.MoraleStreak = 0, 0
	lg.mmeans = nil
}

// moraleReSigned lifts a player who re-signed or extended; a request ends.
func (lg *League) moraleReSigned(p *Player) {
	t := &MoraleTuning
	p.Morale = clamp(p.Morale+t.ReSignLift, t.Min, t.Max)
	if p.TradeRequest > 0 {
		p.TradeRequest = 0
		lg.logEvent(EvRequestWithdrawn, p, p.TeamID)
	}
	p.MoraleStreak = 0
	lg.mmeans = nil
}

// moraleOffseason carries moods into the new season (they relax toward
// content); requests stay until the player is traded or recovers.
func (lg *League) moraleOffseason() {
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			p.Morale = int(math.Round(float64(p.Morale) * MoraleTuning.OffseasonKeep))
			p.MoraleStreak = 0
		}
	}
	lg.mmeans = nil
}

// ---- effects ----

// moraleMeans are the league means the game effect is measured against
// (minutes weighted; derived from the rosters, never saved; valid for as long
// as the badge means they were built beside).
type moraleMeans struct {
	src  *badgeMeans
	mean float64
}

func (lg *League) moraleMeansNow() *moraleMeans {
	bm := lg.badgeMeansNow()
	if lg.mmeans != nil && lg.mmeans.src == bm {
		return lg.mmeans
	}
	m := &moraleMeans{src: bm}
	any := false
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			if p.Morale != 0 {
				any = true
			}
		}
	}
	if any {
		sum, w := 0.0, 0.0
		for _, t := range lg.Teams {
			for _, pm := range t.EffectiveMinutes() {
				sum += float64(pm.Min) * float64(pm.Player.Morale)
				w += float64(pm.Min)
			}
		}
		if w > 0 {
			m.mean = sum / w
		}
	}
	lg.mmeans = m
	return m
}

// moraleEffect is the make-chance nudge of one player's mood against the
// league mean.
func moraleEffect(p *Player, mean float64) float64 {
	t := &MoraleTuning
	if p.Morale == 0 && mean == 0 {
		return 0
	}
	return clampf(t.EffectScale*(float64(p.Morale)-mean), t.EffectCap)
}

// moraleStancePts is the morale part of the re-sign stance.
func moraleStancePts(p *Player) (float64, string) {
	t := &MoraleTuning
	switch {
	case p.Morale <= -t.StanceFrom:
		return t.StanceNeg * float64(p.Morale) / 50, "Unhappy with his situation"
	case p.Morale >= t.StanceFrom:
		return t.StancePos * float64(p.Morale) / 50, "Happy with his situation"
	}
	return 0, ""
}

// moraleAskFactor multiplies his extension ask: more when unhappy, less when
// happy.
func moraleAskFactor(p *Player) float64 {
	t := &MoraleTuning
	if p.Morale < 0 {
		return 1 - t.AskUnhappy*float64(p.Morale)/50
	}
	return 1 - t.AskHappy*float64(p.Morale)/50
}
