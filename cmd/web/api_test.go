package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"hoopsim/engine"
)

type tc struct {
	s  *server
	t  *testing.T
	ts *httptest.Server
}

func newTC(t *testing.T) *tc {
	s := &server{savePath: filepath.Join(t.TempDir(), "league.json"), realDir: "../../data/real", webDir: t.TempDir()}
	ts := httptest.NewServer(s.routes())
	t.Cleanup(ts.Close)
	return &tc{s: s, t: t, ts: ts}
}

func (c *tc) call(method, path string, body any, want int) M {
	c.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, c.ts.URL+path, rd)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out M
	json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != want {
		c.t.Fatalf("%s %s: status %d, want %d: %v", method, path, resp.StatusCode, want, out)
	}
	c.checkNoImpact(path, out)
	return out
}

func (c *tc) checkNoImpact(path string, v any) {
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case M:
			for k, e := range x {
				if strings.Contains(strings.ToLower(k), "impact") {
					c.t.Fatalf("%s: response has key %q", path, k)
				}
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(v)
}

func num(v any) int { f, _ := v.(float64); return int(f) }

func TestFlow(t *testing.T) {
	c := newTC(t)
	c.call("GET", "/api/state", nil, 409)
	meta := c.call("GET", "/api/meta", nil, 200)
	if meta["loaded"] != false || len(meta["fictionalTeams"].([]any)) != 30 {
		t.Fatalf("bad meta")
	}
	c.call("POST", "/api/new", M{"mode": "fictional", "seed": 7, "team": 3}, 200)
	st := c.call("GET", "/api/state", nil, 200)
	if st["phase"] != "regular" || st["nextAction"].(M)["id"] != "sim_day" {
		t.Fatalf("state: %v", st)
	}
	c.call("POST", "/api/playoffs/day", nil, 409)
	r := c.call("POST", "/api/sim", M{"days": 3}, 200)
	if len(r["days"].([]any)) != 3 || num(r["state"].(M)["day"]) != 3 {
		t.Fatalf("sim: %v", r)
	}
	c.call("GET", "/api/standings", nil, 200)
	c.call("GET", "/api/leaders", nil, 200)
	team := c.call("GET", "/api/team/3", nil, 200)
	pid := num(team["roster"].([]any)[0].(M)["id"])
	c.call("GET", "/api/player/"+itoa(pid), nil, 200)
	c.call("GET", "/api/schedule?team=3", nil, 200)
	c.call("GET", "/api/box/last", nil, 200)
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)
	c.call("POST", "/api/sim", M{"toEnd": true}, 200)
	st = c.call("GET", "/api/state", nil, 200)
	if st["phase"] != "regular_done" || st["summary"] == nil {
		t.Fatalf("expected regular_done: %v", st["phase"])
	}
	c.call("POST", "/api/sim", M{"days": 1}, 409)
	c.call("POST", "/api/playoffs/start", nil, 200)
	c.call("GET", "/api/playoffs", nil, 200)
	c.call("POST", "/api/playoffs/day", nil, 200)
	out := c.call("POST", "/api/playoffs/all", nil, 200)
	st = out["state"].(M)
	if st["phase"] != "playoffs_done" || st["playoffs"].(M)["champion"] == nil {
		t.Fatalf("expected playoffs_done: %v", st["phase"])
	}
	c.call("GET", "/api/box/0", nil, 200)
	c.call("GET", "/api/game/0/events", nil, 200)
	st = c.call("POST", "/api/offseason/begin", nil, 200)
	if st["phase"] != "offseason" {
		t.Fatalf("phase %v", st["phase"])
	}
	c.call("GET", "/api/offseason", nil, 200)
	c.call("POST", "/api/draft/step", nil, 409)
	c.call("POST", "/api/offseason/to-draft", nil, 200)
	for i := 0; i < 200; i++ {
		d := c.call("GET", "/api/draft", nil, 200)
		if d["done"] == true {
			break
		}
		if d["userOnClock"] == true {
			pr := d["prospects"].([]any)[0].(M)
			c.call("POST", "/api/draft/pick", M{"playerId": num(pr["id"])}, 200)
			continue
		}
		c.call("POST", "/api/draft/step", M{"max": 7}, 200)
	}
	st = c.call("GET", "/api/state", nil, 200)
	if st["phase"] != "draft_done" {
		t.Fatalf("expected draft_done: %v", st["phase"])
	}
	c.call("POST", "/api/offseason/finish", nil, 200)
	st = c.call("GET", "/api/state", nil, 200)
	if st["phase"] != "regular" || num(st["year"]) != 2026 || num(st["day"]) != 0 {
		t.Fatalf("next season: %v", st)
	}
	h := c.call("GET", "/api/history", nil, 200)
	if len(h["seasons"].([]any)) != 1 {
		t.Fatalf("history: %v", h)
	}
	c.call("POST", "/api/save", nil, 200)
	c.call("POST", "/api/load", nil, 200)
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func TestNoHiddenNumber(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 2, "team": 0}, 200)
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)
	c.call("POST", "/api/sim", M{"days": 30}, 200)
	for _, p := range []string{"/api/state", "/api/standings", "/api/leaders", "/api/team/0", "/api/player/1", "/api/schedule", "/api/box/last", "/api/history", "/api/playoffs"} {
		c.call("GET", p, nil, 200) // checkNoImpact runs on every response
	}
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)
	c.call("POST", "/api/sim", M{"toEnd": true}, 200)
	c.call("POST", "/api/playoffs/start", nil, 200)
	c.call("POST", "/api/playoffs/all", nil, 200)
	c.call("POST", "/api/offseason/begin", nil, 200)
	c.call("GET", "/api/offseason", nil, 200)
	c.call("POST", "/api/offseason/to-draft", nil, 200)
	c.call("GET", "/api/draft", nil, 200)
	c.call("GET", "/api/game/0/events", nil, 200)
}

func TestRealLeagueConferences(t *testing.T) {
	c := newTC(t)
	meta := c.call("GET", "/api/meta", nil, 200)
	if meta["realAvailable"] != true {
		t.Skip("real data absent")
	}
	for _, x := range meta["realTeams"].([]any) {
		tm := x.(M)
		if tm["abbr"] == "ATL" && tm["confName"] != "East" {
			t.Fatalf("Atlanta is %v", tm["confName"])
		}
		if tm["abbr"] == "LAL" && tm["confName"] != "West" {
			t.Fatalf("Lakers are %v", tm["confName"])
		}
	}
}

func blockIDs(c *tc, team int) (best, worst, top int) {
	b := c.call("GET", "/api/trade/block/"+itoa(team), nil, 200)["players"].([]any)
	bv, wv := -1.0, 1e18
	for _, x := range b {
		p := x.(M)
		v := p["value"].(float64)
		if v > bv {
			bv, best = v, num(p["id"])
		}
		if v < wv {
			wv, worst = v, num(p["id"])
		}
	}
	top = best
	return
}

// salaryOf reads a player's salary from a team's trade block.
func salaryOf(c *tc, team, id int) int {
	for _, x := range c.call("GET", "/api/trade/block/"+itoa(team), nil, 200)["players"].([]any) {
		if p := x.(M); num(p["id"]) == id {
			return num(p["salary"])
		}
	}
	c.t.Fatalf("player %d not on the block of team %d", id, team)
	return 0
}

