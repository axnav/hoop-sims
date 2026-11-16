package engine

import (
	"fmt"
	"sort"
)

const homePattern = "HHAAHAH" // H = better seed hosts

// PGame is one played playoff game.
type PGame struct {
	N         int // game number within the series, from 1
	Home      int // team ID
	HomeScore int
	AwayScore int
	OT        int
	Day       int // playoff day
	Box       int // key in Season.PlayoffBoxes
}

// Series is one playoff series. Round 1-3 are inside a conference, 4 is the
// finals (Conf -1). Slot orders series inside a round and conference; round-1
// slots are 1v8, 4v5, 2v7, 3v6 so slots 2k and 2k+1 feed slot k of the next round.
// High is the team with home-court advantage.
type Series struct {
	Round, Conf, Slot int
	High, Low         int
	HighSeed, LowSeed int
	HighWins, LowWins int
	Winner            int // -1 until Done
	Done              bool
	Games             []PGame
}

var roundNames = []string{"", "Round 1", "Conference Semifinals", "Conference Finals", "Finals"}

func RoundName(r int) string {
	if r < 1 || r > 4 {
		return ""
	}
	return roundNames[r]
}

func (s *Season) seedOf(team int) int {
	for _, sr := range s.Series {
		if sr.Round == 1 {
			if sr.High == team {
				return sr.HighSeed
			}
			if sr.Low == team {
				return sr.LowSeed
			}
		}
	}
	return 0
}

func (s *Season) newSeries(round, conf, slot, a, b int) {
	sa, sb := s.seedOf(a), s.seedOf(b)
	hi, lo, hs, ls := a, b, sa, sb
	swap := sb < sa
	if round == 4 {
		rank := map[int]int{}
		for i, id := range s.Standings() {
			rank[id] = i
		}
		swap = rank[b] < rank[a]
	}
	if swap {
		hi, lo, hs, ls = b, a, sb, sa
	}
	s.Series = append(s.Series, Series{Round: round, Conf: conf, Slot: slot, High: hi, Low: lo,
		HighSeed: hs, LowSeed: ls, Winner: -1})
}

// StartPlayoffs seeds the bracket (top 8 per conference).
func (lg *League) StartPlayoffs() error {
	s := lg.Season
	if !s.Done() {
		return fmt.Errorf("regular season not finished")
	}
	if s.PlayoffsStarted {
		return fmt.Errorf("playoffs already started")
	}
	s.PlayoffsStarted = true
	s.Series = nil
	for conf := 0; conf < 2; conf++ {
		seeds := s.ConferenceStandings(conf)[:8]
		for slot, pr := range [][2]int{{0, 7}, {3, 4}, {1, 6}, {2, 5}} {
			sr := Series{Round: 1, Conf: conf, Slot: slot, High: seeds[pr[0]], Low: seeds[pr[1]],
				HighSeed: pr[0] + 1, LowSeed: pr[1] + 1, Winner: -1}
			s.Series = append(s.Series, sr)
		}
	}
	return nil
}

// CurrentRound is the lowest round with an unfinished series (0 if none).
func (s *Season) CurrentRound() int {
	r := 0
	for _, sr := range s.Series {
		if !sr.Done && (r == 0 || sr.Round < r) {
			r = sr.Round
		}
	}
	return r
}

// PlayoffsActive reports whether playoff games remain to be played.
func (s *Season) PlayoffsActive() bool { return s.PlayoffsStarted && s.Champion < 0 }

// UserSeriesIdx is the index of the user team's unfinished series in the
// current round, or -1.
func (lg *League) UserSeriesIdx() int {
	s := lg.Season
	r := s.CurrentRound()
	for i, sr := range s.Series {
		if sr.Round == r && !sr.Done && (sr.High == lg.UserTeamID || sr.Low == lg.UserTeamID) {
			return i
		}
	}
	return -1
}

func (s *Season) winnerOf(round, conf, slot int) int {
	for _, sr := range s.Series {
		if sr.Round == round && sr.Conf == conf && sr.Slot == slot {
			return sr.Winner
		}
	}
	return -1
}

// advance creates the next round once every series of the current one is done.
func (s *Season) advance() {
	if s.CurrentRound() != 0 || len(s.Series) == 0 {
		return
	}
	last := s.Series[len(s.Series)-1].Round
	switch last {
	case 1:
		for conf := 0; conf < 2; conf++ {
			for k := 0; k < 2; k++ {
				s.newSeries(2, conf, k, s.winnerOf(1, conf, 2*k), s.winnerOf(1, conf, 2*k+1))
			}
		}
	case 2:
		for conf := 0; conf < 2; conf++ {
			s.newSeries(3, conf, 0, s.winnerOf(2, conf, 0), s.winnerOf(2, conf, 1))
		}
	case 3:
		s.newSeries(4, -1, 0, s.winnerOf(3, 0, 0), s.winnerOf(3, 1, 0))
	}
}

