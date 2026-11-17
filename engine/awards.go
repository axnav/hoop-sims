package engine

import (
	"fmt"
	"sort"
)

// Season awards beyond MVP, scoring leader and Finals MVP (those stay in
// season.go and playoffs.go). Everything here is computed once, at the end of
// the regular season, from visible statistics and visible ratings only: the
// hidden Impact is never read, and no random number is drawn.
//
// Ties are always broken by the lower player (or team) ID, and candidates are
// visited in ID order, so a result never depends on map iteration order.

// AwardTuning holds every number behind the awards. Edit here.
var AwardTuning = struct {
	// Minimum games played (the rookie awards use RookieMinGames).
	MinGames, RookieMinGames int
	// Production score = PPG + RebW * RPG + AstW * APG (the same weights as MVP).
	RebW, AstW float64
	// Defensive Player of the Year: score = (Defense + DPOYRebW * Rebounding) *
	// min(1, minutes per game / DPOYFullMinutes) + DPOYTeamW * (league mean points
	// allowed per game - his team's). Needs DPOYMinMPG minutes per game.
	DPOYRebW, DPOYFullMinutes, DPOYTeamW, DPOYMinMPG float64
	// Sixth Man: not among his team's SixthTop players by minutes per game,
	// at least SixthMinMPG minutes per game; best production score.
	SixthTop    int
	SixthMinMPG float64
	// Most Improved: production score this season minus last season's (needs
	// MinGames both seasons); the gain must be at least MIPMinGain.
	MIPMinGain float64
	// Coach of the Year: wins + CoachImproveW * (wins - last season's wins);
	// the first recorded season counts wins only.
	CoachImproveW float64
	// All-League teams and the All-Rookie team.
	AllLeagueTeams, AllRookieSize int
}{
	MinGames: 50, RookieMinGames: 30,
	RebW: 0.5, AstW: 0.7,
	DPOYRebW: 0.2, DPOYFullMinutes: 32, DPOYTeamW: 1.5, DPOYMinMPG: 20,
	SixthTop: 5, SixthMinMPG: 15,
	MIPMinGain:     2,
	CoachImproveW:  0.6,
	AllLeagueTeams: 3, AllRookieSize: 5,
}

// CoachAward is the Coach of the Year.
type CoachAward struct {
	Name         string
	TeamID, W, L int
}

// PlayerAward is one honour in a player's list (see PlayerAwards).
type PlayerAward struct {
	Year  int
	Title string
}

func prodScore(ppg, rpg, apg float64) float64 {
	return ppg + AwardTuning.RebW*rpg + AwardTuning.AstW*apg
}

func (ps *PlayerSeason) prod() float64 { return prodScore(ps.PPG(), ps.RPG(), ps.APG()) }

// isGuard, isForward: the three position groups of an All-League team.
func posGroup(p Position) int {
	switch p {
	case PG, SG:
		return 0
	case SF, PF:
		return 1
	}
	return 2
}

// seasonRookie: a player in his first season (drafted last offseason or
// later, no finished season on record). The initial league's players have
// DraftYear 0 and are not rookies.
func (lg *League) seasonRookie(p *Player, s *Season) bool {
	return p != nil && p.DraftYear != 0 && p.DraftYear >= s.Year-1 && len(lg.Careers[p.ID]) == 0
}

// cand is one scored award candidate.
type cand struct {
	ps    *PlayerSeason
	p     *Player
	score float64
}

func sortCands(cs []cand) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].score != cs[j].score {
			return cs[i].score > cs[j].score
		}
		return cs[i].ps.PlayerID < cs[j].ps.PlayerID
	})
}

// statPlayers returns the season's stat lines in player ID order, each with
// his Player (skipped when he cannot be found).
func (lg *League) statPlayers(s *Season) []cand {
	byID := map[int]*Player{}
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			byID[p.ID] = p
		}
	}
	for _, p := range lg.FreeAgents {
		byID[p.ID] = p
	}
	ids := make([]int, 0, len(s.Stats))
	for id := range s.Stats {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	var out []cand
	for _, id := range ids {
		if p := byID[id]; p != nil {
			out = append(out, cand{ps: s.Stats[id], p: p})
		}
	}
	return out
}

