package engine

import "sort"

const (
	NumTeams     = 30
	GamesPerDay  = NumTeams / 2
	SeasonGames  = 82
	SeasonDays   = SeasonGames // every round is one day
	MinGPLeaders = 50
)

// Conference returns 0 for teams 0-14 and 1 for teams 15-29.
func Conference(teamID int) int { return teamID / (NumTeams / 2) }

type Game struct {
	Day        int
	Home, Away int // team IDs
	Played     bool
	HomeScore  int
	AwayScore  int
}

// PlayerSeason holds one player's regular-season totals.
type PlayerSeason struct {
	PlayerID                     int
	Name                         string
	TeamID                       int
	GP, Min                      int
	Pts, Reb, Ast, Tov           int
	FGM, FGA, TPM, TPA, FTM, FTA int
}

func (p *PlayerSeason) per(v int) float64 {
	if p.GP == 0 {
		return 0
	}
	return float64(v) / float64(p.GP)
}
func (p *PlayerSeason) PPG() float64 { return p.per(p.Pts) }
func (p *PlayerSeason) RPG() float64 { return p.per(p.Reb) }
func (p *PlayerSeason) APG() float64 { return p.per(p.Ast) }

// Award records a season award winner with the per-game line at the time.
type Award struct {
	PlayerID      int
	Name          string
	TeamID        int
	GP            int
	PPG, RPG, APG float64
}

type Awards struct {
	MVP           Award
	ScoringLeader Award
	FinalsMVP     Award
}

type Season struct {
	Year      int
	Games     []Game // ordered by day, GamesPerDay per day
	Day       int    // next day to play
	Wins      []int
	Losses    []int
	PointDiff []int
	Stats     map[int]*PlayerSeason
	Series    []Series // the playoff bracket, flat; see playoffs.go
	Champion  int      // team ID, -1 until playoffs are done
	Awards    Awards

	PlayoffsStarted bool
	PlayoffDay      int // playoff days played so far (also the RNG stream)
	PlayoffGames    int // playoff games played so far (box score IDs)
	PlayoffStats    map[int]*PlayerSeason
	PlayoffBoxes    map[int]*Box // every playoff game of this postseason, with play-by-play
}

func (lg *League) dayRNG(year, day int) *RNG {
	return NewRNG(lg.Seed*1000003 + uint64(year)*10007 + uint64(day))
}

// Offsets keeping playoff and offseason streams apart from regular-season days.
const (
	playoffOffset   = 1000
	offseasonOffset = 5000
	scheduleOffset  = 9000
)

// StartSeason builds the schedule for the league's current year.
func (lg *League) StartSeason() *Season {
	s := &Season{
		Year: lg.Year, Champion: -1,
		Wins: make([]int, NumTeams), Losses: make([]int, NumTeams), PointDiff: make([]int, NumTeams),
		Stats: map[int]*PlayerSeason{}, PlayoffStats: map[int]*PlayerSeason{}, PlayoffBoxes: map[int]*Box{},
	}
	r := lg.dayRNG(lg.Year, scheduleOffset)
	order := r.Perm(NumTeams)
	n := NumTeams
	round := func(k int) [][2]int {
		arr := make([]int, n)
		arr[0] = order[0]
		for i := 1; i < n; i++ {
			arr[i] = order[1+(i-1+k)%(n-1)]
		}
		var ps [][2]int
		for i := 0; i < n/2; i++ {
			a, b := arr[i], arr[n-1-i]
			if (k+i)%2 == 1 {
				a, b = b, a
			}
			ps = append(ps, [2]int{a, b})
		}
		return ps
	}
	day := 0
	add := func(h, a int) {
		s.Games = append(s.Games, Game{Day: day, Home: h, Away: a})
	}
	for pass := 0; pass < 3; pass++ {
		rounds := n - 1
		if pass == 2 {
			rounds = SeasonGames - 2*(n-1)
		}
		bal := make([]int, n) // home games minus away games
		for k := 0; k < rounds; k++ {
			for _, pr := range round(k) {
				h, a := pr[0], pr[1]
				switch pass {
				case 1:
					h, a = a, h
				case 2:
					if bal[a] < bal[h] {
						h, a = a, h
					}
				}
				bal[h]++
				bal[a]--
				add(h, a)
			}
			day++
		}
	}
	lg.Season = s
	return s
}

func addStats(m map[int]*PlayerSeason, res *GameResult) {
	for _, tb := range []*TeamBox{res.Home, res.Away} {
		for _, l := range tb.Lines {
			if l.Min == 0 {
				continue
			}
			ps := m[l.Player.ID]
			if ps == nil {
				ps = &PlayerSeason{PlayerID: l.Player.ID, Name: l.Player.Name, TeamID: tb.Team.ID}
				m[l.Player.ID] = ps
			}
			ps.TeamID = tb.Team.ID // a traded player's line follows him to his latest team
			ps.GP++
			ps.Min += l.Min
			ps.Pts += l.Pts
			ps.Reb += l.Reb
			ps.Ast += l.Ast
			ps.Tov += l.Tov
			ps.FGM += l.FGM
			ps.FGA += l.FGA
			ps.TPM += l.TPM
			ps.TPA += l.TPA
			ps.FTM += l.FTM
			ps.FTA += l.FTA
		}
	}
}