// matchID picks a player from a team's block whose salary is close enough to
// every reference salary that a swap is salary-legal both ways (the trade rule
// allows 125% plus $250K). best chooses the highest trade value, else the lowest.
func matchID(c *tc, team int, best bool, refs ...int) int {
	c.t.Helper()
	id, bv := -1, 0.0
	for _, x := range c.call("GET", "/api/trade/block/"+itoa(team), nil, 200)["players"].([]any) {
		p := x.(M)
		sal, ok := num(p["salary"]), true
		for _, r := range refs {
			if sal > r*5/4+250 || r > sal*5/4+250 {
				ok = false
			}
		}
		v := p["value"].(float64)
		if ok && (id < 0 || (best && v > bv) || (!best && v < bv)) {
			id, bv = num(p["id"]), v
		}
	}
	if id < 0 {
		c.t.Fatalf("no salary-matched player on team %d for %v", team, refs)
	}
	return id
}

// roomiest returns the team (not except) with the most cap room.
func roomiest(c *tc, except int) int {
	best, room := -1, -1<<30
	for id := 0; id < 30; id++ {
		if id == except {
			continue
		}
		if r := num(c.call("GET", "/api/team/"+itoa(id), nil, 200)["payroll"].(M)["room"]); r > room {
			best, room = id, r
		}
	}
	return best
}

// generousPair finds the salary-legal swap where team mine gives the most trade
// value over what it gets back from team theirs. A side is legal when it ends
// at or under the cap, or takes back at most 125% of what it sends plus $250K.
func generousPair(c *tc, mine, theirs int) (give, get int) {
	c.t.Helper()
	ba := c.call("GET", "/api/trade/block/"+itoa(mine), nil, 200)
	bb := c.call("GET", "/api/trade/block/"+itoa(theirs), nil, 200)
	legal := func(blk M, in, out int) bool {
		pay := blk["payroll"].(M)
		return num(pay["payroll"])+in-out <= num(pay["cap"]) || in <= out*5/4+250
	}
	best, found := 0.0, false
	for _, x := range ba["players"].([]any) {
		for _, y := range bb["players"].([]any) {
			p, q := x.(M), y.(M)
			sp, sq := num(p["salary"]), num(q["salary"])
			if !legal(ba, sq, sp) || !legal(bb, sp, sq) {
				continue
			}
			if d := p["value"].(float64) - q["value"].(float64); !found || d > best {
				best, found, give, get = d, true, num(p["id"]), num(q["id"])
			}
		}
	}
	if !found {
		c.t.Fatalf("no salary-legal pair between teams %d and %d", mine, theirs)
	}
	return
}

// dealTeams returns the per-team verdicts, failing with the problem text if the deal is invalid.
func dealTeams(t *testing.T, v M) []any {
	t.Helper()
	if v["valid"] != true {
		t.Fatalf("deal invalid: %v", v["problem"])
	}
	return v["teams"].([]any)
}

func TestTrades(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	if c.call("GET", "/api/trade/status", nil, 200)["open"] != true {
		t.Fatal("window should be open at season start")
	}
	_, myWorst, _ := blockIDs(c, 3)
	// salary-legal partners: similar salaries so the cap rules stay out of the way
	theirBest := matchID(c, 4, true, salaryOf(c, 3, myWorst))
	// the generous swap goes to the team with the most cap room (it must absorb salary)
	gp := roomiest(c, 3)
	myBest, theirWorst := generousPair(c, 3, gp)
	// lopsided: my worst for their best
	v := c.call("POST", "/api/trade/evaluate", M{"team": 4, "give": []int{myWorst}, "get": []int{theirBest}}, 200)
	if v["valid"] != true || v["accept"] != false || v["reason"] == "" {
		t.Fatalf("lopsided offer: %v", v)
	}
	c.call("POST", "/api/trade/execute", M{"team": 4, "give": []int{myWorst}, "get": []int{theirBest}}, 409)
	c.call("POST", "/api/trade/counter", M{"team": 4, "give": []int{myWorst}, "get": []int{theirBest}}, 200)
	// generous: my best for their worst
	v = c.call("POST", "/api/trade/evaluate", M{"team": gp, "give": []int{myBest}, "get": []int{theirWorst}}, 200)
	if v["accept"] != true {
		t.Fatalf("generous offer refused: %v", v)
	}
	r := c.call("POST", "/api/trade/execute", M{"team": gp, "give": []int{myBest}, "get": []int{theirWorst}}, 200)
	if len(r["moved"].([]any)) != 2 || r["state"] == nil {
		t.Fatalf("execute: %v", r)
	}
	for _, id := range []int{3, gp} {
		if n := len(c.call("GET", "/api/team/"+itoa(id), nil, 200)["roster"].([]any)); n != 13 {
			t.Fatalf("team %d roster %d", id, n)
		}
	}
	if n := len(c.call("GET", "/api/transactions", nil, 200)["transactions"].([]any)); n != 1 {
		t.Fatalf("transactions %d", n)
	}
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)
	c.call("POST", "/api/sim", M{"days": 60}, 200)
	if c.call("GET", "/api/trade/status", nil, 200)["open"] != false {
		t.Fatal("window should be closed after the deadline")
	}
	v = c.call("POST", "/api/trade/evaluate", M{"team": 5, "give": []int{myWorst}, "get": []int{theirWorst}}, 200)
	if v["valid"] != false || v["problem"] == "" {
		t.Fatalf("closed window evaluate: %v", v)
	}
	c.call("POST", "/api/trade/execute", M{"team": 5, "give": []int{myWorst}, "get": []int{theirWorst}}, 409)
}

func TestDeals(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	if st := c.call("GET", "/api/trade/status", nil, 200); num(st["maxTeams"]) != 3 || num(st["maxPicks"]) != 4 {
		t.Fatalf("status: %v", st)
	}
	blk := c.call("GET", "/api/trade/block/3", nil, 200)
	picks := blk["picks"].([]any)
	if len(picks) != 8 {
		t.Fatalf("picks %d", len(picks))
	}
	p0 := picks[0].(M)
	if p0["own"] != true || p0["label"] == "" || p0["original"] == nil {
		t.Fatalf("pick: %v", p0)
	}
	myPick := func(i int) M {
		p := picks[i].(M)
		return M{"year": p["year"], "round": p["round"], "original": num(p["original"].(M)["id"])}
	}
	// two-team: my first-round pick for their worst player (salaries matched)
	_, myWorst, _ := blockIDs(c, 3)
	mySal := salaryOf(c, 3, myWorst)
	theirWorst := matchID(c, 4, false, mySal)
	third := matchID(c, 5, false, mySal, salaryOf(c, 4, theirWorst))
	moves := []M{{"from": 3, "to": 4, "pick": myPick(0)}, {"from": 3, "to": 4, "playerId": myWorst}, {"from": 4, "to": 3, "playerId": theirWorst}}
	v := c.call("POST", "/api/deal/evaluate", M{"moves": moves}, 200)
	if len(dealTeams(t, v)) != 1 {
		t.Fatalf("two-team: %v", v)
	}
	vt := v["teams"].([]any)[0].(M)
	if vt["reason"] == "" || vt["team"] == nil {
		t.Fatalf("verdict: %v", vt)
	}
	// three-team
	moves3 := []M{{"from": 3, "to": 4, "pick": myPick(0)}, {"from": 3, "to": 4, "playerId": myWorst}, {"from": 4, "to": 5, "playerId": theirWorst},
		{"from": 5, "to": 3, "playerId": third}}
	v = c.call("POST", "/api/deal/evaluate", M{"moves": moves3}, 200)
	if len(dealTeams(t, v)) != 2 {
		t.Fatalf("three-team: %v", v)
	}
	// malformed moves
	c.call("POST", "/api/deal/evaluate", M{"moves": []M{{"from": 3, "to": 4}}}, 400)
	c.call("POST", "/api/deal/evaluate", M{"moves": []M{{"from": 3, "to": 4, "playerId": 1, "pick": myPick(0)}}}, 400)
	// generous: all my picks plus my best player for their worst player
	gp := roomiest(c, 3)
	myBest, theirWorst2 := generousPair(c, 3, gp)
	gen := []M{{"from": gp, "to": 3, "playerId": theirWorst2}, {"from": 3, "to": gp, "playerId": myBest}}
	for i := 0; i < 4; i++ {
		gen = append(gen, M{"from": 3, "to": gp, "pick": myPick(i)})
	}
	v = c.call("POST", "/api/deal/evaluate", M{"moves": gen}, 200)
	if v["accept"] != true {
		t.Fatalf("generous deal refused: %v", v)
	}
	r := c.call("POST", "/api/deal/execute", M{"moves": gen}, 200)
	if r["state"] == nil || len(r["picks"].([]any)) != 4 || len(r["teams"].([]any)) != 2 {
		t.Fatalf("execute: %v", r)
	}
	tx := c.call("GET", "/api/transactions", nil, 200)["transactions"].([]any)
	if len(tx) != 1 || len(tx[0].(M)["picks"].([]any)) != 4 {
		t.Fatalf("transactions: %v", tx)
	}
	if n := len(c.call("GET", "/api/trade/block/"+itoa(gp), nil, 200)["picks"].([]any)); n != 12 {
		t.Fatalf("team %d picks %d", gp, n)
	}
	// a rejected deal is a 409
	c.call("POST", "/api/deal/execute", M{"moves": []M{{"from": 3, "to": 4, "playerId": 0}}}, 409)
}

func TestNegotiationAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	blk := c.call("GET", "/api/trade/block/3", nil, 200)
	for _, k := range []string{"players", "picks"} {
		for _, x := range blk[k].([]any) {
			if x.(M)["tier"] == "" || x.(M)["tier"] == nil {
				t.Fatalf("%s row without tier: %v", k, x)
			}
		}
	}
	_, myWorst, _ := blockIDs(c, 3)
	theirBest := matchID(c, 4, true, salaryOf(c, 3, myWorst))
	moves := []M{{"from": 3, "to": 4, "playerId": myWorst}, {"from": 4, "to": 3, "playerId": theirBest}}
	v := c.call("POST", "/api/deal/evaluate", M{"moves": moves}, 200)
	if v["accept"] != false {
		t.Fatalf("expected refusal: %v", v)
	}
	tv := dealTeams(t, v)[0].(M)
	if tv["mood"] == "" || tv["mood"] == nil || tv["askText"] == "" || tv["askText"] == nil {
		t.Fatalf("verdict: %v", tv)
	}
	if ask, ok := tv["ask"].(M); ok {
		if tv["askLabel"] == "" || tv["askLabel"] == nil {
			t.Fatalf("no askLabel: %v", tv)
		}
		v2 := c.call("POST", "/api/deal/evaluate", M{"moves": append(moves, ask)}, 200)
		if dealTeams(t, v2)[0].(M)["accept"] != true {
			t.Fatalf("ask did not satisfy: %v", v2)
		}
	}

	// options
	o := c.call("GET", "/api/options", nil, 200)
	if o["noDeadlineStop"] != false || o["noOfferStop"] != false {
		t.Fatalf("options: %v", o)
	}
	o = c.call("POST", "/api/options", M{"noOfferStop": true}, 200)
	if o["noDeadlineStop"] != false || o["noOfferStop"] != true {
		t.Fatalf("options: %v", o)
	}
	c.call("POST", "/api/options", M{"noOfferStop": false}, 200)

	// a single manual day never pauses
	r := c.call("POST", "/api/sim", M{"days": 1}, 200)
	if r["pause"] != nil || r["newOffers"] == nil {
		t.Fatalf("manual day: %v", r["pause"])
	}
}

func TestDeadlinePause(t *testing.T) {
	for _, off := range []bool{false, true} {
		c := newTC(t)
		c.call("POST", "/api/new", M{"seed": 7, "team": 2}, 200)
		c.call("POST", "/api/options", M{"noDeadlineStop": off, "noOfferStop": true}, 200)
		pauses := 0
		for i := 0; i < 100; i++ {
			r := c.call("POST", "/api/sim", M{"auto": true}, 200)
			if r["state"].(M)["phase"] != "regular" {
				break
			}
			if p, ok := r["pause"].(M); ok && p["reason"] == "deadline" {
				pauses++
				if num(r["state"].(M)["day"]) != engine.TradeDeadlineDay || len(r["days"].([]any)) != 0 {
					t.Fatalf("bad deadline pause: %v", r["state"])
				}
			}
		}
		if (off && pauses != 0) || (!off && pauses != 1) {
			t.Fatalf("noDeadlineStop=%v: %d pauses", off, pauses)
		}
	}
}

func TestOfferFlow(t *testing.T) {
	for seed := 1; seed <= 6; seed++ {
		c := newTC(t)
		c.call("POST", "/api/new", M{"seed": seed, "team": 3}, 200)
		c.call("POST", "/api/options", M{"noDeadlineStop": true}, 200)
		got := 0
		for i := 0; i < 70 && got < 2; i++ {
			r := c.call("POST", "/api/sim", M{"auto": true}, 200)
			p, _ := r["pause"].(M)
			if p == nil || p["reason"] != "offer" || p["text"] == "" {
				continue
			}
			got++
			list := c.call("GET", "/api/offers", nil, 200)["offers"].([]any)
			if len(list) == 0 || num(r["state"].(M)["offers"]) != len(list) {
				t.Fatalf("offers %d vs state %v", len(list), r["state"].(M)["offers"])
			}
			o := list[0].(M)
			if len(o["give"].([]any)) == 0 || len(o["get"].([]any)) == 0 || len(o["moves"].([]any)) == 0 || o["message"] == "" {
				t.Fatalf("offer: %v", o)
			}
			id := itoa(num(o["id"]))
			if got == 1 {
				res := c.call("POST", "/api/offers/"+id+"/accept", nil, 200)
				if res["state"] == nil || res["moved"] == nil {
					t.Fatalf("accept: %v", res)
				}
				c.call("POST", "/api/offers/"+id+"/accept", nil, 404)
			} else {
				res := c.call("POST", "/api/offers/"+id+"/reject", nil, 200)
				if res["ok"] != true || len(res["offers"].([]any)) != len(list)-1 {
					t.Fatalf("reject: %v", res)
				}
				c.call("POST", "/api/offers/"+id+"/reject", nil, 404)
			}
		}
		if got >= 2 {
			return
		}
	}
	t.Fatal("no seed produced two offer pauses")
}

