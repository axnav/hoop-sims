package engine

// CareerRow is one player's line for one finished season.
type CareerRow struct {
	Year, TeamID, Age, Overall, Potential, GP int
	PPG, RPG, APG                             float64
}

// SeasonRecord summarises a finished season for the history page.
type SeasonRecord struct {
	Year          int
	Champion      int
	MVP           Award
	ScoringLeader Award
	FinalsMVP     Award
	Series        []Series
}

// BoxLine is one player's line in a stored box score.
type BoxLine struct {
	PlayerID                     int
	Name, Pos                    string
	Min, Pts, Reb, Ast, Tov      int
	FGM, FGA, TPM, TPA, FTM, FTA int
}

// BoxTeam is one side of a stored box score.
type BoxTeam struct {
	TeamID, Score int
	Lines         []BoxLine
}

// Box is a plain-data box score (safe to save as JSON). Teams[0] is home.
type Box struct {
	Year, Day int // Day is -1 for playoff games
	ID        int // playoff game ID (key in Season.PlayoffBoxes); meaningless for regular season
	Label     string
	Overtimes int
	Teams     [2]BoxTeam
	Events    []Event `json:",omitempty"`
}

func newBox(year, day int, res *GameResult) *Box {
	b := &Box{Year: year, Day: day, Overtimes: res.Overtimes, Events: res.Events}
	for i, tb := range []*TeamBox{res.Home, res.Away} {
		bt := BoxTeam{TeamID: tb.Team.ID, Score: tb.Score}
		for _, l := range tb.Lines {
			if l.Min == 0 {
				continue
			}
			bt.Lines = append(bt.Lines, BoxLine{l.Player.ID, l.Player.Name, l.Player.PosLabel(),
				l.Min, l.Pts, l.Reb, l.Ast, l.Tov, l.FGM, l.FGA, l.TPM, l.TPA, l.FTM, l.FTA})
		}
		b.Teams[i] = bt
	}
	return b
}

// recordBox keeps the box score if the user's team played.
func (lg *League) recordBox(day int, res *GameResult) {
	if res.Home.Team.ID == lg.UserTeamID || res.Away.Team.ID == lg.UserTeamID {
		lg.LastBox = newBox(lg.Year, day, res)
	}
}

// FindPlayer looks a player up on rosters, in free agency, or among the retired.
func (lg *League) FindPlayer(id int) *Player {
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			if p.ID == id {
				return p
			}
		}
	}
	for _, p := range lg.FreeAgents {
		if p.ID == id {
			return p
		}
	}
	for _, p := range lg.Retired {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// recordSeason stores career rows and the season record. Called once per
// offseason, before players age.
func (lg *League) recordSeason() {
	s := lg.Season
	if s == nil {
		return
	}
	if lg.Careers == nil {
		lg.Careers = map[int][]CareerRow{}
	}
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			row := CareerRow{Year: s.Year, TeamID: t.ID, Age: p.Age, Overall: p.Overall(), Potential: p.Potential}
			if ps := s.Stats[p.ID]; ps != nil {
				row.GP, row.PPG, row.RPG, row.APG = ps.GP, ps.PPG(), ps.RPG(), ps.APG()
				row.TeamID = ps.TeamID
			}
			lg.Careers[p.ID] = append(lg.Careers[p.ID], row)
		}
	}
	lg.Records = append(lg.Records, &SeasonRecord{Year: s.Year, Champion: s.Champion,
		MVP: s.Awards.MVP, ScoringLeader: s.Awards.ScoringLeader, FinalsMVP: s.Awards.FinalsMVP, Series: s.Series})
}
