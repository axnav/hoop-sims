package main

import (
	"math"
	"net/http"
	"strconv"

	"hoopsim/engine"
)

// ---- trades ----

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func (s *server) tradeStatus(r *http.Request) (any, error) {
	open, why := engine.TradeWindowOpen(s.lg)
	reopens := ""
	if !open {
		if s.lg.Draft != nil {
			reopens = "When the draft and offseason are finished, the offseason window opens."
		} else {
			reopens = "The offseason window opens after the playoffs and the draft."
		}
	}
	return M{"open": open, "reason": why, "reopens": reopens, "deadlineDay": engine.TradeDeadlineDay,
		"day": s.lg.Season.Day, "maxPlayers": engine.MaxTradePlayers, "rosterSize": engine.RosterSize,
		"maxTeams": engine.MaxDealTeams, "maxPicks": engine.MaxDealPicks}, nil
}

func (s *server) tradeBlock(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	if id < 0 || id >= len(s.lg.Teams) {
		return nil, missing("no such team")
	}
	rows := []M{}
	for _, a := range engine.TradeBlock(s.lg, id) {
		rows = append(rows, s.playerRow(id, a))
	}
	picks := []M{}
	for _, p := range s.lg.PicksOwnedBy(id) {
		picks = append(picks, s.pickRow(id, p.Year, p.Round, p.Original))
	}
	return M{"team": s.team(id), "isUser": id == s.lg.UserTeamID, "players": rows, "picks": picks}, nil
}

// playerRow is one trade-block row for a player on team owner's roster.
func (s *server) playerRow(owner int, a engine.TradeAsset) M {
	tier := ""
	for _, p := range s.lg.Teams[owner].Roster {
		if p.ID == a.PlayerID {
			tier = engine.PlayerTier(p)
			break
		}
	}
	return M{"id": a.PlayerID, "name": a.Name, "pos": a.Pos, "age": a.Age, "ovr": a.Overall, "pot": a.Pot,
		"value": round1(a.Value), "untouchable": a.Untouchable, "tier": tier}
}

func (s *server) pickLabelFor(owner, year, round, original int) string {
	label := pickLabel(year, round)
	if original != owner {
		label += " (via " + s.team(original)["abbr"].(string) + ")"
	}
	return label
}

// pickRow is one trade-block row for a pick owned by team owner.
func (s *server) pickRow(owner, year, round, original int) M {
	return M{"year": year, "round": round, "original": s.team(original), "own": original == owner,
		"label": s.pickLabelFor(owner, year, round, original),
		"value": round1(s.lg.PickValue(year, round, original, owner)), "tier": s.lg.PickTier(year, round, original)}
}

func pickLabel(year, round int) string {
	if round == 1 {
		return strconv.Itoa(year) + " 1st"
	}
	return strconv.Itoa(year) + " 2nd"
}

type tradeIn struct {
	Team int   `json:"team"`
	Give []int `json:"give"`
	Get  []int `json:"get"`
}

func (s *server) readOffer(r *http.Request) (engine.TradeOffer, error) {
	var in tradeIn
	if err := readJSON(r, &in); err != nil {
		return engine.TradeOffer{}, err
	}
	return engine.TradeOffer{FromTeam: s.lg.UserTeamID, ToTeam: in.Team, Give: in.Give, Get: in.Get}, nil
}

func verdictDTO(v engine.TradeVerdict) M {
	return M{"valid": v.Valid, "problem": v.Problem, "accept": v.Accept, "reason": v.Reason,
		"theyGive": round1(v.TheyGive), "theyGet": round1(v.TheyGet), "youGive": round1(v.YouGive), "youGet": round1(v.YouGet),
		"needed": math.Round(v.Needed*100) / 100}
}

func (s *server) tradeEvaluate(r *http.Request) (any, error) {
	o, err := s.readOffer(r)
	if err != nil {
		return nil, err
	}
	return verdictDTO(engine.EvaluateTrade(s.lg, o)), nil
}

func (s *server) tradeCounter(r *http.Request) (any, error) {
	o, err := s.readOffer(r)
	if err != nil {
		return nil, err
	}
	c, ok := engine.SuggestCounter(s.lg, o)
	if !ok {
		return M{"found": false}, nil
	}
	return M{"found": true, "give": c.Give, "get": c.Get, "verdict": verdictDTO(engine.EvaluateTrade(s.lg, c))}, nil
}

func (s *server) txPlayers(in []engine.TxPlayer) []M {
	out := []M{}
	for _, p := range in {
		out = append(out, M{"playerId": p.PlayerID, "name": p.Name, "from": s.team(p.From), "to": s.team(p.To)})
	}
	return out
}