func TestInjuriesAndCommissioner(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"mode": "fictional", "seed": 11, "team": 2}, 200)

	o := c.call("GET", "/api/options", nil, 200)
	for _, k := range []string{"noDeadlineStop", "noOfferStop", "noInjuries", "commissioner"} {
		if o[k] != false {
			t.Fatalf("option %s default: %v", k, o)
		}
	}
	o = c.call("POST", "/api/options", M{"noInjuries": true}, 200)
	if o["noInjuries"] != true || o["commissioner"] != false || o["noOfferStop"] != false {
		t.Fatalf("options: %v", o)
	}
	o = c.call("POST", "/api/options", M{"noInjuries": false, "commissioner": true}, 200)
	if o["noInjuries"] != false || o["commissioner"] != true {
		t.Fatalf("options: %v", o)
	}
	c.call("POST", "/api/options", M{"commissioner": false, "noOfferStop": true, "noDeadlineStop": true}, 200)

	// a player to edit, before any injuries
	team := c.call("GET", "/api/team/2", nil, 200)
	pid := num(team["roster"].([]any)[0].(M)["id"])
	path := "/api/player/" + strconv.Itoa(pid)
	admin := "/api/admin/player/" + strconv.Itoa(pid)
	base := c.call("GET", path, nil, 200)
	wantKeys := []string{"inside", "mid", "three", "playmaking", "rebounding", "defense", "athleticism"}
	rs := base["ratings"].([]any)
	if len(rs) != 7 || base["editable"] != false || base["limits"] == nil {
		t.Fatalf("player: %v", base)
	}
	for i, k := range wantKeys {
		if rs[i].(M)["key"] != k {
			t.Fatalf("rating order: %v", rs)
		}
	}

	// refused with commissioner off
	c.call("POST", admin, M{"age": 30}, 409)
	c.call("POST", "/api/options", M{"commissioner": true}, 200)
	c.call("POST", "/api/admin/player/999999", M{"age": 30}, 404)
	c.call("POST", admin, M{"ratings": M{"inside": 500}}, 400)
	c.call("POST", admin, M{"ratings": M{"nonsense": 50}}, 400)
	same := c.call("GET", path, nil, 200)
	if num(same["age"]) != num(base["age"]) || num(same["pot"]) != num(base["pot"]) {
		t.Fatalf("failed edit changed player")
	}
	got := c.call("POST", admin, M{"age": 31, "potential": 99, "ratings": M{"inside": 77, "three": 66}}, 200)
	chk := c.call("GET", path, nil, 200)
	for _, v := range []M{got, chk} {
		r := v["ratings"].([]any)
		if num(v["age"]) != 31 || num(v["pot"]) != 99 || num(r[0].(M)["value"]) != 77 || num(r[2].(M)["value"]) != 66 || v["editable"] != true {
			t.Fatalf("edit not applied: %v", v)
		}
	}
	c.call("POST", "/api/options", M{"commissioner": false}, 200)
	if c.call("GET", path, nil, 200)["editable"] != false {
		t.Fatalf("editable should follow option")
	}

	// injuries
	var inj []any
	for i := 0; i < 40 && len(inj) == 0; i++ {
		r := c.call("POST", "/api/sim", M{"days": 5}, 200)
		if _, ok := r["injuries"].([]any); !ok {
			t.Fatalf("sim lacks injuries array: %v", r)
		}
		inj = c.call("GET", "/api/injuries?team=all", nil, 200)["injuries"].([]any)
	}
	if len(inj) == 0 {
		t.Fatal("no injuries in 200 days")
	}
	first := inj[0].(M)
	tid := num(first["team"].(M)["id"])
	tp := c.call("GET", "/api/team/"+strconv.Itoa(tid), nil, 200)
	found := false
	for _, r := range tp["roster"].([]any) {
		row := r.(M)
		if num(row["id"]) == num(first["playerId"]) {
			found = true
			if row["injury"] == nil || num(row["injury"].(M)["gamesLeft"]) != num(first["gamesLeft"]) {
				t.Fatalf("roster row injury: %v", row)
			}
		} else if _, has := row["injury"]; has && row["injury"] == nil {
			t.Fatalf("nil injury key")
		}
	}
	if !found {
		t.Fatal("injured player not on team page")
	}
	prev := 1 << 30
	for _, x := range inj {
		g := num(x.(M)["gamesLeft"])
		if g > prev {
			t.Fatalf("injuries not sorted")
		}
		prev = g
	}
	mine := c.call("GET", "/api/injuries", nil, 200)["injuries"].([]any)
	if st := c.call("GET", "/api/state", nil, 200); num(st["injured"]) != len(mine) {
		t.Fatalf("state.injured %v vs %d", st["injured"], len(mine))
	}
	c.call("GET", "/api/injuries?team=x", nil, 400)
	c.call("GET", "/api/trade/block/"+strconv.Itoa(tid), nil, 200)
	c.call("GET", "/api/offers", nil, 200)

	// heal
	ip := "/api/player/" + strconv.Itoa(num(first["playerId"]))
	if c.call("GET", ip, nil, 200)["injury"] == nil {
		t.Fatal("player page lacks injury")
	}
	c.call("POST", "/api/options", M{"commissioner": true}, 200)
	healed := c.call("POST", "/api/admin/player/"+strconv.Itoa(num(first["playerId"])), M{"heal": true}, 200)
	if _, has := healed["injury"]; has {
		t.Fatalf("still injured: %v", healed)
	}
}

func TestContractsInSeason(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	team := c.call("GET", "/api/team/3", nil, 200)
	sum := 0
	for _, x := range team["roster"].([]any) {
		r := x.(M)
		if num(r["salary"]) <= 0 || num(r["years"]) < 1 || r["expiring"] != false {
			t.Fatalf("roster row: %v", r)
		}
		sum += num(r["salary"])
	}
	pay := team["payroll"].(M)
	if num(pay["payroll"]) != sum {
		t.Fatalf("payroll %v, roster sums to %d", pay["payroll"], sum)
	}
	if !(num(pay["cap"]) < num(pay["taxLine"]) && num(pay["taxLine"]) < num(pay["ceiling"])) || num(pay["minSalary"]) <= 0 ||
		num(pay["room"]) != num(pay["cap"])-num(pay["payroll"]) {
		t.Fatalf("payroll object: %v", pay)
	}
	pid := num(team["roster"].([]any)[0].(M)["id"])
	pl := c.call("GET", "/api/player/"+itoa(pid), nil, 200)
	if num(pl["salary"]) <= 0 || num(pl["years"]) < 1 {
		t.Fatalf("player: %v", pl)
	}
	blk := c.call("GET", "/api/trade/block/4", nil, 200)
	if blk["payroll"] == nil {
		t.Fatalf("block without payroll")
	}
	for _, x := range blk["players"].([]any) {
		if r := x.(M); num(r["salary"]) <= 0 || num(r["years"]) < 1 {
			t.Fatalf("block row: %v", r)
		}
	}
	st := c.call("GET", "/api/state", nil, 200)
	if num(st["expiring"]) != 0 || st["payroll"] == nil {
		t.Fatalf("state: %v", st)
	}
	ex := c.call("GET", "/api/contracts/expiring", nil, 200)
	if len(ex["players"].([]any)) != 0 || ex["open"] != false || ex["payroll"] == nil {
		t.Fatalf("expiring in season: %v", ex)
	}
	c.call("POST", "/api/contracts/release", M{"playerId": pid}, 409)
	c.call("POST", "/api/contracts/resign", M{"playerId": pid}, 409)
}

