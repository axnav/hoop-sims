package engine

import (
	"fmt"
	"strings"
)

// The league news feed. The engine writes each story as a finished sentence at
// the moment the thing happens and keeps the newest NewsTuning.Kept in
// League.News (saved; nil in an old save). Nothing here draws a random number
// or changes a game: it only reads state and appends text, so results for a
// seed are the same with or without it. Stories name no hidden value, and a
// draft prospect appears by name and pick number only.

// NewsTuning holds every threshold behind the feed. Edit here.
var NewsTuning = struct {
	Kept int // stories kept (newest)
	// Notable player: visible overall at or above NotableOvr. Big: at or above BigOvr.
	NotableOvr, BigOvr int
	// A notable player (or any player of the user's team) out at least
	// InjuryMinGames games makes a story; a notable player out InjuryBigGames or
	// more makes a headline.
	InjuryMinGames, InjuryBigGames int
	// Scoring nights: a story at BigGamePts points, a headline at BigGameBig.
	BigGamePts, BigGameBig int
	// Streaks: a story when a team's streak reaches one of these lengths; a
	// headline for BigWinStreak wins in a row.
	WinStreaks, LoseStreaks []int
	BigWinStreak            int
	// Draft: the first DraftTop picks make stories (and every pick of the user's team).
	DraftTop int
	// Retirement: a story when his best or last overall reached RetireOvr, or he
	// won any award; a headline from RetireBigOvr.
	RetireOvr, RetireBigOvr int
	// A signing or release of a non-notable player of the user's team makes a
	// story only above the minimum salary.
	UserMinSalaryOnly bool
}{
	Kept:       400,
	NotableOvr: 70, BigOvr: 80,
	InjuryMinGames: 10, InjuryBigGames: 40,
	BigGamePts: 50, BigGameBig: 60,
	WinStreaks: []int{10, 15, 20}, LoseStreaks: []int{10, 15},
	BigWinStreak: 15,
	DraftTop:     5,
	RetireOvr:    75, RetireBigOvr: 85,
	UserMinSalaryOnly: true,
}

// News kinds (NewsItem.Kind).
const (
	NwTrade    = "trade"
	NwSigning  = "signing"
	NwRelease  = "release"
	NwInjury   = "injury"
	NwRequest  = "request"
	NwGame     = "game"
	NwStreak   = "streak"
	NwPlayoffs = "playoffs"
	NwDraft    = "draft"
	NwAward    = "award"
	NwRetire   = "retire"
	NwCoach    = "coach"
	NwLeague   = "league"
)

// NewsItem is one story in League.News.
type NewsItem struct {
	ID        int    // increasing; the next ID is the last ID + 1, so it survives trimming
	Year, Day int    // Day is the engine's 0-based season day, -1 outside the regular season
	Stage     string // "season", "playoffs", "draft", "free_agency", "offseason"
	Kind      string
	Text      string   // the finished sentence, written when it happened
	Big       bool     // headline story
	TeamIDs   []int    `json:",omitempty"`
	PlayerIDs []int    `json:",omitempty"`
	Names     []string `json:",omitempty"` // names matching PlayerIDs, as they were then
}

// News filter keys, in the order the screens list them.
var newsFilterKeys = []string{"all", "mine", "trades", "signings", "injuries", "games", "awards", "draft"}

// NewsFilterKeys lists the filter keys of NewsFor in display order.
func NewsFilterKeys() []string { return append([]string(nil), newsFilterKeys...) }

// ValidNewsFilter says whether key is one of the filters.
func ValidNewsFilter(key string) bool {
	for _, k := range newsFilterKeys {
		if k == key {
			return true
		}
	}
	return false
}

// newsStageNow gives the stage and day for a story written now. An explicit
// stage ("draft", "offseason") wins; "" works it out from the league's state.
func (lg *League) newsStageNow(stage string) (string, int) {
	s := lg.Season
	if stage != "" {
		return stage, -1
	}
	switch {
	case lg.FA != nil:
		return "free_agency", -1
	case s != nil && s.Year == lg.Year && lg.Draft == nil:
		if s.Champion >= 0 {
			return "offseason", -1 // the playoffs are over
		}
		if s.PlayoffsStarted {
			return "playoffs", -1
		}
		return "season", min(s.Day, SeasonDays-1)
	}
	return "offseason", -1
}

