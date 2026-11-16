package main

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"hoopsim/engine"
)

// foldAccents lowercases and strips the common Latin accents, so "Doncic" finds "Dončić".
var foldAccents = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ä", "a", "ã", "a", "å", "a", "ā", "a", "ă", "a", "ą", "a",
	"ć", "c", "č", "c", "ç", "c", "ď", "d", "đ", "d", "é", "e", "è", "e", "ê", "e", "ë", "e", "ē", "e", "ę", "e", "ě", "e",
	"ğ", "g", "í", "i", "ì", "i", "î", "i", "ï", "i", "ī", "i", "ı", "i", "ł", "l", "ń", "n", "ñ", "n", "ň", "n",
	"ó", "o", "ò", "o", "ô", "o", "ö", "o", "õ", "o", "ø", "o", "ō", "o", "ő", "o", "ř", "r", "ś", "s", "š", "s", "ş", "s",
	"ť", "t", "ú", "u", "ù", "u", "û", "u", "ü", "u", "ū", "u", "ů", "u", "ű", "u", "ý", "y", "ÿ", "y", "ź", "z", "ž", "z", "ż", "z",
)

func foldName(s string) string { return foldAccents.Replace(strings.ToLower(s)) }

// players is GET /api/players: every player on a team plus free agents (never
// draft prospects or retired players), filtered, sorted and paged.
func (s *server) players(r *http.Request) (any, error) {
	lg := s.lg
	q := r.URL.Query()
	intParam := func(key string) (int, bool, error) {
		v := q.Get(key)
		if v == "" {
			return 0, false, nil
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, false, bad("bad %s", key)
		}
		return n, true, nil
	}
	var (
		minOvr, maxOvr, minAge, maxAge, maxSal, yrs int
		hasMinOvr, hasMaxOvr, hasMinAge, hasMaxAge  bool
		hasMaxSal, hasYrs                           bool
		err                                         error
	)
	for _, f := range []struct {
		key string
		n   *int
		ok  *bool
	}{{"minOvr", &minOvr, &hasMinOvr}, {"maxOvr", &maxOvr, &hasMaxOvr}, {"minAge", &minAge, &hasMinAge},
		{"maxAge", &maxAge, &hasMaxAge}, {"maxSalary", &maxSal, &hasMaxSal}, {"yearsLeft", &yrs, &hasYrs}} {
		if *f.n, *f.ok, err = intParam(f.key); err != nil {
			return nil, err
		}
	}
	teamID, hasTeam, wantFA := 0, false, false
	if v := q.Get("team"); v == "fa" {
		wantFA = true
	} else if v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n >= len(lg.Teams) {
			return nil, bad("bad team")
		}
		teamID, hasTeam = n, true
	}
	pos := strings.ToUpper(q.Get("pos"))
	var wantPos engine.Position = -1
	if pos != "" {
		for i := engine.PG; i <= engine.C; i++ {
			if i.String() == pos {
				wantPos = i
			}
		}
		if wantPos < 0 {
			return nil, bad("unknown position %q", pos)
		}
	}
	badge := q.Get("badge")
	if badge != "" && !engine.ValidBadgeKey(badge) {
		return nil, bad("unknown badge %q", badge)
	}
	morale := q.Get("morale")
	if morale != "" {
		ok := false
		for _, l := range []string{"Furious", "Unhappy", "Content", "Happy", "Thrilled"} {
			ok = ok || strings.EqualFold(l, morale)
		}
		if !ok {
			return nil, bad("unknown morale %q", morale)
		}
	}
	sortKey, dir := q.Get("sort"), q.Get("dir")
	if sortKey == "" {
		sortKey = "ovr"
	}
	switch sortKey {
	case "ovr", "pot", "age", "salary", "name", "badges":
	default:
		return nil, bad("unknown sort %q", sortKey)
	}
	if dir != "" && dir != "asc" && dir != "desc" {
		return nil, bad("bad dir")
	}
	desc := dir == "desc" || (dir == "" && sortKey != "name" && sortKey != "age")
	limit, offset := 50, 0
	if n, ok, err := intParam("limit"); err != nil {
		return nil, err
	} else if ok {
		limit = max(1, min(n, 200))
	}
	if n, ok, err := intParam("offset"); err != nil {
		return nil, err
	} else if ok {
		offset = max(0, n)
	}
	needle := foldName(strings.TrimSpace(q.Get("q")))
	wantReq := q.Get("tradeRequest") == "1"

	type cand struct {
		p      *engine.Player
		team   int // -1 free agent
		badges int
	}
	var cands []cand
	consider := func(p *engine.Player, team int) {
		if needle != "" && !strings.Contains(foldName(p.Name), needle) {
			return
		}
		if hasMinOvr && p.Overall() < minOvr || hasMaxOvr && p.Overall() > maxOvr ||
			hasMinAge && p.Age < minAge || hasMaxAge && p.Age > maxAge ||
			hasMaxSal && p.Contract.Salary > maxSal || hasYrs && p.Contract.Years != yrs {
			return
		}
		if wantPos >= 0 {
			found := false
			for _, ps := range p.Positions() {
				found = found || ps == wantPos
			}
			if !found {
				return
			}
		}
		if badge != "" && !lg.HasBadge(p, badge) {
			return
		}
		if wantReq && p.TradeRequest == 0 {
			return
		}
		if morale != "" {
			if _, label, ok := lg.MoraleBrief(p); !ok || !strings.EqualFold(label, morale) {
				return
			}
		}
		cands = append(cands, cand{p: p, team: team})
	}
	if !wantFA {
		for _, t := range lg.Teams {
			if hasTeam && t.ID != teamID {
				continue
			}
			for _, p := range t.Roster {
				consider(p, t.ID)
			}
		}
	}
	if !hasTeam {
		for _, p := range lg.FreeAgents {
			if !s.inPool(p.ID) {
				consider(p, -1)
			}
		}
	}
	if sortKey == "badges" {
		for i := range cands {
			cands[i].badges = lg.BadgeCount(cands[i].p)
		}
	}
	less := func(a, b cand) int {
		switch sortKey {
		case "ovr":
			return a.p.Overall() - b.p.Overall()
		case "pot":
			return a.p.Potential - b.p.Potential
		case "age":
			return a.p.Age - b.p.Age
		case "salary":
			return a.p.Contract.Salary - b.p.Contract.Salary
		case "badges":
			return a.badges - b.badges
		}
		return strings.Compare(foldName(a.p.Name), foldName(b.p.Name))
	}
	sort.SliceStable(cands, func(i, j int) bool {
		c := less(cands[i], cands[j])
		if c == 0 {
			return cands[i].p.ID < cands[j].p.ID
		}
		if desc {
			return c > 0
		}
		return c < 0
	})
	total := len(cands)
	if offset > total {
		offset = total
	}
	end := min(total, offset+limit)
	rows := []M{}
	for _, c := range cands[offset:end] {
		p := c.p
		row := M{"id": p.ID, "name": p.Name, "pos": p.PosLabel(), "age": p.Age, "ovr": p.Overall(), "pot": p.Potential,
			"team": nil}
		if c.team >= 0 {
			row["team"] = s.team(c.team)
		}
		addInjury(row, p)
		addContract(row, p)
		addDev(row, p)
		s.addPhoto(row, p)
		rows = append(rows, row)
	}
	return M{"total": total, "players": rows}, nil
}