func TestDealSalaryRules(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	// team 4 is over the cap: it sends its cheapest player and takes my most expensive
	var cheap, dear, cheapSal, dearSal int
	cheapSal = 1 << 30
	for _, x := range c.call("GET", "/api/trade/block/4", nil, 200)["players"].([]any) {
		if r := x.(M); num(r["salary"]) < cheapSal {
			cheap, cheapSal = num(r["id"]), num(r["salary"])
		}
	}
	for _, x := range c.call("GET", "/api/trade/block/3", nil, 200)["players"].([]any) {
		if r := x.(M); num(r["salary"]) > dearSal {
			dear, dearSal = num(r["id"]), num(r["salary"])
		}
	}
	moves := []M{{"from": 3, "to": 4, "playerId": dear}, {"from": 4, "to": 3, "playerId": cheap}}
	v := c.call("POST", "/api/deal/evaluate", M{"moves": moves}, 200)
	if v["valid"] != false || v["problem"] == "" {
		t.Fatalf("expected a salary problem: %v", v)
	}
	ps := v["payrolls"].([]any)
	if len(ps) != 2 {
		t.Fatalf("payrolls: %v", ps)
	}
	want := []struct{ team, in, out int }{{3, cheapSal, dearSal}, {4, dearSal, cheapSal}}
	for i, x := range ps {
		r := x.(M)
		if num(r["team"].(M)["id"]) != want[i].team || num(r["in"]) != want[i].in || num(r["out"]) != want[i].out ||
			num(r["after"]) != num(r["before"])+num(r["in"])-num(r["out"]) {
			t.Fatalf("payroll entry %d: %v", i, r)
		}
		if num(r["before"]) != num(c.call("GET", "/api/team/"+itoa(want[i].team), nil, 200)["payroll"].(M)["payroll"]) {
			t.Fatalf("before does not match team payroll: %v", r)
		}
	}
}

func TestContractsOffseason(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 7, "team": 3}, 200)
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)
	c.call("POST", "/api/sim", M{"toEnd": true}, 200)
	c.call("POST", "/api/playoffs/start", nil, 200)
	c.call("POST", "/api/playoffs/all", nil, 200)
	c.call("POST", "/api/offseason/begin", nil, 200)

	ex := c.call("GET", "/api/contracts/expiring", nil, 200)
	list := ex["players"].([]any)
	if ex["open"] != true || len(list) < 2 {
		t.Fatalf("expiring: open=%v n=%d", ex["open"], len(list))
	}
	prev := 1 << 30
	for _, x := range list {
		r := x.(M)
		if num(r["askSalary"]) <= 0 || num(r["askYears"]) < 1 || r["tier"] == "" || r["canReSign"] == nil || r["problem"] == nil {
			t.Fatalf("expiring row: %v", r)
		}
		if num(r["ovr"]) > prev {
			t.Fatalf("not sorted by ovr")
		}
		prev = num(r["ovr"])
	}
	if st := c.call("GET", "/api/state", nil, 200); num(st["expiring"]) != len(list) {
		t.Fatalf("state.expiring %v, list %d", st["expiring"], len(list))
	}
	team := c.call("GET", "/api/team/3", nil, 200)
	flagged := 0
	for _, x := range team["roster"].([]any) {
		if x.(M)["expiring"] == true {
			flagged++
		}
	}
	if flagged != len(list) {
		t.Fatalf("roster flags %d, list %d", flagged, len(list))
	}
	has := func(roster []any, id int) bool {
		for _, x := range roster {
			if num(x.(M)["id"]) == id {
				return true
			}
		}
		return false
	}

	// re-sign the first one who can be
	resigned := -1
	for _, x := range list {
		r := x.(M)
		if r["canReSign"] != true {
			continue
		}
		out := c.call("POST", "/api/contracts/resign", M{"playerId": num(r["id"])}, 200)
		if len(out["expiring"].(M)["players"].([]any)) != len(list)-1 || num(out["state"].(M)["expiring"]) != len(list)-1 {
			t.Fatalf("resign: %v", out)
		}
		resigned = num(r["id"])
		break
	}
	if resigned < 0 {
		t.Fatalf("nobody could be re-signed")
	}
	pl := c.call("GET", "/api/player/"+itoa(resigned), nil, 200)
	if num(pl["years"]) < 1 || num(pl["salary"]) <= 0 {
		t.Fatalf("re-signed player: %v", pl)
	}
	c.call("POST", "/api/contracts/resign", M{"playerId": resigned}, 409)

	// release another expiring one
	list = c.call("GET", "/api/contracts/expiring", nil, 200)["players"].([]any)
	if len(list) == 0 {
		t.Fatalf("no one left to release")
	}
	gone := num(list[0].(M)["id"])
	out := c.call("POST", "/api/contracts/release", M{"playerId": gone}, 200)
	if len(out["expiring"].(M)["players"].([]any)) != len(list)-1 {
		t.Fatalf("release: %v", out)
	}
	if has(c.call("GET", "/api/team/3", nil, 200)["roster"].([]any), gone) {
		t.Fatalf("released player still on roster")
	}
	c.call("POST", "/api/contracts/release", M{"playerId": gone}, 409)

	// through the draft and out
	left := c.call("GET", "/api/contracts/expiring", nil, 200)["players"].([]any)
	c.call("POST", "/api/offseason/to-draft", nil, 200)
	c.call("POST", "/api/draft/auto", nil, 200)
	if ex := c.call("GET", "/api/contracts/expiring", nil, 200); ex["open"] != true {
		t.Fatalf("still open after the draft: %v", ex["open"])
	}
	c.call("POST", "/api/offseason/finish", nil, 200)
	roster := c.call("GET", "/api/team/3", nil, 200)["roster"].([]any)
	if len(roster) != 13 {
		t.Fatalf("roster %d after offseason", len(roster))
	}
	for _, x := range left {
		if has(roster, num(x.(M)["id"])) {
			t.Fatalf("ignored expiring player %v still on roster", x.(M)["name"])
		}
	}
	if !has(roster, resigned) {
		t.Fatalf("re-signed player gone")
	}
	st := c.call("GET", "/api/state", nil, 200)
	if num(st["expiring"]) != 0 {
		t.Fatalf("state.expiring %v", st["expiring"])
	}
	if ex := c.call("GET", "/api/contracts/expiring", nil, 200); ex["open"] != false || len(ex["players"].([]any)) != 0 {
		t.Fatalf("expiring after offseason: %v", ex)
	}
}

// toDraftEnd walks a new league to the end of the draft.
func toDraftEnd(c *tc) {
	c.call("POST", "/api/new", M{"seed": 11, "team": 3}, 200)
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)
	c.call("POST", "/api/sim", M{"toEnd": true}, 200)
	c.call("POST", "/api/playoffs/start", nil, 200)
	c.call("POST", "/api/playoffs/all", nil, 200)
	c.call("POST", "/api/offseason/begin", nil, 200)
	c.call("POST", "/api/offseason/to-draft", nil, 200)
	c.call("POST", "/api/draft/auto", nil, 200)
}