// addNews appends a story. stage "" = work it out. Players are given as
// *Player so the name is kept as it is now.
func (lg *League) addNews(stage, kind, text string, big bool, teams []int, players ...*Player) {
	st, day := lg.newsStageNow(stage)
	id := 1
	if n := len(lg.News); n > 0 {
		id = lg.News[n-1].ID + 1
	}
	it := NewsItem{ID: id, Year: lg.Year, Day: day, Stage: st, Kind: kind, Text: text, Big: big}
	for _, t := range teams {
		if t >= 0 && t < len(lg.Teams) {
			it.TeamIDs = append(it.TeamIDs, t)
		}
	}
	for _, p := range players {
		if p != nil {
			it.PlayerIDs = append(it.PlayerIDs, p.ID)
			it.Names = append(it.Names, p.Name)
		}
	}
	lg.News = append(lg.News, it)
	if n := len(lg.News); n > NewsTuning.Kept {
		lg.News = append([]NewsItem(nil), lg.News[n-NewsTuning.Kept:]...)
	}
}

func (lg *League) teamName(id int) string {
	if id >= 0 && id < len(lg.Teams) {
		return lg.Teams[id].FullName()
	}
	return "free agency"
}

func (lg *League) notable(p *Player) bool { return p != nil && p.Overall() >= NewsTuning.NotableOvr }