func (lg *League) finishPlayoffs() {
	s := lg.Season
	fin := s.Series[len(s.Series)-1]
	s.Champion = fin.Winner
	// Finals MVP: best (PPG + 0.5 RPG + 0.7 APG) on the champion over the finals.
	tot := map[int]*PlayerSeason{}
	for _, g := range fin.Games {
		b := s.PlayoffBoxes[g.Box]
		for _, bt := range b.Teams {
			if bt.TeamID != fin.Winner {
				continue
			}
			for _, l := range bt.Lines {
				ps := tot[l.PlayerID]
				if ps == nil {
					ps = &PlayerSeason{PlayerID: l.PlayerID, Name: l.Name, TeamID: bt.TeamID}
					tot[l.PlayerID] = ps
				}
				ps.GP++
				ps.Pts += l.Pts
				ps.Reb += l.Reb
				ps.Ast += l.Ast
			}
		}
	}
	var ids []int
	for id := range tot {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	best := -1.0
	var mvp *PlayerSeason
	for _, id := range ids {
		p := tot[id]
		if v := p.PPG() + 0.5*p.RPG() + 0.7*p.APG(); v > best {
			best, mvp = v, p
		}
	}
	if mvp != nil {
		s.Awards.FinalsMVP = awardOf(mvp)
	}
}

// SimPlayoffDay plays the next game of every active series in the current
// round. Returns false if there is nothing to play.
func (lg *League) SimPlayoffDay() bool {
	s := lg.Season
	if !s.PlayoffsActive() {
		return false
	}
	lg.gmAutoLineup() // assistant GM: auto lineup (gm.go)
	round := s.CurrentRound()
	r := lg.dayRNG(s.Year, playoffOffset+s.PlayoffDay)
	ir := lg.dayRNG(s.Year, injuryOffset+injuryPlayoffOffset+s.PlayoffDay)
	lg.newInjuries = nil
	lg.bmeans = nil
	for i := range s.Series {
		sr := &s.Series[i]
		if sr.Round != round || sr.Done {
			continue
		}
		g := len(sr.Games)
		h, aw := sr.High, sr.Low
		if homePattern[g] == 'A' {
			h, aw = aw, h
		}
		res := lg.SimGameRec(r, lg.Teams[h], lg.Teams[aw], true)
		homeWon := res.Home.Score > res.Away.Score
		if homeWon == (h == sr.High) {
			sr.HighWins++
		} else {
			sr.LowWins++
		}
		id := s.PlayoffGames
		s.PlayoffGames++
		sr.Games = append(sr.Games, PGame{N: g + 1, Home: h, HomeScore: res.Home.Score,
			AwayScore: res.Away.Score, OT: res.Overtimes, Day: s.PlayoffDay, Box: id})
		box := newBox(s.Year, -1, res)
		box.ID = id
		conf := ""
		if sr.Conf >= 0 {
			conf = [2]string{"West ", "East "}[sr.Conf]
		}
		box.Label = fmt.Sprintf("%s%s, game %d", conf, RoundName(sr.Round), g+1)
		s.PlayoffBoxes[id] = box
		if h == lg.UserTeamID || aw == lg.UserTeamID {
			lb := *box
			lb.Events = nil
			lg.LastBox = &lb
		}
		addStats(s.PlayoffStats, res)
		lg.afterGame(ir, res)
		if sr.HighWins == 4 || sr.LowWins == 4 {
			sr.Done = true
			sr.Winner = sr.High
			if sr.LowWins == 4 {
				sr.Winner = sr.Low
			}
		}
	}
	s.PlayoffDay++
	s.advance()
	if s.CurrentRound() == 0 {
		lg.finishPlayoffs()
	}
	return true
}

// SimUserSeries plays days until the user's current series ends.
func (lg *League) SimUserSeries() {
	i := lg.UserSeriesIdx()
	if i < 0 {
		return
	}
	for !lg.Season.Series[i].Done && lg.SimPlayoffDay() {
	}
}

// SimRound plays days until the current round is complete.
func (lg *League) SimRound() {
	s := lg.Season
	r := s.CurrentRound()
	for s.PlayoffsActive() && s.CurrentRound() == r {
		lg.SimPlayoffDay()
	}
}

// SimAllPlayoffs plays everything that remains.
func (lg *League) SimAllPlayoffs() {
	for lg.SimPlayoffDay() {
	}
}

// SimPlayoffs runs the full playoffs after the regular season.
func (lg *League) SimPlayoffs() error {
	if !lg.Season.PlayoffsStarted {
		if err := lg.StartPlayoffs(); err != nil {
			return err
		}
	}
	lg.SimAllPlayoffs()
	return nil
}