func TestFreeAgencyAPI(t *testing.T) {
	c := newTC(t)
	toDraftEnd(c)
	st := c.call("GET", "/api/state", nil, 200)
	if st["nextAction"].(M)["id"] != "begin_free_agency" || st["fa"] != nil {
		t.Fatalf("before FA: %v %v", st["nextAction"], st["fa"])
	}
	if f := c.call("GET", "/api/freeagents", nil, 200); f["open"] != false || len(f["players"].([]any)) != 0 || num(f["daysLeft"]) != 0 {
		t.Fatalf("FA open early: %v", f["open"])
	}
	c.call("POST", "/api/freeagents/day", nil, 409)
	out := c.call("POST", "/api/freeagents/begin", nil, 200)
	st = out["state"].(M)
	if st["phase"] != "free_agency" || st["fa"] == nil {
		t.Fatalf("begin: phase %v fa %v", st["phase"], st["fa"])
	}
	c.call("POST", "/api/freeagents/begin", nil, 409)
	if ex := c.call("GET", "/api/contracts/expiring", nil, 200); ex["open"] != false {
		t.Fatalf("expiring still open in FA")
	}
	fa := c.call("GET", "/api/freeagents", nil, 200)
	days := num(fa["days"])
	if fa["open"] != true || num(fa["day"]) != 1 || days < 1 || num(fa["daysLeft"]) != days {
		t.Fatalf("FA object: %v", fa)
	}
	if fs := st["fa"].(M); num(fs["day"]) != 1 || num(fs["days"]) != days || num(fs["daysLeft"]) != days {
		t.Fatalf("state.fa: %v", fs)
	}
	players := fa["players"].([]any)
	if len(players) < 5 {
		t.Fatalf("pool of %d", len(players))
	}
	prev := 1 << 30
	for _, x := range players {
		r := x.(M)
		if num(r["askSalary"]) <= 0 || num(r["askYears"]) < 1 || r["tier"] == "" || num(r["ovr"]) > prev {
			t.Fatalf("pool row: %v", r)
		}
		prev = num(r["ovr"])
	}

	// make room on the roster
	rosterSize := num(fa["rosterSize"])
	if num(fa["rosterCount"]) >= rosterSize {
		roster := c.call("GET", "/api/team/3", nil, 200)["roster"].([]any)
		c.call("POST", "/api/contracts/release", M{"playerId": num(roster[len(roster)-1].(M)["id"])}, 200)
		fa = c.call("GET", "/api/freeagents", nil, 200)
	}
	minSal, maxYears := num(fa["minSalary"]), num(fa["maxYears"])
	room := num(fa["payroll"].(M)["room"])
	mid := players[len(players)/2].(M)
	midID := num(mid["id"])
	fa = c.call("POST", "/api/freeagents/offer", M{"playerId": midID, "salary": minSal, "years": 1}, 200)
	findMine := func(fa M, id int) M {
		for _, x := range fa["players"].([]any) {
			if num(x.(M)["id"]) == id {
				return x.(M)
			}
		}
		return nil
	}
	row := findMine(fa, midID)
	mo, _ := row["myOffer"].(M)
	if mo == nil || num(mo["salary"]) != minSal || num(mo["years"]) != 1 || num(row["interest"]) < 1 || len(fa["offers"].([]any)) != 1 {
		t.Fatalf("offer not shown: %v", row)
	}
	firstID := num(mo["id"])
	// replacing
	fa = c.call("POST", "/api/freeagents/offer", M{"playerId": midID, "salary": minSal, "years": maxYears}, 200)
	offers := fa["offers"].([]any)
	if len(offers) != 1 || num(offers[0].(M)["years"]) != maxYears || num(offers[0].(M)["playerId"]) != midID || offers[0].(M)["name"] == "" {
		t.Fatalf("replace: %v", offers)
	}
	if num(findMine(fa, midID)["myOffer"].(M)["id"]) == firstID {
		t.Fatalf("offer id not renewed")
	}
	c.call("POST", "/api/freeagents/offer", "not json{", 400)
	// full-ask offer to a different player, or an over-cap refusal
	var other M
	for _, x := range players {
		r := x.(M)
		if num(r["id"]) != midID && num(r["askSalary"]) <= room {
			other = r
			break
		}
	}
	if other != nil {
		fa = c.call("POST", "/api/freeagents/offer", M{"playerId": num(other["id"]), "salary": num(other["askSalary"]), "years": num(other["askYears"])}, 200)
		if len(fa["offers"].([]any)) != 2 || findMine(fa, num(other["id"]))["myOffer"] == nil {
			t.Fatalf("full-ask offer missing")
		}
	} else {
		top := players[0].(M)
		if out := c.call("POST", "/api/freeagents/offer", M{"playerId": num(top["id"]), "salary": num(top["askSalary"]), "years": 3}, 409); out["error"] == nil || out["error"] == "" {
			t.Fatalf("over-cap offer: %v", out)
		}
	}
	c.call("POST", "/api/freeagents/offer", M{"playerId": midID, "salary": minSal, "years": maxYears + 1}, 409)
	// withdraw
	fa = c.call("POST", "/api/freeagents/withdraw", M{"offerId": num(fa["offers"].([]any)[0].(M)["id"])}, 200)
	want := 0
	if other != nil {
		want = 1
	}
	if len(fa["offers"].([]any)) != want {
		t.Fatalf("withdraw: %v", fa["offers"])
	}
	for _, x := range fa["offers"].([]any) {
		c.call("POST", "/api/freeagents/withdraw", M{"offerId": num(x.(M)["id"])}, 200)
	}
	c.call("POST", "/api/freeagents/withdraw", M{"offerId": 999999}, 409)
	c.call("POST", "/api/freeagents/offer", M{"playerId": midID, "salary": minSal, "years": 2}, 200)

	// run the days
	logN := 0
	for i := 0; i < days; i++ {
		d := c.call("POST", "/api/freeagents/day", nil, 200)
		f := d["freeAgency"].(M)
		if num(f["day"]) > days || num(f["day"]) < 1 {
			t.Fatalf("day %v of %d", f["day"], days)
		}
		for _, x := range d["signings"].([]any) {
			g := x.(M)
			if num(g["day"]) != i+1 || g["name"] == "" || num(g["salary"]) <= 0 || num(g["years"]) < 1 || g["team"].(M)["id"] == nil || g["mine"] == nil || g["pos"] == "" {
				t.Fatalf("signing: %v", g)
			}
		}
		n := len(f["log"].([]any))
		if n != logN+len(d["signings"].([]any)) {
			t.Fatalf("log %d, was %d, signings %d", n, logN, len(d["signings"].([]any)))
		}
		logN = n
		if num(f["daysLeft"]) != days-(i+1) || num(d["state"].(M)["fa"].(M)["daysLeft"]) != days-(i+1) {
			t.Fatalf("daysLeft %v after day %d", f["daysLeft"], i+1)
		}
	}
	if logN == 0 {
		t.Fatalf("nobody signed in free agency")
	}
	log := c.call("GET", "/api/freeagents", nil, 200)["log"].([]any)
	for i := 1; i < len(log); i++ {
		if num(log[i].(M)["day"]) > num(log[i-1].(M)["day"]) {
			t.Fatalf("log not newest first")
		}
	}
	c.call("POST", "/api/freeagents/day", nil, 409)
	c.call("POST", "/api/freeagents/offer", M{"playerId": midID, "salary": minSal, "years": 1}, 409)
	if st := c.call("GET", "/api/state", nil, 200); st["nextAction"].(M)["id"] != "start_season" {
		t.Fatalf("after last day: %v", st["nextAction"])
	}
	st = c.call("POST", "/api/offseason/finish", nil, 200)
	if st["phase"] != "regular" || st["fa"] != nil {
		t.Fatalf("after finish: phase %v fa %v", st["phase"], st["fa"])
	}
	roster := c.call("GET", "/api/team/3", nil, 200)["roster"].([]any)
	if len(roster) != rosterSize {
		t.Fatalf("roster %d, want %d", len(roster), rosterSize)
	}
	for _, x := range roster {
		if num(x.(M)["years"]) < 1 {
			t.Fatalf("roster player without contract: %v", x)
		}
	}
	if f := c.call("GET", "/api/freeagents", nil, 200); f["open"] != false || len(f["players"].([]any)) != 0 {
		t.Fatalf("FA still open after finish")
	}
}