func (s *Season) Done() bool { return s.Day >= SeasonDays }

// SimDay plays the next day of the regular season. Returns false if finished.
func (lg *League) SimDay() bool {
	s := lg.Season
	if s.Done() {
		return false
	}
	lg.newProposals = 0
	lg.newInjuries = nil
	lg.newDev = nil
	lg.bmeans = nil
	ir := lg.dayRNG(s.Year, injuryOffset+s.Day)
	r := lg.dayRNG(s.Year, s.Day)
	for i := s.Day * GamesPerDay; i < (s.Day+1)*GamesPerDay; i++ {
		g := &s.Games[i]
		user := g.Home == lg.UserTeamID || g.Away == lg.UserTeamID
		res := lg.SimGameRec(r, lg.Teams[g.Home], lg.Teams[g.Away], user)
		g.Played, g.HomeScore, g.AwayScore = true, res.Home.Score, res.Away.Score
		if g.HomeScore > g.AwayScore {
			s.Wins[g.Home]++
			s.Losses[g.Away]++
		} else {
			s.Wins[g.Away]++
			s.Losses[g.Home]++
		}
		lg.recordBox(s.Day, res)
		s.PointDiff[g.Home] += g.HomeScore - g.AwayScore
		s.PointDiff[g.Away] += g.AwayScore - g.HomeScore
		addStats(s.Stats, res)
		lg.afterGame(ir, res)
	}
	s.Day++
	if c := &Progression; !lg.Options.NoInSeasonDev && s.Day%c.InSeasonEvery == 0 && s.Day/c.InSeasonEvery <= c.InSeasonSteps {
		lg.inSeasonDev(s.Day / c.InSeasonEvery)
	}
	if s.Day%7 == 0 {
		lg.moraleWeek()
	}
	if s.Done() {
		s.computeAwards()
		lg.awardPerfBadges()
	}
	lg.updateProposals()
	return true
}

func (lg *League) SimDays(n int) {
	for i := 0; i < n && lg.SimDay(); i++ {
	}
}

func (lg *League) SimRegularSeason() {
	for lg.SimDay() {
	}
}

// Standings returns team IDs sorted best-first (wins, then point diff, then ID).
func (s *Season) Standings() []int {
	ids := make([]int, NumTeams)
	for i := range ids {
		ids[i] = i
	}
	sort.SliceStable(ids, func(a, b int) bool {
		x, y := ids[a], ids[b]
		if s.Wins[x] != s.Wins[y] {
			return s.Wins[x] > s.Wins[y]
		}
		return s.PointDiff[x] > s.PointDiff[y]
	})
	return ids
}

// ConferenceStandings is Standings filtered to one conference.
func (s *Season) ConferenceStandings(conf int) []int {
	var out []int
	for _, id := range s.Standings() {
		if Conference(id) == conf {
			out = append(out, id)
		}
	}
	return out
}

// Leaders returns the top n players by per-game average of stat
// ("pts", "reb" or "ast"), minimum MinGPLeaders games.
func (s *Season) Leaders(stat string, n int) []*PlayerSeason {
	var rows []*PlayerSeason
	for _, ps := range s.Stats {
		if ps.GP >= MinGPLeaders {
			rows = append(rows, ps)
		}
	}
	val := func(p *PlayerSeason) float64 {
		switch stat {
		case "reb":
			return p.RPG()
		case "ast":
			return p.APG()
		}
		return p.PPG()
	}
	sort.Slice(rows, func(i, j int) bool {
		if vi, vj := val(rows[i]), val(rows[j]); vi != vj {
			return vi > vj
		}
		return rows[i].PlayerID < rows[j].PlayerID
	})
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows
}

func awardOf(p *PlayerSeason) Award {
	return Award{p.PlayerID, p.Name, p.TeamID, p.GP, p.PPG(), p.RPG(), p.APG()}
}

func (s *Season) computeAwards() {
	var mvp *PlayerSeason
	best := -1.0
	for _, ps := range s.Stats {
		if ps.GP < MinGPLeaders {
			continue
		}
		wp := float64(s.Wins[ps.TeamID]) / float64(s.Wins[ps.TeamID]+s.Losses[ps.TeamID])
		v := (ps.PPG() + 0.5*ps.RPG() + 0.7*ps.APG()) * (0.5 + wp)
		if v > best || (v == best && ps.PlayerID < mvp.PlayerID) {
			best, mvp = v, ps
		}
	}
	if mvp != nil {
		s.Awards.MVP = awardOf(mvp)
	}
	if l := s.Leaders("pts", 1); len(l) > 0 {
		s.Awards.ScoringLeader = awardOf(l[0])
	}
}

// statsFor returns a player's regular-season totals, nil if none (or no season).
func (s *Season) statsFor(id int) *PlayerSeason {
	if s == nil {
		return nil
	}
	return s.Stats[id]
}