// nameList is "A", "A and B" or "A, B and C".
func nameList(xs []string) string {
	switch len(xs) {
	case 0:
		return ""
	case 1:
		return xs[0]
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " and " + xs[len(xs)-1]
}

func ordinalRound(r int) string {
	switch r {
	case 1:
		return "first-round"
	case 2:
		return "second-round"
	}
	return fmt.Sprintf("round-%d", r)
}

// ---- trades, signings, releases ----

// newsTrade writes the story for an executed trade (called where the trade is logged).
func (lg *League) newsTrade(res TradeResult) {
	teams := res.Teams
	if len(teams) == 0 {
		teams = []int{res.TeamA, res.TeamB}
	}
	var parts []string
	big := false
	var ps []*Player
	for _, tid := range teams {
		var got []string
		for _, m := range res.Moved {
			if m.To == tid {
				got = append(got, m.Name)
				if p := lg.FindPlayer(m.PlayerID); p != nil {
					ps = append(ps, p)
					if p.Overall() >= NewsTuning.BigOvr {
						big = true
					}
				}
			}
		}
		for _, pk := range res.Picks {
			if pk.To == tid {
				got = append(got, fmt.Sprintf("a %d %s pick", pk.Year, ordinalRound(pk.Round)))
			}
		}
		if len(got) == 0 {
			got = []string{"cash considerations"}
		}
		parts = append(parts, "the "+lg.teamName(tid)+" receive "+nameList(got))
		if tid == lg.UserTeamID {
			big = true
		}
	}
	lg.addNews("", NwTrade, "Trade: "+strings.Join(parts, "; ")+".", big, teams, ps...)
}

// newsSigning writes the story for a signing, re-signing or extension.
// how is "signs", "re-signs" or "extends".
func (lg *League) newsSigning(teamID int, p *Player, salary, years int, how string) {
	user := teamID == lg.UserTeamID
	if !lg.notable(p) && !(user && (!NewsTuning.UserMinSalaryOnly || salary > Cap.MinSalary)) {
		return
	}
	var text string
	switch how {
	case "re-signs":
		text = fmt.Sprintf("The %s re-sign %s: %s a year for %d %s.", lg.teamName(teamID), p.Name, fmtM(salary), years, yearWord(years))
	case "extends":
		text = fmt.Sprintf("The %s extend %s: %s a year for %d more %s.", lg.teamName(teamID), p.Name, fmtM(salary), years, yearWord(years))
	default:
		text = fmt.Sprintf("The %s sign %s: %s a year for %d %s.", lg.teamName(teamID), p.Name, fmtM(salary), years, yearWord(years))
	}
	big := (how == "signs" && p.Overall() >= NewsTuning.BigOvr) || (user && lg.notable(p))
	lg.addNews("", NwSigning, text, big, []int{teamID}, p)
}

func yearWord(n int) string {
	if n == 1 {
		return "year"
	}
	return "years"
}

// newsRelease writes the story when a team cuts a player.
func (lg *League) newsRelease(teamID int, p *Player) {
	user := teamID == lg.UserTeamID
	if !lg.notable(p) && !(user && (!NewsTuning.UserMinSalaryOnly || p.Contract.Salary > Cap.MinSalary)) {
		return
	}
	lg.addNews("", NwRelease, fmt.Sprintf("The %s release %s.", lg.teamName(teamID), p.Name),
		p.Overall() >= NewsTuning.BigOvr, []int{teamID}, p)
}

// ---- injuries and trade requests ----

// newsInjury is called when a player is hurt in a game (p.Injury is set).
func (lg *League) newsInjury(p *Player) {
	t := &NewsTuning
	if p.Injury == nil || p.Injury.GamesLeft < t.InjuryMinGames {
		return
	}
	notable := lg.notable(p)
	if !notable && p.TeamID != lg.UserTeamID {
		return
	}
	text := fmt.Sprintf("%s of the %s is out about %d games with a %s.", p.Name, lg.teamName(p.TeamID),
		p.Injury.GamesLeft, strings.ToLower(p.Injury.Name))
	lg.addNews("", NwInjury, text, notable && p.Injury.GamesLeft >= t.InjuryBigGames, []int{p.TeamID}, p)
}

// newsRequest is called next to logEvent for trade requests and withdrawals.
func (lg *League) newsRequest(typ string, p *Player, teamID int) {
	if !lg.notable(p) && teamID != lg.UserTeamID {
		return
	}
	switch typ {
	case EvTradeRequest:
		lg.addNews("", NwRequest, fmt.Sprintf("%s has asked the %s to trade him.", p.Name, lg.teamName(teamID)),
			p.Overall() >= NewsTuning.BigOvr, []int{teamID}, p)
	case EvRequestWithdrawn:
		lg.addNews("", NwRequest, fmt.Sprintf("%s has withdrawn his trade request.", p.Name), false, []int{teamID}, p)
	}
}

// ---- games and streaks ----

// newsGame writes a story for each big scoring night in the game just played.
func (lg *League) newsGame(res *GameResult) {
	t := &NewsTuning
	for i, tb := range []*TeamBox{res.Home, res.Away} {
		opp := res.Away
		if i == 1 {
			opp = res.Home
		}
		mine, theirs := tb.Score, opp.Score
		for _, l := range tb.Lines {
			if l.Pts < t.BigGamePts {
				continue
			}
			result := "a win"
			if mine < theirs {
				result = "a loss"
			}
			text := fmt.Sprintf("%s scored %d points for the %s in %s against the %s.", l.Player.Name, l.Pts,
				tb.Team.FullName(), result, opp.Team.FullName())
			lg.addNews("", NwGame, text, l.Pts >= t.BigGameBig, []int{tb.Team.ID, opp.Team.ID}, l.Player)
		}
	}
}

// newsStreaks is called after a regular-season day's games are played (before
// the day counter moves). It derives each team's streak from the played games.
func (lg *League) newsStreaks() {
	s := lg.Season
	day := s.Day
	t := &NewsTuning
	result := func(team, d int) (win, ok bool) {
		for i := d * GamesPerDay; i < (d+1)*GamesPerDay && i < len(s.Games); i++ {
			g := &s.Games[i]
			if !g.Played || (g.Home != team && g.Away != team) {
				continue
			}
			return (g.Home == team) == (g.HomeScore > g.AwayScore), true
		}
		return false, false
	}
	for team := 0; team < NumTeams; team++ {
		win, ok := result(team, day)
		if !ok {
			continue
		}
		n := 1
		for d := day - 1; d >= 0; d-- {
			w, ok := result(team, d)
			if !ok || w != win {
				break
			}
			n++
		}
		list := t.LoseStreaks
		if win {
			list = t.WinStreaks
		}
		hit := false
		for _, k := range list {
			hit = hit || k == n
		}
		if !hit {
			continue
		}
		name := lg.teamName(team)
		if win {
			lg.addNews("", NwStreak, fmt.Sprintf("The %s have won %d games in a row.", name, n), n >= t.BigWinStreak, []int{team})
		} else {
			lg.addNews("", NwStreak, fmt.Sprintf("The %s have lost %d games in a row.", name, n), false, []int{team})
		}
	}
}

// ---- playoffs ----

var reachNames = []string{"", "the conference semifinals", "the conference finals", "the Finals"}

// newsSeries is called when a playoff series ends.
func (lg *League) newsSeries(sr *Series) {
	w, l := sr.Winner, sr.High+sr.Low-sr.Winner
	wins, losses := max(sr.HighWins, sr.LowWins), min(sr.HighWins, sr.LowWins)
	if sr.Round >= 4 {
		lg.addNews("playoffs", NwPlayoffs, fmt.Sprintf("The %s are the %d champions, beating the %s %d-%d in the Finals.",
			lg.teamName(w), lg.Year, lg.teamName(l), wins, losses), true, []int{w, l})
		return
	}
	lg.addNews("playoffs", NwPlayoffs, fmt.Sprintf("The %s beat the %s %d-%d to reach %s.",
		lg.teamName(w), lg.teamName(l), wins, losses, reachNames[sr.Round]), false, []int{w, l})
}

// newsFinalsMVP is called when the Finals MVP is decided.
func (lg *League) newsFinalsMVP() {
	a := lg.Season.Awards.FinalsMVP
	if a.Name == "" {
		return
	}
	lg.addNews("playoffs", NwPlayoffs, fmt.Sprintf("%s of the %s is the Finals MVP (%.1f points, %.1f rebounds, %.1f assists over the series).",
		a.Name, lg.teamName(a.TeamID), a.PPG, a.RPG, a.APG), false, []int{a.TeamID}, &Player{ID: a.PlayerID, Name: a.Name})
}

// ---- awards ----

func (lg *League) awardStory(aw Award, title string, big bool) {
	if aw.Name == "" {
		return
	}
	lg.addNews("", NwAward, fmt.Sprintf("%s of the %s wins %s, averaging %.1f points, %.1f rebounds and %.1f assists.",
		aw.Name, lg.teamName(aw.TeamID), title, aw.PPG, aw.RPG, aw.APG), big, []int{aw.TeamID}, &Player{ID: aw.PlayerID, Name: aw.Name})
}

// newsAwards writes one story per award decided at the end of the regular season.
func (lg *League) newsAwards() {
	a := lg.Season.Awards
	y := lg.Year
	lg.awardStory(a.MVP, fmt.Sprintf("the %d MVP award", y), true)
	if a.ScoringLeader.PlayerID != a.MVP.PlayerID {
		lg.awardStory(a.ScoringLeader, fmt.Sprintf("the %d scoring title", y), false)
	}
	lg.awardStory(a.ROY, "Rookie of the Year", false)
	lg.awardStory(a.DPOY, "Defensive Player of the Year", false)
	lg.awardStory(a.SixthMan, "Sixth Man of the Year", false)
	lg.awardStory(a.MIP, "Most Improved Player", false)
	if c := a.Coach; c.Name != "" {
		lg.addNews("", NwAward, fmt.Sprintf("%s of the %s is Coach of the Year after a %d-%d season.",
			c.Name, lg.teamName(c.TeamID), c.W, c.L), false, []int{c.TeamID})
	}
	if len(a.AllLeague) > 0 {
		var names []string
		var ps []*Player
		var teams []int
		for _, aw := range a.AllLeague[0] {
			names = append(names, aw.Name)
			ps = append(ps, &Player{ID: aw.PlayerID, Name: aw.Name})
			teams = append(teams, aw.TeamID)
		}
		lg.addNews("", NwAward, fmt.Sprintf("The All-League first team: %s.", nameList(names)), false, teams, ps...)
	}
}

// ---- draft ----

// newsLottery is called when the lottery has been drawn.
func (lg *League) newsLottery() {
	d := lg.Draft
	if d == nil {
		return
	}
	for _, row := range d.Lottery {
		if row.Slot == 1 {
			lg.addNews("draft", NwDraft, fmt.Sprintf("The %s win the draft lottery and will pick first.", lg.teamName(row.Owner)),
				row.Owner == lg.UserTeamID, []int{row.Owner})
		}
	}
}

// newsPick is called after each draft pick. Only name and pick number are told.
func (lg *League) newsPick(pr PickResult) {
	user := pr.TeamID == lg.UserTeamID
	if pr.Pick > NewsTuning.DraftTop && !user {
		return
	}
	text := fmt.Sprintf("The %s select %s with pick number %d.", lg.teamName(pr.TeamID), pr.Name, pr.Pick)
	lg.addNews("draft", NwDraft, text, pr.Pick == 1 || (user && pr.Round == 1 && pr.Pick <= 10), []int{pr.TeamID},
		&Player{ID: pr.PlayerID, Name: pr.Name})
}

// ---- retirements, coaches, league ----

// newsRetire is called for each player who retires at the start of the offseason.
func (lg *League) newsRetire(p *Player) {
	t := &NewsTuning
	peak := p.Overall()
	for _, r := range lg.Careers[p.ID] {
		peak = max(peak, r.Overall)
	}
	if peak < t.RetireOvr && len(lg.PlayerAwards(p.ID)) == 0 {
		return
	}
	text := fmt.Sprintf("%s retires at %d after %d seasons.", p.Name, p.Age, p.Experience)
	if p.TeamID >= 0 {
		text = fmt.Sprintf("%s retires at %d after %d seasons, last with the %s.", p.Name, p.Age, p.Experience, lg.teamName(p.TeamID))
	}
	lg.addNews("offseason", NwRetire, text, peak >= t.RetireBigOvr, []int{p.TeamID}, p)
}

// newsCoach is called when a team hires a head coach. old may be nil.
func (lg *League) newsCoach(teamID int, old, hired *Coach) {
	if hired == nil {
		return
	}
	text := fmt.Sprintf("The %s hire %s as head coach.", lg.teamName(teamID), hired.Name)
	if old != nil {
		text = fmt.Sprintf("The %s replace coach %s with %s.", lg.teamName(teamID), old.Name, hired.Name)
	}
	lg.addNews("", NwCoach, text, false, []int{teamID})
}

// newsLeague writes a league-level story (a season starting, the user's job).
func (lg *League) newsLeague(text string, big bool, teams ...int) {
	lg.addNews("", NwLeague, text, big, teams)
}

// ---- reading the feed ----

func newsInFilter(it *NewsItem, filter string, user int) bool {
	switch filter {
	case "all":
		return true
	case "mine":
		for _, t := range it.TeamIDs {
			if t == user {
				return true
			}
		}
		return false
	case "trades":
		return it.Kind == NwTrade
	case "signings":
		return it.Kind == NwSigning || it.Kind == NwRelease
	case "injuries":
		return it.Kind == NwInjury
	case "games":
		return it.Kind == NwGame || it.Kind == NwStreak || it.Kind == NwPlayoffs
	case "awards":
		return it.Kind == NwAward || it.Kind == NwRetire
	case "draft":
		return it.Kind == NwDraft
	}
	return false
}

// NewsFor returns up to limit stories matching filter, newest first, the count
// of stories under every filter key, and the number of matches before the limit.
// An unknown filter is treated as "all" (callers that want an error check
// ValidNewsFilter first). limit <= 0 means no limit.
func (lg *League) NewsFor(filter string, limit int) (items []NewsItem, counts map[string]int, total int) {
	if !ValidNewsFilter(filter) {
		filter = "all"
	}
	counts = map[string]int{}
	for _, k := range newsFilterKeys {
		counts[k] = 0
	}
	items = []NewsItem{}
	for i := len(lg.News) - 1; i >= 0; i-- {
		it := &lg.News[i]
		for _, k := range newsFilterKeys {
			if newsInFilter(it, k, lg.UserTeamID) {
				counts[k]++
			}
		}
		if newsInFilter(it, filter, lg.UserTeamID) {
			total++
			if limit <= 0 || len(items) < limit {
				items = append(items, *it)
			}
		}
	}
	return items, counts, total
}