func TestFinishWithoutFreeAgencyScreen(t *testing.T) {
	c := newTC(t)
	toDraftEnd(c)
	st := c.call("POST", "/api/offseason/finish", nil, 200)
	if st["phase"] != "regular" {
		t.Fatalf("phase %v", st["phase"])
	}
	if len(c.call("GET", "/api/team/3", nil, 200)["roster"].([]any)) != engine.RosterSize {
		t.Fatalf("short roster")
	}
}

func TestLineup(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"mode": "fictional", "seed": 7, "team": 3}, 200)
	c.call("POST", "/api/options", M{"noInjuries": true}, 200)
	lu := c.call("GET", "/api/lineup", nil, 200)
	if lu["custom"] != false || num(lu["total"]) != 240 || len(lu["starters"].([]any)) != 5 {
		t.Fatalf("fresh lineup: %v", lu)
	}
	team := c.call("GET", "/api/team/3", nil, 200)
	if len(lu["players"].([]any)) != len(team["roster"].([]any)) {
		t.Fatalf("players %d, roster %d", len(lu["players"].([]any)), len(team["roster"].([]any)))
	}
	sum := 0
	for _, x := range lu["players"].([]any) {
		p := x.(M)
		sum += num(p["minutes"])
		if num(p["minutes"]) != num(p["autoMinutes"]) {
			t.Fatalf("auto mismatch: %v", p)
		}
	}
	if sum != 240 {
		t.Fatalf("minutes sum %d", sum)
	}
	// build a valid body from the automatic lineup
	players := lu["players"].([]any)
	starters := []int{}
	for _, v := range lu["starters"].([]any) {
		starters = append(starters, num(v))
	}
	mins := map[string]int{}
	var bench []int
	for _, x := range players {
		p := x.(M)
		mins[itoa(num(p["id"]))] = num(p["minutes"])
		if num(p["starterSlot"]) < 0 {
			bench = append(bench, num(p["id"]))
		}
	}
	body := func() M {
		m := map[string]int{}
		for k, v := range mins {
			m[k] = v
		}
		return M{"starters": append([]int(nil), starters...), "minutes": m}
	}
	// swap starter 4 with bench[0]; move 10 minutes from starter 0 to the bench player (kept within 42)
	b0 := bench[0]
	old := starters[4]
	oldMin, benchMin := mins[itoa(old)], mins[itoa(b0)]
	good := body()
	good["starters"].([]int)[4] = b0
	gm := good["minutes"].(map[string]int)
	gm[itoa(b0)] = 20
	gm[itoa(old)] = oldMin + benchMin - 20
	if gm[itoa(old)] < 1 || gm[itoa(old)] > 42 {
		gm[itoa(old)] = 15
		gm[itoa(b0)] = oldMin + benchMin - 15
	}
	sm0 := gm[itoa(starters[0])]
	gm[itoa(starters[0])] = sm0 - 4
	gm[itoa(starters[1])] += 4
	if gm[itoa(starters[1])] > 42 {
		t.Skip("unlucky minutes layout")
	}
	set := c.call("POST", "/api/lineup", good, 200)
	if set["custom"] != true {
		t.Fatalf("not custom: %v", set)
	}
	for _, x := range set["players"].([]any) {
		p := x.(M)
		if num(p["minutes"]) != gm[itoa(num(p["id"]))] {
			t.Fatalf("minutes not stored: %v", p)
		}
		if num(p["id"]) == b0 && num(p["starterSlot"]) != 4 {
			t.Fatalf("bench player not starter: %v", p)
		}
	}
	// play until the user's team has a game, then check the box
	played := false
	for i := 0; i < 5 && !played; i++ {
		c.call("POST", "/api/sim", M{"days": 1}, 200)
		box := c.call("GET", "/api/box/last", nil, 200)
		if num(box["day"]) != i {
			continue
		}
		played = true
		found := false
		for _, tm := range box["teams"].([]any) {
			for _, l := range tm.(M)["lines"].([]any) {
				if num(l.(M)["id"]) == b0 {
					found = true
					if num(l.(M)["min"]) != gm[itoa(b0)] {
						t.Fatalf("box minutes %v, want %d", l, gm[itoa(b0)])
					}
				}
			}
		}
		if !found {
			t.Fatalf("player %d not in box", b0)
		}
	}
	if !played {
		t.Fatal("user team never played")
	}
	// invalid posts leave the stored lineup alone
	check := func(name string, b M) {
		out := c.call("POST", "/api/lineup", b, 400)
		if s, _ := out["error"].(string); s == "" {
			t.Fatalf("%s: no message: %v", name, out)
		}
		now := c.call("GET", "/api/lineup", nil, 200)
		for _, x := range now["players"].([]any) {
			p := x.(M)
			if num(p["minutes"]) != gm[itoa(num(p["id"]))] {
				t.Fatalf("%s: stored lineup changed", name)
			}
		}
	}
	b := body()
	b["minutes"].(map[string]int)[itoa(starters[0])]++
	check("total", b)
	b = body()
	b["minutes"].(map[string]int)[itoa(starters[0])] = 43
	b["minutes"].(map[string]int)[itoa(starters[1])] -= 43 - mins[itoa(starters[0])]
	check("max", b)
	b = body()
	b["starters"].([]int)[1] = starters[0]
	check("dup", b)
	b = body()
	mm := b["minutes"].(map[string]int)
	mm[itoa(b0)] += mm[itoa(starters[0])]
	mm[itoa(starters[0])] = 0
	check("zero starter", b)
	b = body()
	b["starters"].([]int)[0] = 99999
	check("unknown", b)
	c.call("POST", "/api/lineup", M{"starters": []int{1, 2}, "minutes": M{}}, 400)
	c.call("POST", "/api/lineup", M{"starters": starters, "minutes": M{"x": 5}}, 400)

	auto := c.call("POST", "/api/lineup/auto", nil, 200)
	if auto["custom"] != false {
		t.Fatalf("still custom: %v", auto)
	}
	for _, x := range auto["players"].([]any) {
		p := x.(M)
		if num(p["minutes"]) != num(p["autoMinutes"]) {
			t.Fatalf("auto values: %v", p)
		}
	}
}

// ---- draft lottery and scouting ----

// toDraftStart walks a new league to the draft screen (nothing picked).
func toDraftStart(c *tc, seed int) {
	c.call("POST", "/api/new", M{"seed": seed, "team": 3}, 200)
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)
	c.call("POST", "/api/sim", M{"toEnd": true}, 200)
	c.call("POST", "/api/playoffs/start", nil, 200)
	c.call("POST", "/api/playoffs/all", nil, 200)
	c.call("POST", "/api/offseason/begin", nil, 200)
	c.call("POST", "/api/offseason/to-draft", nil, 200)
}

// checkNoProspectTruth: prospect rows never carry ovr or pot keys.
func (c *tc) checkNoProspectTruth(rows []any) {
	c.t.Helper()
	for _, r := range rows {
		m := r.(M)
		if _, ok := m["ovr"]; ok {
			c.t.Fatalf("prospect row has ovr: %v", m)
		}
		if _, ok := m["pot"]; ok {
			c.t.Fatalf("prospect row has pot: %v", m)
		}
	}
}