// computeLeagueAwards decides the awards that need the league (rookies,
// ratings, career rows, coaches), then writes their news. Called at the end of
// the regular season, right after computeAwards.
func (lg *League) computeLeagueAwards() {
	s := lg.Season
	t := &AwardTuning
	all := lg.statPlayers(s)

	// Points allowed per game by team, from the played games.
	pa := make([]float64, NumTeams)
	gp := make([]int, NumTeams)
	for _, g := range s.Games {
		if !g.Played {
			continue
		}
		pa[g.Home] += float64(g.AwayScore)
		pa[g.Away] += float64(g.HomeScore)
		gp[g.Home]++
		gp[g.Away]++
	}
	meanPA, n := 0.0, 0
	for i := range pa {
		if gp[i] > 0 {
			pa[i] /= float64(gp[i])
			meanPA += pa[i]
			n++
		}
	}
	if n > 0 {
		meanPA /= float64(n)
	}

	// Minutes-per-game rank inside each team (for the Sixth Man).
	byTeam := map[int][]cand{}
	for _, c := range all {
		byTeam[c.ps.TeamID] = append(byTeam[c.ps.TeamID], c)
	}
	top := map[int]bool{}
	for _, cs := range byTeam {
		sort.Slice(cs, func(i, j int) bool {
			a, b := mpg(cs[i].ps), mpg(cs[j].ps)
			if a != b {
				return a > b
			}
			return cs[i].ps.PlayerID < cs[j].ps.PlayerID
		})
		for i := 0; i < len(cs) && i < t.SixthTop; i++ {
			top[cs[i].ps.PlayerID] = true
		}
	}

	var rookies, dpoy, sixth, mip, mvpStyle []cand
	for _, c := range all {
		ps, p := c.ps, c.p
		if ps.GP == 0 || ps.TeamID < 0 || ps.TeamID >= NumTeams {
			continue
		}
		if lg.seasonRookie(p, s) {
			if ps.GP >= t.RookieMinGames {
				rookies = append(rookies, cand{ps, p, ps.prod()})
			}
			continue
		}
		if ps.GP < t.MinGames {
			continue
		}
		wp := float64(s.Wins[ps.TeamID]) / float64(max(1, s.Wins[ps.TeamID]+s.Losses[ps.TeamID]))
		mvpStyle = append(mvpStyle, cand{ps, p, ps.prod() * (0.5 + wp)})
		if m := mpg(ps); m >= t.DPOYMinMPG {
			def := (float64(p.Ratings.Defense) + t.DPOYRebW*float64(p.Ratings.Rebounding)) * minF(1, m/t.DPOYFullMinutes)
			dpoy = append(dpoy, cand{ps, p, def + t.DPOYTeamW*(meanPA-pa[ps.TeamID])})
		}
		if !top[ps.PlayerID] && mpg(ps) >= t.SixthMinMPG {
			sixth = append(sixth, cand{ps, p, ps.prod()})
		}
		if rows := lg.Careers[p.ID]; len(rows) > 0 {
			if last := rows[len(rows)-1]; last.Year == s.Year-1 && last.GP >= t.MinGames {
				if gain := ps.prod() - prodScore(last.PPG, last.RPG, last.APG); gain >= t.MIPMinGain {
					mip = append(mip, cand{ps, p, gain})
				}
			}
		}
	}
	// Rookies compete for the All-League teams too if good enough; they are
	// added after the loop's rookie branch kept them out of mvpStyle, so add
	// the rookies who meet the full minimum.
	for _, c := range all {
		if c.ps.GP >= t.MinGames && c.ps.TeamID >= 0 && c.ps.TeamID < NumTeams && lg.seasonRookie(c.p, s) {
			wp := float64(s.Wins[c.ps.TeamID]) / float64(max(1, s.Wins[c.ps.TeamID]+s.Losses[c.ps.TeamID]))
			mvpStyle = append(mvpStyle, cand{c.ps, c.p, c.ps.prod() * (0.5 + wp)})
		}
	}

	a := &s.Awards
	pick := func(cs []cand) Award {
		if len(cs) == 0 {
			return Award{}
		}
		sortCands(cs)
		return awardOf(cs[0].ps)
	}
	a.ROY, a.DPOY, a.SixthMan, a.MIP = pick(rookies), pick(dpoy), pick(sixth), pick(mip)

	sortCands(rookies)
	a.AllRookie = nil
	for i := 0; i < len(rookies) && i < t.AllRookieSize; i++ {
		a.AllRookie = append(a.AllRookie, awardOf(rookies[i].ps))
	}

	// All-League: 2 guards, 2 forwards, 1 centre per team, best score first;
	// a group that runs short is filled by the best player left.
	sortCands(mvpStyle)
	used := map[int]bool{}
	a.AllLeague = nil
	need := [3]int{2, 2, 1}
	for k := 0; k < t.AllLeagueTeams; k++ {
		var team [3][]Award
		left := need
		for _, c := range mvpStyle {
			if used[c.ps.PlayerID] {
				continue
			}
			g := posGroup(c.p.Pos)
			if left[g] > 0 {
				left[g]--
				used[c.ps.PlayerID] = true
				team[g] = append(team[g], awardOf(c.ps))
			}
		}
		for g := 0; g < 3; g++ { // fill gaps from the best unused player
			for left[g] > 0 {
				found := false
				for _, c := range mvpStyle {
					if !used[c.ps.PlayerID] {
						used[c.ps.PlayerID] = true
						team[g] = append(team[g], awardOf(c.ps))
						left[g]--
						found = true
						break
					}
				}
				if !found {
					break
				}
			}
		}
		row := append(append(append([]Award{}, team[0]...), team[1]...), team[2]...)
		if len(row) == 0 {
			break
		}
		a.AllLeague = append(a.AllLeague, row)
	}

	// Coach of the Year.
	var prev []int
	if n := len(lg.Records); n > 0 {
		prev = lg.Records[n-1].Wins
	}
	best, bestTeam := -1e9, -1
	for _, tm := range lg.Teams {
		if tm.Coach == nil || tm.ID >= NumTeams {
			continue
		}
		w := float64(s.Wins[tm.ID])
		v := w
		if prev != nil && tm.ID < len(prev) {
			v += t.CoachImproveW * (w - float64(prev[tm.ID]))
		}
		if v > best {
			best, bestTeam = v, tm.ID
		}
	}
	if bestTeam >= 0 {
		a.Coach = CoachAward{Name: lg.Teams[bestTeam].Coach.Name, TeamID: bestTeam, W: s.Wins[bestTeam], L: s.Losses[bestTeam]}
	}
	lg.newsAwards()
}