func (s *server) txDTO(t engine.TradeResult) M {
	d := M{"year": t.Year, "day": t.Day, "teamA": s.team(t.TeamA), "teamB": s.team(t.TeamB),
		"moved": s.txPlayers(t.Moved), "released": s.txPlayers(t.Released), "signed": s.txPlayers(t.Signed)}
	if len(t.Teams) > 0 {
		ts := []M{}
		for _, id := range t.Teams {
			ts = append(ts, s.team(id))
		}
		d["teams"] = ts
	}
	if len(t.Picks) > 0 {
		ps := []M{}
		for _, p := range t.Picks {
			o := s.team(p.Original)
			ps = append(ps, M{"year": p.Year, "round": p.Round, "original": o,
				"label": pickLabel(p.Year, p.Round) + " (" + o["abbr"].(string) + ")",
				"from":  s.team(p.From), "to": s.team(p.To)})
		}
		d["picks"] = ps
	}
	return d
}

type dealPickIn struct {
	Year     int `json:"year"`
	Round    int `json:"round"`
	Original int `json:"original"`
}

type dealMoveIn struct {
	From     int         `json:"from"`
	To       int         `json:"to"`
	PlayerID *int        `json:"playerId"`
	Pick     *dealPickIn `json:"pick"`
}

func (s *server) readDeal(r *http.Request) (engine.Deal, error) {
	var in struct {
		Moves []dealMoveIn `json:"moves"`
	}
	if err := readJSON(r, &in); err != nil {
		return engine.Deal{}, err
	}
	var d engine.Deal
	for _, m := range in.Moves {
		if (m.PlayerID == nil) == (m.Pick == nil) {
			return engine.Deal{}, bad("each move needs exactly one of playerId or pick")
		}
		a := engine.Asset{}
		if m.PlayerID != nil {
			a = engine.PlayerOf(*m.PlayerID)
		} else {
			a = engine.PickOf(m.Pick.Year, m.Pick.Round, m.Pick.Original)
		}
		d.Moves = append(d.Moves, engine.Move{From: m.From, To: m.To, Asset: a})
	}
	return d, nil
}

func (s *server) dealEvaluate(r *http.Request) (any, error) {
	d, err := s.readDeal(r)
	if err != nil {
		return nil, err
	}
	v := engine.EvaluateDeal(s.lg, d, s.lg.UserTeamID)
	teams := []M{}
	for _, t := range v.Teams {
		row := M{"team": s.team(t.Team), "accept": t.Accept, "reason": t.Reason,
			"gives": round1(t.Gives), "gets": round1(t.Gets), "needed": round1(t.Needed),
			"mood": t.Mood, "askText": t.AskText}
		if t.Ask != nil {
			user := s.lg.UserTeamID
			row["ask"] = s.wireMove(engine.Move{From: user, To: t.Team, Asset: *t.Ask})
			row["askLabel"] = s.askLabel(*t.Ask, user)
		}
		teams = append(teams, row)
	}
	return M{"valid": v.Valid, "problem": v.Problem, "accept": v.Accept, "teams": teams}, nil
}

func (s *server) dealExecute(r *http.Request) (any, error) {
	d, err := s.readDeal(r)
	if err != nil {
		return nil, err
	}
	res, err := engine.ExecuteDeal(s.lg, d, s.lg.UserTeamID)
	if err != nil {
		return nil, conflict("%v", err)
	}
	out := s.txDTO(res)
	out["state"] = s.state()
	return out, nil
}

func (s *server) tradeExecute(r *http.Request) (any, error) {
	o, err := s.readOffer(r)
	if err != nil {
		return nil, err
	}
	res, err := engine.ExecuteTrade(s.lg, o)
	if err != nil {
		return nil, conflict("%v", err)
	}
	d := s.txDTO(res)
	d["state"] = s.state()
	return d, nil
}

func (s *server) transactions(r *http.Request) (any, error) {
	limit := 100
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	rows := []M{}
	tx := s.lg.Transactions
	for i := len(tx) - 1; i >= 0 && len(rows) < limit; i-- {
		rows = append(rows, s.txDTO(tx[i]))
	}
	return M{"transactions": rows, "userTeam": s.lg.UserTeamID}, nil
}

// offseasonTrades lists the computer-to-computer trades of the offseason that
// follows the report's season (FinishOffseason makes them after Year++).
func (s *server) offseasonTrades(reportYear int) []M {
	out := []M{}
	for _, t := range s.lg.Transactions {
		if t.Year != reportYear+1 || t.Day != 0 || t.TeamA == s.lg.UserTeamID || t.TeamB == s.lg.UserTeamID {
			continue
		}
		out = append(out, s.txDTO(t))
	}
	return out
}