func TestDraftLotteryAndScouting(t *testing.T) {
	c := newTC(t)
	toDraftStart(c, 11)
	d := c.call("GET", "/api/draft", nil, 200)
	pros := d["prospects"].([]any)
	if len(pros) == 0 {
		t.Fatal("no prospects")
	}
	c.checkNoProspectTruth(pros)
	for _, r := range pros {
		m := r.(M)
		for _, k := range []string{"ovrLo", "ovrHi", "potLo", "potHi", "scoutLevel"} {
			if _, ok := m[k]; !ok {
				t.Fatalf("prospect missing %s: %v", k, m)
			}
		}
		if num(m["scoutLevel"]) != 0 {
			t.Fatalf("scout level: %v", m)
		}
	}
	lot := d["lottery"].([]any)
	if len(lot) != 14 {
		t.Fatalf("lottery rows: %d", len(lot))
	}
	seen, sum := map[int]bool{}, 0.0
	for i, r := range lot {
		m := r.(M)
		sl := num(m["slot"])
		if sl != i+1 || seen[sl] {
			t.Fatalf("lottery slots: %v", lot)
		}
		seen[sl] = true
		sum += m["odds"].(float64)
		if m["team"] == nil || m["owner"] == nil {
			t.Fatalf("lottery row: %v", m)
		}
	}
	if sum < 99 || sum > 101 {
		t.Fatalf("odds sum %v", sum)
	}
	if len(d["picks"].([]any)) != num(d["total"]) {
		t.Fatalf("picks %d total %v", len(d["picks"].([]any)), d["total"])
	}
	sc := d["scouting"].(M)
	if num(sc["points"]) != num(sc["perDraft"]) || num(sc["maxLevel"]) != engine.MaxScoutLevel {
		t.Fatalf("scouting: %v", sc)
	}
}

func TestProspectSecrecy(t *testing.T) {
	c := newTC(t)
	leaked := false
	for _, seed := range []int{11, 12, 13} {
		toDraftStart(c, seed)
		pool := c.s.lg.Draft.Pool
		d := c.call("GET", "/api/draft", nil, 200)
		rows := d["prospects"].([]any)
		// the listed order must not be the true-overall order
		ids := []int{}
		for _, r := range rows {
			ids = append(ids, num(r.(M)["id"]))
		}
		byOvr := append([]*engine.Player(nil), pool...)
		sort.SliceStable(byOvr, func(i, j int) bool { return byOvr[i].Overall() > byOvr[j].Overall() })
		for i := range ids {
			if ids[i] != byOvr[i].ID {
				leaked = true
				break
			}
		}
		if seed != 11 {
			continue
		}
		pts := num(d["scouting"].(M)["points"])
		for _, p := range pool[:3] {
			truthO, truthP := p.Overall(), p.Potential
			path := "/api/player/" + itoa(p.ID)
			prevW := 1 << 30
			var cur M
			for lvl := 0; lvl <= 3; lvl++ {
				if lvl > 0 {
					c.call("POST", "/api/draft/scout", M{"playerId": p.ID}, 200)
				}
				cur = nil
				for _, r := range c.call("GET", "/api/draft", nil, 200)["prospects"].([]any) {
					if num(r.(M)["id"]) == p.ID {
						cur = r.(M)
					}
				}
				if cur == nil || num(cur["scoutLevel"]) != lvl {
					t.Fatalf("level %d: %v", lvl, cur)
				}
				if truthO < num(cur["ovrLo"]) || truthO > num(cur["ovrHi"]) || truthP < num(cur["potLo"]) || truthP > num(cur["potHi"]) {
					t.Fatalf("range misses truth (%d/%d): %v", truthO, truthP, cur)
				}
				w := num(cur["ovrHi"]) - num(cur["ovrLo"]) + num(cur["potHi"]) - num(cur["potLo"])
				if lvl > 0 && w >= prevW {
					t.Fatalf("level %d did not narrow: %d vs %d", lvl, w, prevW)
				}
				prevW = w
				pl := c.call("GET", path, nil, 200)
				if pl["prospect"] != true || pl["editable"] != false {
					t.Fatalf("prospect flags: %v", pl)
				}
				sv := pl["scout"].(M)
				if num(sv["level"]) != lvl || num(pl["ovr"]) != (num(sv["ovrLo"])+num(sv["ovrHi"])+1)/2 ||
					num(pl["pot"]) != (num(sv["potLo"])+num(sv["potHi"])+1)/2 {
					t.Fatalf("player midpoints: %v", pl)
				}
				rs := pl["ratings"].([]any)
				if len(rs) != 7 {
					t.Fatalf("ratings: %v", rs)
				}
				for _, x := range rs {
					r := x.(M)
					if num(r["lo"]) > num(r["hi"]) || num(r["value"]) < num(r["lo"]) || num(r["value"]) > num(r["hi"]) {
						t.Fatalf("rating range: %v", r)
					}
				}
			}
			c.call("POST", "/api/draft/scout", M{"playerId": p.ID}, 409)
		}
		after := num(c.call("GET", "/api/draft", nil, 200)["scouting"].(M)["points"])
		if pts-after != 9 {
			t.Fatalf("points used: %d -> %d", pts, after)
		}
	}
	if !leaked {
		t.Fatal("prospect order matches true overall order in every league")
	}
}

func TestScoutPointsRunOut(t *testing.T) {
	c := newTC(t)
	toDraftStart(c, 11)
	d := c.call("GET", "/api/draft", nil, 200)
	per := num(d["scouting"].(M)["perDraft"])
	pool := c.s.lg.Draft.Pool
	n := 0
	for _, p := range pool {
		for l := 0; l < engine.MaxScoutLevel && n < per; l++ {
			c.call("POST", "/api/draft/scout", M{"playerId": p.ID}, 200)
			n++
		}
	}
	if n != per {
		t.Fatalf("scouted %d of %d", n, per)
	}
	out := c.call("POST", "/api/draft/scout", M{"playerId": pool[len(pool)-1].ID}, 409)
	if msg, _ := out["error"].(string); msg == "" {
		t.Fatalf("empty message: %v", out)
	}
	if num(c.call("GET", "/api/draft", nil, 200)["scouting"].(M)["points"]) != 0 {
		t.Fatal("points left")
	}
}

func TestProspectAdminAndDrafted(t *testing.T) {
	c := newTC(t)
	toDraftStart(c, 11)
	c.call("POST", "/api/options", M{"commissioner": true}, 200)
	p := c.s.lg.Draft.Pool[0]
	id := p.ID
	c.call("POST", "/api/admin/player/"+itoa(id), M{"age": 20}, 409)
	if pl := c.call("GET", "/api/player/"+itoa(id), nil, 200); pl["editable"] != false || pl["prospect"] != true {
		t.Fatalf("prospect editable: %v", pl)
	}
	c.call("POST", "/api/draft/auto", nil, 200)
	pl := c.call("GET", "/api/player/"+itoa(id), nil, 200)
	if _, ok := pl["prospect"]; ok {
		t.Fatalf("drafted player still a prospect: %v", pl)
	}
	if num(pl["ovr"]) != p.Overall() || num(pl["pot"]) != p.Potential {
		t.Fatalf("drafted ovr %v, want %d", pl["ovr"], p.Overall())
	}
	d := c.call("GET", "/api/draft", nil, 200)
	if len(d["lottery"].([]any)) != 14 || len(d["prospects"].([]any)) != 0 && d["done"] != true {
		t.Fatalf("done draft: %v", d["done"])
	}
}