func mpg(ps *PlayerSeason) float64 {
	if ps.GP == 0 {
		return 0
	}
	return float64(ps.Min) / float64(ps.GP)
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// awardRows lists the titles a set of awards gives to one player, in a fixed order.
func awardTitles(a *Awards, id int) []string {
	var out []string
	add := func(aw Award, title string) {
		if aw.Name != "" && aw.PlayerID == id {
			out = append(out, title)
		}
	}
	add(a.MVP, "MVP")
	add(a.FinalsMVP, "Finals MVP")
	add(a.ScoringLeader, "Scoring leader")
	add(a.ROY, "Rookie of the Year")
	add(a.DPOY, "Defensive Player of the Year")
	add(a.SixthMan, "Sixth Man of the Year")
	add(a.MIP, "Most Improved Player")
	for i, tm := range a.AllLeague {
		for _, aw := range tm {
			if aw.PlayerID == id {
				out = append(out, fmt.Sprintf("All-League %s Team", [...]string{"First", "Second", "Third"}[min(i, 2)]))
			}
		}
	}
	for _, aw := range a.AllRookie {
		if aw.PlayerID == id {
			out = append(out, "All-Rookie Team")
		}
	}
	return out
}

// PlayerAwards lists a player's honours, newest season first. It is derived
// from the season records plus the current season once its awards are decided;
// nothing extra is saved.
func (lg *League) PlayerAwards(playerID int) []PlayerAward {
	out := []PlayerAward{}
	if y, a, ok := lg.CurrentAwards(); ok {
		for _, t := range awardTitles(&a, playerID) {
			out = append(out, PlayerAward{y, t})
		}
	}
	for i := len(lg.Records) - 1; i >= 0; i-- {
		rec := lg.Records[i]
		a := Awards{MVP: rec.MVP, ScoringLeader: rec.ScoringLeader, FinalsMVP: rec.FinalsMVP, ROY: rec.ROY,
			DPOY: rec.DPOY, SixthMan: rec.SixthMan, MIP: rec.MIP, AllLeague: rec.AllLeague, AllRookie: rec.AllRookie}
		for _, t := range awardTitles(&a, playerID) {
			out = append(out, PlayerAward{rec.Year, t})
		}
	}
	return out
}

// CurrentAwards returns the current season's awards once the regular season is
// over and the season is not yet in the records.
func (lg *League) CurrentAwards() (year int, a Awards, ok bool) {
	s := lg.Season
	if s == nil || !s.Done() || s.Awards.MVP.Name == "" {
		return 0, Awards{}, false
	}
	for _, rec := range lg.Records {
		if rec.Year == s.Year {
			return 0, Awards{}, false
		}
	}
	return s.Year, s.Awards, true
}
