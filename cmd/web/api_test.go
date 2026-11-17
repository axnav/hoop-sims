package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
	c.call("POST", "/api/contracts/release", M{"playerId": 99999999}, 409) // not on the team
	c.call("POST", "/api/contracts/resign", M{"playerId": pid}, 409)
}

func TestReleaseInSeason(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	// a new league has no free agents; put one in the pool
	lg := c.s.lg
	fa := lg.Teams[4].Roster[12]
	lg.Teams[4].Roster = lg.Teams[4].Roster[:12]
	fa.TeamID, fa.Contract = -1, engine.Contract{}
	lg.FreeAgents = append(lg.FreeAgents, fa)
	team := c.call("GET", "/api/team/3", nil, 200)
	before := num(team["payroll"].(M)["payroll"])
	if d, ok := team["deadMoney"].([]any); !ok || len(d) != 0 {
		t.Fatalf("deadMoney should be an empty list: %v", team["deadMoney"])
	}
	pl := team["roster"].([]any)[0].(M)
	pid, sal, yrs := num(pl["id"]), num(pl["salary"]), num(pl["years"])
	pv := c.call("GET", "/api/contracts/release/"+itoa(pid), nil, 200)
	for _, k := range []string{"allowed", "reason", "deadSalary", "deadYears", "refill", "refillName"} {
		if _, ok := pv[k]; !ok {
			t.Fatalf("preview lacks %q: %v", k, pv)
		}
	}
	if pv["allowed"] != true || num(pv["deadSalary"]) != sal || num(pv["deadYears"]) != yrs || pv["refill"] != true || pv["refillName"] == "" {
		t.Fatalf("preview: %v (salary %d years %d)", pv, sal, yrs)
	}
	c.checkNoImpact("preview", pv)
	if bad := c.call("GET", "/api/contracts/release/99999999", nil, 200); bad["allowed"] != false || bad["reason"] == "" {
		t.Fatalf("preview of a stranger: %v", bad)
	}
	out := c.call("POST", "/api/contracts/release", M{"playerId": pid}, 200)
	c.checkNoImpact("release", out)
	team = c.call("GET", "/api/team/3", nil, 200)
	onRoster := false
	for _, r := range team["roster"].([]any) {
		onRoster = onRoster || num(r.(M)["id"]) == pid
	}
	if onRoster || len(team["roster"].([]any)) != 13 {
		t.Fatalf("roster after release: %d players", len(team["roster"].([]any)))
	}
	dm := team["deadMoney"].([]any)
	if len(dm) != 1 || num(dm[0].(M)["salary"]) != sal || num(dm[0].(M)["years"]) != yrs || dm[0].(M)["name"] == "" {
		t.Fatalf("deadMoney: %v", dm)
	}
	pay := team["payroll"].(M)
	if num(pay["payroll"]) != before-sal+1200+sal || num(pay["dead"]) != sal {
		t.Fatalf("payroll %v, before %d", pay, before)
	}
	c.checkNoImpact("team", team)
	c.call("POST", "/api/contracts/release", M{"playerId": pid}, 409)
	tx := c.call("GET", "/api/transactions", nil, 200)["transactions"].([]any)
	last := tx[0].(M)
	if len(last["released"].([]any)) != 1 || len(last["signed"].([]any)) != 1 {
		t.Fatalf("transaction: %v", last)
	}
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

func TestDevChangeAndLiveReplay(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 11, "team": 3}, 200)
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)
	// Before any progression: no keys.
	team := c.call("GET", "/api/team/3", nil, 200)
	if _, ok := team["roster"].([]any)[0].(M)["ovrChange"]; ok {
		t.Fatal("ovrChange before first progression")
	}
	// Live replay of the regular-season game.
	r := c.call("POST", "/api/sim", M{"days": 1}, 200)
	day := r["days"].([]any)[0].(M)
	if day["hasReplay"] != true {
		t.Fatalf("day result without hasReplay: %v", day)
	}
	if box := c.call("GET", "/api/box/last", nil, 200); box["hasReplay"] != true {
		t.Fatalf("box/last hasReplay: %v", box["hasReplay"])
	}
	ev := c.call("GET", "/api/game/last/events", nil, 200)
	if evs, _ := ev["events"].([]any); len(evs) == 0 {
		t.Fatal("no events for last game")
	}
	// Full offseason.
	c.call("POST", "/api/sim", M{"toEnd": true}, 200)
	c.call("POST", "/api/playoffs/start", nil, 200)
	c.call("POST", "/api/playoffs/all", nil, 200)
	c.call("POST", "/api/offseason/begin", nil, 200)
	c.call("POST", "/api/offseason/to-draft", nil, 200)
	d := c.call("GET", "/api/draft", nil, 200)
	for _, p := range d["prospects"].([]any) {
		m := p.(M)
		if _, ok := m["ovrChange"]; ok {
			t.Fatalf("prospect has ovrChange: %v", m)
		}
		if _, ok := m["potChange"]; ok {
			t.Fatalf("prospect has potChange: %v", m)
		}
		if _, ok := m["ratingChanges"]; ok {
			t.Fatalf("prospect has ratingChanges: %v", m)
		}
	}
	for i := 0; i < 200; i++ {
		d := c.call("GET", "/api/draft", nil, 200)
		if d["done"] == true {
			break
		}
		if d["userOnClock"] == true {
			c.call("POST", "/api/draft/pick", M{"playerId": num(d["prospects"].([]any)[0].(M)["id"])}, 200)
			continue
		}
		c.call("POST", "/api/draft/step", M{"max": 7}, 200)
	}
	c.call("POST", "/api/offseason/finish", nil, 200)
	team = c.call("GET", "/api/team/3", nil, 200)
	found := false
	for _, p := range team["roster"].([]any) {
		m := p.(M)
		if _, ok := m["ovrChange"]; !ok {
			continue
		}
		if _, ok := m["potChange"]; !ok {
			t.Fatalf("potChange missing: %v", m)
		}
		pg := c.call("GET", "/api/player/"+itoa(num(m["id"])), nil, 200)
		rc, ok := pg["ratingChanges"].(M)
		if !ok || len(rc) != len(engine.RatingKeys()) {
			t.Fatalf("ratingChanges: %v", pg["ratingChanges"])
		}
		found = true
		break
	}
	if !found {
		t.Fatal("no roster row has ovrChange after a full offseason")
	}
}

func TestInSeasonDevAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 4, "team": 1}, 200)
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)
	o := c.call("POST", "/api/options", M{"noInSeasonDev": true}, 200)
	if o["noInSeasonDev"] != true {
		t.Fatalf("options: %v", o)
	}
	o = c.call("POST", "/api/options", M{"noInSeasonDev": false}, 200)
	if o["noInSeasonDev"] != false || c.call("GET", "/api/options", nil, 200)["noInSeasonDev"] != false {
		t.Fatalf("options: %v", o)
	}
	r := c.call("POST", "/api/sim", M{"days": 16}, 200)
	dev, ok := r["development"].([]any)
	if !ok {
		t.Fatalf("development is not a list: %v", r["development"])
	}
	for _, d := range dev {
		row := d.(M)
		if row["name"] == nil || num(row["change"]) == 0 {
			t.Fatalf("bad development row: %v", row)
		}
	}
	team := c.call("GET", "/api/team/1", nil, 200)
	season := false
	var pid int
	for _, row := range team["roster"].([]any) {
		if row.(M)["changeScope"] == "season" {
			season, pid = true, num(row.(M)["id"])
		}
	}
	if !season {
		t.Fatalf("no roster row with changeScope season")
	}
	pl := c.call("GET", "/api/player/"+itoa(pid), nil, 200)
	if pl["changeScope"] != "season" || pl["ratingChanges"] == nil {
		t.Fatalf("player page: %v", pl["changeScope"])
	}
	// Days that are not a step give an empty list, not null.
	r = c.call("POST", "/api/sim", M{"days": 1}, 200)
	if l, ok := r["development"].([]any); !ok || len(l) != 0 {
		t.Fatalf("development after a plain day: %v", r["development"])
	}
}

func TestTeamsListAndStatus(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 7, "team": 3}, 200)
	valid := map[string]bool{"contender": true, "buyer": true, "seller": true, "rebuilding": true}
	checkStatus := func(where string, v any) {
		st, _ := v.(M)
		if st == nil || !valid[st["key"].(string)] || st["label"] == "" || st["wants"] == "" {
			t.Fatalf("%s: bad status %v", where, v)
		}
	}
	rows := c.call("GET", "/api/teams", nil, 200)["teams"].([]any)
	if len(rows) != 30 {
		t.Fatalf("%d teams", len(rows))
	}
	users := 0
	for i, x := range rows {
		r := x.(M)
		for _, k := range []string{"team", "w", "l", "confRank", "payroll", "capRoom", "rosterSize", "top8", "star", "status", "offers", "isUser"} {
			if _, ok := r[k]; !ok {
				t.Fatalf("team row %d missing %q", i, k)
			}
		}
		if num(r["team"].(M)["id"]) != i {
			t.Fatalf("row %d out of order", i)
		}
		if r["star"].(M)["name"] == "" {
			t.Fatalf("row %d star", i)
		}
		if r["isUser"] == true {
			users++
		}
		checkStatus("teams row", r["status"])
	}
	if users != 1 {
		t.Fatalf("%d user rows", users)
	}
	checkStatus("team page", c.call("GET", "/api/team/4", nil, 200)["status"])
	checkStatus("trade block", c.call("GET", "/api/trade/block/4", nil, 200)["status"])
}

func TestRestrictedAndExceptionAPI(t *testing.T) {
	c := newTC(t)
	toDraftEnd(c)
	lg := c.s.lg
	u := lg.UserTeamID
	if st := c.call("GET", "/api/state", nil, 200); num(st["pendingSheets"]) != 0 {
		t.Fatalf("pendingSheets before FA: %v", st["pendingSheets"])
	}
	// an expiring young player of the user is restricted
	young, old := lg.Teams[u].Roster[0], lg.Teams[u].Roster[1]
	young.Contract.Years, young.Experience = 0, 1
	old.Contract.Years, old.Experience = 0, 9
	seen := 0
	for _, x := range c.call("GET", "/api/contracts/expiring", nil, 200)["players"].([]any) {
		r := x.(M)
		b, ok := r["restricted"].(bool)
		if !ok {
			t.Fatalf("expiring row without restricted: %v", r)
		}
		if num(r["id"]) == young.ID && b || num(r["id"]) == old.ID && !b {
			seen++
		}
	}
	if seen != 2 {
		t.Fatalf("restricted flags wrong (%d of 2 right)", seen)
	}
	c.call("POST", "/api/freeagents/begin", nil, 200)
	fa := c.call("GET", "/api/freeagents", nil, 200)
	exc, ok := fa["exception"].(M)
	if !ok {
		t.Fatalf("no exception object: %v", fa["exception"])
	}
	for _, k := range []string{"kind", "label", "maxSalary", "maxYears", "used", "reason"} {
		if _, ok := exc[k]; !ok {
			t.Errorf("exception lacks %q", k)
		}
	}
	if sh, ok := fa["sheets"].([]any); !ok || len(sh) != 0 {
		t.Fatalf("sheets should be an empty list: %v", fa["sheets"])
	}
	row := func(fa M, id int) M {
		for _, x := range fa["players"].([]any) {
			if num(x.(M)["id"]) == id {
				return x.(M)
			}
		}
		return nil
	}
	if r := row(fa, young.ID); r == nil || r["restricted"] != true || r["mine"] != true || r["rightsTeam"].(M)["id"] == nil || num(r["minYears"]) != 1 {
		t.Fatalf("user's restricted player row: %v", r)
	}
	if r := row(fa, old.ID); r == nil || r["restricted"] != false || r["rightsTeam"] != nil {
		t.Fatalf("unrestricted row: %v", r)
	}
	// another team's restricted player needs two years
	other := lg.FreeAgents[len(lg.FreeAgents)-1]
	off := (u + 1) % len(lg.Teams)
	lg.FA.Rights[other.ID] = (u + 2) % len(lg.Teams)
	fa = c.call("GET", "/api/freeagents", nil, 200)
	if r := row(fa, other.ID); r["restricted"] != true || r["mine"] != false || num(r["minYears"]) != 2 {
		t.Fatalf("other team's restricted row: %v", r)
	}
	c.call("POST", "/api/freeagents/offer", M{"playerId": other.ID, "salary": engine.Cap.MinSalary, "years": 1}, 409)
	delete(lg.FA.Rights, other.ID)

	// two offer sheets against the user (built directly: rival bids are hidden)
	if len(lg.Teams[u].Roster) > engine.RosterSize-2 {
		lg.Teams[u].Roster = lg.Teams[u].Roster[:engine.RosterSize-2]
	}
	for _, p := range lg.Teams[u].Roster {
		p.Contract = engine.Contract{Salary: 5000, Years: 2}
	}
	if len(lg.Teams[off].Roster) > 9 {
		lg.Teams[off].Roster = lg.Teams[off].Roster[:9]
	}
	for _, p := range lg.Teams[off].Roster {
		p.Contract = engine.Contract{Salary: engine.Cap.MinSalary, Years: 2}
	}
	ps := []*engine.Player{lg.FreeAgents[0], lg.FreeAgents[1]}
	for i, p := range ps {
		lg.FA.Rights[p.ID] = u
		lg.FA.NextOfferID++
		lg.FA.Offers = append(lg.FA.Offers, engine.FAOffer{ID: lg.FA.NextOfferID, Team: off, PlayerID: p.ID, Salary: 9000 + 1000*i, Years: 3, Day: 0})
		lg.FA.Sheets = append(lg.FA.Sheets, engine.OfferSheet{PlayerID: p.ID, Team: off, Salary: 9000 + 1000*i, Years: 3, Day: 0})
	}
	st := c.call("GET", "/api/state", nil, 200)
	if num(st["pendingSheets"]) != 2 {
		t.Fatalf("pendingSheets: %v", st["pendingSheets"])
	}
	fa = c.call("GET", "/api/freeagents", nil, 200)
	sheets := fa["sheets"].([]any)
	if len(sheets) != 2 {
		t.Fatalf("sheets: %v", sheets)
	}
	for _, x := range sheets {
		r := x.(M)
		for _, k := range []string{"playerId", "name", "pos", "age", "ovr", "pot", "team", "salary", "years", "canMatch", "problem"} {
			if _, ok := r[k]; !ok {
				t.Errorf("sheet lacks %q: %v", k, r)
			}
		}
		if r["name"] == "" || r["canMatch"] != true || r["team"].(M)["id"] == nil {
			t.Errorf("sheet row: %v", r)
		}
	}
	// nothing moves until the sheets are answered
	if out := c.call("POST", "/api/freeagents/day", nil, 409); !strings.Contains(out["error"].(string), "offer sheet") {
		t.Errorf("advance error: %v", out["error"])
	}
	c.call("POST", "/api/offseason/finish", nil, 409)
	c.call("POST", "/api/freeagents/sheet", M{"playerId": -5, "match": true}, 409)
	c.call("POST", "/api/freeagents/sheet", "nope{", 400)

	// match the first: he signs with the user at the sheet's terms
	out := c.call("POST", "/api/freeagents/sheet", M{"playerId": ps[0].ID, "match": true}, 200)
	sg := out["signings"].([]any)
	if len(sg) != 1 || out["voided"] != false || out["state"] == nil || out["freeAgency"] == nil {
		t.Fatalf("match response: %v", out)
	}
	g := sg[0].(M)
	if g["matched"] != true || g["via"] != "own" || g["mine"] != true || g["sheetFrom"].(M)["id"] == nil || num(g["salary"]) != 9000 {
		t.Fatalf("matched row: %v", g)
	}
	if ps[0].TeamID != u || num(out["state"].(M)["pendingSheets"]) != 1 {
		t.Fatalf("after matching: team %d", ps[0].TeamID)
	}
	// decline the second: the offering team signs him
	out = c.call("POST", "/api/freeagents/sheet", M{"playerId": ps[1].ID, "match": false}, 200)
	g = out["signings"].([]any)[0].(M)
	if g["matched"] != false || g["via"] != "room" || num(g["team"].(M)["id"]) != off || g["mine"] != false || num(out["state"].(M)["pendingSheets"]) != 0 {
		t.Fatalf("declined row: %v", g)
	}
	if _, has := g["sheetFrom"]; has {
		t.Error("sheetFrom on an unmatched signing")
	}
	fa = out["freeAgency"].(M)
	log := fa["log"].([]any)
	if len(log) < 2 || log[0].(M)["via"] == nil || log[0].(M)["matched"] == nil {
		t.Fatalf("log rows lack matched/via: %v", log)
	}
	c.call("POST", "/api/freeagents/sheet", M{"playerId": ps[1].ID, "match": true}, 409)
	c.call("POST", "/api/freeagents/day", nil, 200)

	// an exception offer shows its way of signing
	if len(lg.Teams[u].Roster) > engine.RosterSize-1 {
		lg.Teams[u].Roster = lg.Teams[u].Roster[:engine.RosterSize-1]
	}
	for _, p := range lg.Teams[u].Roster {
		p.Contract = engine.Contract{Salary: (engine.Cap.SalaryCap - 5000) / len(lg.Teams[u].Roster), Years: 2} // $5.0M of room
	}
	var star *engine.Player
	for _, p := range lg.FreeAgents {
		if _, r := lg.RightsTeam(p.ID); !r {
			star = p
			break
		}
	}
	c.call("POST", "/api/freeagents/offer", M{"playerId": star.ID, "salary": 12000, "years": 2}, 200)
	fa = c.call("GET", "/api/freeagents", nil, 200)
	offers := fa["offers"].([]any)
	if len(offers) != 1 || offers[0].(M)["via"] != "exception" {
		t.Fatalf("exception offer: %v", offers)
	}
	if e := fa["exception"].(M); e["kind"] == nil {
		t.Fatalf("exception obj: %v", e)
	}
}

func (c *tc) rawGet(path string) (int, []byte, http.Header) {
	c.t.Helper()
	resp, err := http.Get(c.ts.URL + path)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.Bytes(), resp.Header
}

func hasPhotoKey(v any) bool {
	switch x := v.(type) {
	case M:
		for k, e := range x {
			if k == "photo" || hasPhotoKey(e) {
				return true
			}
		}
	case []any:
		for _, e := range x {
			if hasPhotoKey(e) {
				return true
			}
		}
	}
	return false
}

func TestPhotosFictional(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"mode": "fictional", "seed": 5, "team": 3}, 200)
	team := c.call("GET", "/api/team/3", nil, 200)
	pid := num(team["roster"].([]any)[0].(M)["id"])
	for _, p := range []string{"/api/photo/" + itoa(pid), "/api/photo/999999", "/api/photo/abc", "/api/photo/" + itoa(pid) + "/credit"} {
		if code, _, _ := c.rawGet(p); code != 404 {
			t.Fatalf("%s: status %d, want 404", p, code)
		}
	}
	for _, p := range []string{"/api/team/3", "/api/player/" + itoa(pid), "/api/lineup", "/api/freeagents"} {
		if hasPhotoKey(c.call("GET", p, nil, 200)) {
			t.Fatalf("%s: fictional league row has photo", p)
		}
	}
}

func TestPhotosReal(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"mode": "fictional", "seed": 5, "team": 3}, 200)
	dir := t.TempDir()
	roster := c.s.lg.Teams[3].Roster
	a, b := roster[0], roster[1]
	a.RealID, b.RealID = "testpl01", "testpl02"
	body := []byte("\xff\xd8\xff fake jpeg")
	if err := os.WriteFile(filepath.Join(dir, "testpl01.jpg"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	csvText := "player_id,name,file,wikipedia_page,licence,artist,descriptionurl\n" +
		"testpl01,Test,testpl01.jpg,p,CC BY 2.0,\"<a href=\"\"x\"\">Jane Doe</a>\",https://example.org/file\n"
	os.WriteFile(filepath.Join(dir, "credits.csv"), []byte(csvText), 0o644)
	c.s.photoDir = dir
	c.s.loadPhotos()

	rows := c.call("GET", "/api/team/3", nil, 200)["roster"].([]any)
	for _, r := range rows {
		row := r.(M)
		id := num(row["id"])
		_, has := row["photo"]
		if want := id == a.ID; has != want {
			t.Fatalf("player %d: photo=%v, want %v", id, has, want)
		}
	}
	if pg := c.call("GET", "/api/player/"+itoa(a.ID), nil, 200); pg["photo"] != true {
		t.Fatalf("player page lacks photo: %v", pg["photo"])
	}
	code, got, h := c.rawGet("/api/photo/" + itoa(a.ID))
	if code != 200 || !bytes.Equal(got, body) || !strings.Contains(h.Get("Cache-Control"), "max-age") {
		t.Fatalf("photo: %d %q %v", code, got, h)
	}
	if code, _, _ := c.rawGet("/api/photo/" + itoa(b.ID)); code != 404 {
		t.Fatalf("player without file: %d", code)
	}
	code, got, _ = c.rawGet("/api/photo/" + itoa(a.ID) + "/credit")
	var cr M
	json.Unmarshal(got, &cr)
	if code != 200 || cr["artist"] != "Jane Doe" || cr["licence"] != "CC BY 2.0" || cr["source"] != "https://example.org/file" {
		t.Fatalf("credit: %d %v", code, cr)
	}
	c.checkNoImpact("credit", cr)
	c.checkNoImpact("team", c.call("GET", "/api/team/3", nil, 200))
}

func TestCleanArtist(t *testing.T) {
	in := "File:Jason Kidd Nets coach.jpg: <a href=\"x\">Keith Allison</a>\nderivative work: Chrishmt0423"
	if got, want := cleanArtist(in), "Keith Allison; derivative work: Chrishmt0423"; got != want {
		t.Fatalf("cleanArtist = %q, want %q", got, want)
	}
}

func TestMotivationAndExtensionAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 9, "team": 3}, 200)
	lg := c.s.lg
	u := lg.UserTeamID
	levels := map[string]bool{"Low": true, "Medium": true, "High": true}
	checkMot := func(where string, v any) {
		m, ok := v.(M)
		if !ok {
			t.Fatalf("%s: no motivation", where)
		}
		for _, k := range []string{"money", "winning", "role", "loyalty"} {
			if s, _ := m[k].(string); !levels[s] {
				t.Fatalf("%s: motivation %v", where, m)
			}
		}
	}

	// every roster player of the user has a stance, others do not
	team := c.call("GET", "/api/team/"+itoa(u), nil, 200)
	for _, x := range team["roster"].([]any) {
		st, ok := x.(M)["stance"].(M)
		if !ok || st["key"] == "" || st["label"] == "" || st["reasons"] != nil {
			t.Fatalf("roster stance: %v", x)
		}
	}
	other := c.call("GET", "/api/team/"+itoa((u+1)%len(lg.Teams)), nil, 200)
	for _, x := range other["roster"].([]any) {
		if x.(M)["stance"] != nil {
			t.Fatalf("stance on another team's row: %v", x)
		}
	}

	// player page: motivation for all, stance and extension for the user's players
	mine := lg.Teams[u].Roster
	pl := c.call("GET", "/api/player/"+itoa(mine[0].ID), nil, 200)
	checkMot("player", pl["motivation"])
	st, ok := pl["stance"].(M)
	if !ok || st["label"] == "" || st["reasons"] == nil || pl["extension"] != nil {
		t.Fatalf("player stance: %v", pl)
	}
	foreign := lg.Teams[(u+1)%len(lg.Teams)].Roster[0]
	fp := c.call("GET", "/api/player/"+itoa(foreign.ID), nil, 200)
	checkMot("foreign player", fp["motivation"])
	if fp["stance"] != nil {
		t.Errorf("stance on a player of another team")
	}

	// extension route: find one who will talk and one who will not
	for _, p := range mine {
		p.Contract = engine.Contract{Salary: engine.MarketSalary(p) * 9 / 10 / 100 * 100, Years: 1}
	}
	var yes, no *engine.Player
	for _, p := range mine {
		r := c.call("GET", "/api/contracts/extension/"+itoa(p.ID), nil, 200)
		if r["ok"] == true {
			if yes == nil && num(r["salary"]) <= num(r["maxSalary"]) {
				yes = p
				if num(r["maxYears"]) != engine.Cap.MaxContractYears || num(r["current"]) != p.Contract.Salary || num(r["maxSalary"]) < num(r["current"]) {
					t.Fatalf("extension ask: %v", r)
				}
			}
		} else if r["reason"] == "" {
			t.Fatalf("refusal without reason: %v", r)
		} else if no == nil {
			no = p
		}
	}
	if yes == nil {
		t.Fatal("nobody on the roster would extend")
	}
	t.Logf("extend: %s; refuse: %v", yes.Name, no != nil)
	if no != nil {
		c.call("POST", "/api/contracts/extend", M{"playerId": no.ID, "salary": engine.Cap.MaxSalary, "years": 2}, 409)
	}
	ask := c.call("GET", "/api/contracts/extension/"+itoa(yes.ID), nil, 200)
	c.call("POST", "/api/contracts/extend", M{"playerId": yes.ID, "salary": num(ask["salary"]) - 100, "years": 2}, 409)
	c.call("POST", "/api/contracts/extend", M{"playerId": yes.ID, "salary": num(ask["salary"]), "years": 9}, 409)
	out := c.call("POST", "/api/contracts/extend", M{"playerId": yes.ID, "salary": num(ask["salary"]), "years": 3}, 200)
	if msg, _ := out["message"].(string); msg == "" || out["state"] == nil {
		t.Fatalf("extend response: %v", out)
	}
	pl = c.call("GET", "/api/player/"+itoa(yes.ID), nil, 200)
	ext, ok := pl["extension"].(M)
	if !ok || num(ext["salary"]) != num(ask["salary"]) || num(ext["years"]) != 3 {
		t.Fatalf("player extension: %v", pl["extension"])
	}
	if r := c.call("GET", "/api/contracts/extension/"+itoa(yes.ID), nil, 200); r["ok"] != false {
		t.Errorf("second ask: %v", r)
	}
	c.call("POST", "/api/contracts/extend", M{"playerId": yes.ID, "salary": num(ask["salary"]), "years": 3}, 409)
	team = c.call("GET", "/api/team/"+itoa(u), nil, 200)
	flagged := 0
	for _, x := range team["roster"].([]any) {
		if x.(M)["extended"] == true {
			flagged++
		}
	}
	if flagged != 1 {
		t.Errorf("extended rows: %d", flagged)
	}
	c.call("POST", "/api/contracts/extend", M{"playerId": 999999, "salary": 5000, "years": 2}, 409)
	c.call("GET", "/api/contracts/extension/x", nil, 400)
}

func TestMotivationOnFreeAgentRows(t *testing.T) {
	c := newTC(t)
	toDraftEnd(c)
	ex := c.call("GET", "/api/contracts/expiring", nil, 200)
	for _, x := range ex["players"].([]any) {
		r := x.(M)
		st, ok := r["stance"].(M)
		if !ok || st["key"] == "" {
			t.Fatalf("expiring row without stance: %v", r)
		}
		if st["key"] == "test" && (r["canReSign"] == true || r["problem"] != "He wants to test free agency.") {
			t.Fatalf("a test player can be re-signed: %v", r)
		}
	}
	c.call("POST", "/api/freeagents/begin", nil, 200)
	fa := c.call("GET", "/api/freeagents", nil, 200)
	rows := fa["players"].([]any)
	if len(rows) == 0 {
		t.Fatal("no free agents")
	}
	for _, x := range rows {
		m, ok := x.(M)["motivation"].(M)
		if !ok || m["money"] == nil || m["winning"] == nil || m["role"] == nil || m["loyalty"] == nil {
			t.Fatalf("free-agent row without motivation: %v", x)
		}
	}
}

func TestTrainingRoute(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	lg := c.s.lg
	u := lg.UserTeamID
	mine := lg.Teams[u].Roster[0]
	pl := c.call("GET", "/api/player/"+itoa(mine.ID), nil, 200)
	tr, ok := pl["training"].(M)
	if !ok || tr["focus"] != "balanced" || tr["editable"] != true || len(tr["options"].([]any)) != len(engine.AllFocuses()) {
		t.Fatalf("training block: %v", pl["training"])
	}
	skills := tr["skills"].([]any)
	if len(skills) != len(engine.RatingKeys()) {
		t.Fatalf("skills: %v", skills)
	}
	for i, x := range skills {
		sk := x.(M)
		if sk["rating"] != engine.RatingKeys()[i].Key || sk["speed"] == "" || sk["stepsFor2"] == "" {
			t.Fatalf("skill line: %v", sk)
		}
		if _, ok := sk["open"].(bool); !ok {
			t.Fatalf("skill open: %v", sk)
		}
	}
	// good
	out := c.call("POST", "/api/training", M{"playerId": mine.ID, "focus": "shooting"}, 200)
	if msg, _ := out["message"].(string); msg == "" || out["training"].(M)["focus"] != "shooting" || out["training"].(M)["editable"] != true {
		t.Fatalf("response: %v", out)
	}
	if mine.Focus != engine.FocusShooting {
		t.Fatalf("focus not set: %v", mine.Focus)
	}
	team := c.call("GET", "/api/team/"+itoa(u), nil, 200)
	for _, x := range team["roster"].([]any) {
		r := x.(M)
		want := "balanced"
		if num(r["id"]) == mine.ID {
			want = "shooting"
		}
		if r["focus"] != want {
			t.Fatalf("roster focus: %v", r)
		}
	}
	other := c.call("GET", "/api/team/"+itoa((u+1)%len(lg.Teams)), nil, 200)
	for _, x := range other["roster"].([]any) {
		if x.(M)["focus"] != nil {
			t.Fatalf("focus on another team's row: %v", x)
		}
	}
	// bad key, other team's player, unknown player
	c.call("POST", "/api/training", M{"playerId": mine.ID, "focus": "dunking"}, 400)
	foreign := lg.Teams[(u+1)%len(lg.Teams)].Roster[0]
	c.call("POST", "/api/training", M{"playerId": foreign.ID, "focus": "body"}, 409)
	if foreign.Focus != engine.FocusBalanced {
		t.Error("another team's player changed")
	}
	fp := c.call("GET", "/api/player/"+itoa(foreign.ID), nil, 200)
	if fp["training"].(M)["editable"] != false {
		t.Errorf("foreign player editable: %v", fp["training"])
	}
	c.call("POST", "/api/training", M{"playerId": 999999, "focus": "body"}, 409)

	// prospects: no training block, route refuses
	toDraftStart(c, 11)
	pr := c.s.lg.Draft.Pool[0]
	pp := c.call("GET", "/api/player/"+itoa(pr.ID), nil, 200)
	if pp["training"] != nil {
		t.Fatalf("training on a prospect: %v", pp["training"])
	}
	c.call("POST", "/api/training", M{"playerId": pr.ID, "focus": "body"}, 404)
}

func TestBadgesAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	lg := c.s.lg

	// Catalogue.
	cat := c.call("GET", "/api/badges", nil, 200)
	list := cat["badges"].([]any)
	if len(list) != 40 {
		t.Fatalf("catalogue has %d badges", len(list))
	}
	keys := map[string]bool{}
	for _, x := range list {
		b := x.(M)
		for _, k := range []string{"key", "name", "group", "kind", "rule", "effect"} {
			if s, _ := b[k].(string); s == "" {
				t.Fatalf("catalogue entry without %s: %v", k, b)
			}
		}
		if strings.Contains(strings.ToLower(b["rule"].(string)+b["effect"].(string)), "impact") {
			t.Fatalf("catalogue mentions the hidden number: %v", b)
		}
		keys[b["key"].(string)] = true
	}
	if !keys["rim_finisher"] || !keys["mentor"] {
		t.Fatalf("catalogue keys: %v", keys)
	}

	// Player page: an array on every player, with the contract fields.
	best := lg.Teams[0].Roster[0]
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			if lg.BadgeCount(p) > lg.BadgeCount(best) {
				best = p
			}
		}
	}
	if lg.BadgeCount(best) < 2 {
		t.Fatal("no player with two badges in the test league")
	}
	pl := c.call("GET", "/api/player/"+itoa(best.ID), nil, 200)
	bl, ok := pl["badges"].([]any)
	if !ok || len(bl) != lg.BadgeCount(best) {
		t.Fatalf("badges on player page: %v", pl["badges"])
	}
	lastRank := 6
	rank := map[string]int{"legend": 5, "hof": 4, "gold": 3, "silver": 2, "bronze": 1, "": 0}
	for _, x := range bl {
		b := x.(M)
		tier, _ := b["tier"].(string)
		r, known := rank[tier]
		if !known || r > lastRank {
			t.Fatalf("badge order or tier: %v", bl)
		}
		lastRank = r
		kind := b["kind"]
		if (kind == "personality") != (tier == "") || (kind != "rating" && kind != "performance" && kind != "personality") {
			t.Fatalf("kind/tier: %v", b)
		}
		g := b["group"]
		if g != "finishing" && g != "shooting" && g != "playmaking" && g != "defense" && g != "rebounding" && g != "general" && g != "personality" {
			t.Fatalf("group: %v", b)
		}
		for _, k := range []string{"key", "name", "rule", "effect"} {
			if s, _ := b[k].(string); s == "" {
				t.Fatalf("badge without %s: %v", k, b)
			}
		}
		if _, ok := b["next"].(string); !ok {
			t.Fatalf("badge without next: %v", b)
		}
		if strings.Contains(strings.ToLower(b["rule"].(string)+b["effect"].(string)), "impact") {
			t.Fatalf("badge text mentions the hidden number: %v", b)
		}
	}
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			if bs := lg.BadgesOf(p); len(bs) > 0 && bs[0].Tier == engine.Legend {
				lp := c.call("GET", "/api/player/"+itoa(p.ID), nil, 200)
				b, _ := json.Marshal(lp["badges"].([]any)[0])
				t.Logf("example badge object (live player page): %s", b)
				goto done
			}
		}
	}
done:
	// A player with none gets an empty array, not null.
	none := lg.Teams[0].Roster[0]
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			if lg.BadgeCount(p) == 0 {
				none = p
			}
		}
	}
	np := c.call("GET", "/api/player/"+itoa(none.ID), nil, 200)
	if arr, ok := np["badges"].([]any); !ok || len(arr) != 0 {
		t.Fatalf("player without badges: %v", np["badges"])
	}

	// Roster rows carry badgeCount.
	team := c.call("GET", "/api/team/"+itoa(best.TeamID), nil, 200)
	seen := false
	for _, x := range team["roster"].([]any) {
		r := x.(M)
		n, ok := r["badgeCount"].(float64)
		if !ok {
			t.Fatalf("roster row without badgeCount: %v", r)
		}
		if num(r["id"]) == best.ID {
			seen = true
			if int(n) != lg.BadgeCount(best) {
				t.Fatalf("badgeCount %v, want %d", n, lg.BadgeCount(best))
			}
		}
	}
	if !seen {
		t.Fatal("player not on his team page")
	}

	// Leaders: filter by badge. Play some days so there are leaders.
	lg.SimDays(engine.MinGPLeaders + 5)
	all := c.call("GET", "/api/leaders", nil, 200)
	if len(all["pts"].([]any)) != 10 {
		t.Fatalf("plain leaders: %v", all["pts"])
	}
	c.call("GET", "/api/leaders?badge=nonsense", nil, 400)
	for _, key := range []string{"rim_finisher", "star", "floor_general", "rim_protector"} {
		f := c.call("GET", "/api/leaders?badge="+key, nil, 200)
		for _, stat := range []string{"pts", "reb", "ast"} {
			rows := f[stat].([]any)
			if len(rows) > 10 {
				t.Fatalf("%s %s: %d rows", key, stat, len(rows))
			}
			for _, x := range rows {
				p := lg.FindPlayer(num(x.(M)["playerId"]))
				if p == nil || !lg.HasBadge(p, key) {
					t.Fatalf("%s %s: row without the badge: %v", key, stat, x)
				}
			}
		}
	}

	// Prospects: no badges key, no badgeCount anywhere.
	toDraftStart(c, 11)
	pr := c.s.lg.Draft.Pool[0]
	pp := c.call("GET", "/api/player/"+itoa(pr.ID), nil, 200)
	if _, has := pp["badges"]; has {
		t.Fatalf("badges on a prospect: %v", pp["badges"])
	}
	board := c.call("GET", "/api/draft", nil, 200)
	for _, x := range board["prospects"].([]any) {
		if _, has := x.(M)["badgeCount"]; has {
			t.Fatalf("badgeCount on a prospect row: %v", x)
		}
	}
}

func TestCoachAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"mode": "fictional", "seed": 5, "team": 3}, 200)
	c.call("POST", "/api/options", M{"noInjuries": true}, 200)

	g := c.call("GET", "/api/coach/3", nil, 200)
	co := g["coach"].(map[string]any)
	for _, k := range []string{"name", "age", "offense", "defense", "development", "system", "systemName"} {
		if _, ok := co[k]; !ok {
			t.Fatalf("coach lacks %q: %v", k, co)
		}
	}
	if g["editable"] != true || g["canChangeSystem"] != true || g["canHire"] != false || g["favourite"] != true {
		t.Fatalf("user team flags: %v", g)
	}
	if g["system"] != co["system"] || g["systemName"] != co["systemName"] {
		t.Errorf("fresh team should run the favourite: %v vs %v", g["system"], co["system"])
	}
	systems := g["systems"].([]any)
	if len(systems) != 6 {
		t.Fatalf("%d systems", len(systems))
	}
	keys := map[string]bool{}
	for _, x := range systems {
		sy := x.(map[string]any)
		for _, k := range []string{"key", "name", "description", "pace", "threes", "starUsage", "defense", "wants", "teamFit"} {
			if _, ok := sy[k]; !ok {
				t.Fatalf("system lacks %q: %v", k, sy)
			}
		}
		tf := sy["teamFit"].(map[string]any)
		if _, ok := tf["score"].(float64); !ok || tf["grade"] == "" {
			t.Errorf("system teamFit %v", tf)
		}
		keys[sy["key"].(string)] = true
	}
	for _, k := range []string{"balanced", "pace_space", "run_gun", "inside_out", "star_led", "defense_first"} {
		if !keys[k] {
			t.Errorf("missing system %s", k)
		}
	}
	players := g["players"].([]any)
	team := c.call("GET", "/api/team/3", nil, 200)
	if len(players) != len(team["roster"].([]any)) {
		t.Fatalf("players %d, roster %d", len(players), len(team["roster"].([]any)))
	}
	last := 1000
	for _, x := range players {
		p := x.(map[string]any)
		m := num(p["minutes"])
		if m > last {
			t.Errorf("players not sorted by minutes: %v after %d", m, last)
		}
		last = m
		f := p["fit"].(map[string]any)
		if _, ok := f["score"].(float64); !ok || f["grade"] == "" || len(f["reasons"].([]any)) > 3 {
			t.Errorf("player fit %v", f)
		}
	}
	// The team page and the lineup carry the fit.
	if tc := team["coach"].(map[string]any); tc["name"] != co["name"] || tc["system"] != g["system"] || tc["systemName"] == "" {
		t.Errorf("team page coach %v", tc)
	}
	if tf := team["teamFit"].(map[string]any); tf["score"] != g["teamFit"].(map[string]any)["score"] {
		t.Errorf("team page fit %v vs %v", tf, g["teamFit"])
	}
	lu := c.call("GET", "/api/lineup", nil, 200)
	if lu["teamFit"].(map[string]any)["grade"] == "" {
		t.Errorf("lineup teamFit %v", lu["teamFit"])
	}
	for _, x := range lu["players"].([]any) {
		if f, ok := x.(map[string]any)["fit"].(map[string]any); !ok || f["grade"] == "" {
			t.Fatalf("lineup row without fit: %v", x)
		}
	}
	// Another team is readable but not editable.
	o := c.call("GET", "/api/coach/5", nil, 200)
	if o["editable"] != false || o["canChangeSystem"] != false || o["canHire"] != false {
		t.Errorf("computer team flags: %v", o)
	}
	c.call("GET", "/api/coach/99", nil, 404)

	// Change the system (any time, also in the season).
	other := "defense_first"
	if co["system"] == other {
		other = "pace_space"
	}
	post := c.call("POST", "/api/coach/system", M{"system": other}, 200)
	if post["system"] != other || post["favourite"] != false {
		t.Errorf("after change: %v %v", post["system"], post["favourite"])
	}
	again := c.call("GET", "/api/coach/3", nil, 200)
	if again["system"] != other || again["teamFit"] == nil {
		t.Errorf("GET after change: %v", again["system"])
	}
	c.call("POST", "/api/coach/system", M{"system": "nonsense"}, 400)
	c.call("POST", "/api/coach/system", M{}, 400)
	// Going back to the favourite clears the flag.
	back := c.call("POST", "/api/coach/system", M{"system": co["system"]}, 200)
	if back["favourite"] != true {
		t.Errorf("favourite after returning: %v", back["favourite"])
	}

	// No hiring in the season.
	pool := c.call("GET", "/api/coach/pool", nil, 200)
	if pool["open"] != false || len(pool["candidates"].([]any)) != 0 {
		t.Errorf("pool in the season: %v", pool)
	}
	c.call("POST", "/api/coach/hire", M{"id": 0}, 409)
	// The game still plays with the saved coach and system.
	c.call("POST", "/api/sim", M{"action": "sim_day"}, 200)
}

func TestCoachHiringInOffseason(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"mode": "fictional", "seed": 6, "team": 2}, 200)
	c.call("POST", "/api/options", M{"noInjuries": true}, 200)
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)
	c.call("POST", "/api/coach/hire", M{"id": 0}, 409)
	c.call("POST", "/api/sim", M{"toEnd": true}, 200)
	c.call("POST", "/api/playoffs/start", nil, 200)
	c.call("POST", "/api/playoffs/all", nil, 200)
	c.call("POST", "/api/coach/hire", M{"id": 0}, 409)
	c.call("POST", "/api/offseason/begin", nil, 200)

	pool := c.call("GET", "/api/coach/pool", nil, 200)
	cands := pool["candidates"].([]any)
	if pool["open"] != true || len(cands) != engine.CoachTuning.PoolSize {
		t.Fatalf("pool %v", pool)
	}
	for _, x := range cands {
		cd := x.(map[string]any)
		for _, k := range []string{"id", "name", "age", "offense", "defense", "development", "system", "systemName"} {
			if _, ok := cd[k]; !ok {
				t.Fatalf("candidate lacks %q: %v", k, cd)
			}
		}
	}
	g := c.call("GET", "/api/coach/2", nil, 200)
	if g["canHire"] != true {
		t.Errorf("canHire in the offseason: %v", g["canHire"])
	}
	pick := cands[4].(map[string]any)
	c.call("POST", "/api/coach/hire", M{"id": 999}, 409)
	c.call("POST", "/api/coach/hire", M{}, 400)
	hired := c.call("POST", "/api/coach/hire", M{"id": pick["id"]}, 200)
	hc := hired["coach"].(map[string]any)
	if hc["name"] != pick["name"] || hc["offense"] != pick["offense"] || hired["favourite"] != true || hired["system"] != pick["system"] {
		t.Errorf("hired %v, picked %v", hired, pick)
	}
	after := c.call("GET", "/api/coach/pool", nil, 200)
	if len(after["candidates"].([]any)) != engine.CoachTuning.PoolSize-1 {
		t.Errorf("pool after hiring: %d", len(after["candidates"].([]any)))
	}
	for _, x := range after["candidates"].([]any) {
		if x.(map[string]any)["id"] == pick["id"] {
			t.Error("hired candidate still in the pool")
		}
	}
	// The offseason finishes, the next season starts, the pool is gone.
	c.call("POST", "/api/offseason/to-draft", nil, 200)
	c.call("POST", "/api/draft/auto", nil, 200)
	c.call("POST", "/api/offseason/finish", nil, 200)
	pool = c.call("GET", "/api/coach/pool", nil, 200)
	if pool["open"] != false || len(pool["candidates"].([]any)) != 0 {
		t.Errorf("pool after the offseason: %v", pool)
	}
	c.call("POST", "/api/coach/hire", M{"id": pick["id"]}, 409)
	g = c.call("GET", "/api/coach/2", nil, 200)
	if g["coach"].(map[string]any)["name"] != pick["name"] {
		t.Error("the hired coach must stay")
	}
}

func TestMoraleAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"mode": "fictional", "seed": 5, "team": 3}, 200)
	lg := c.s.lg
	other := lg.Teams[7].Roster[0]
	mine := lg.Teams[3].Roster[0]
	mine.Morale, mine.TradeRequest = -35, lg.Year*100+2
	other.Morale = 22

	pl := c.call("GET", "/api/player/"+strconv.Itoa(mine.ID), nil, 200)
	mo, ok := pl["morale"].(map[string]any)
	if !ok {
		t.Fatalf("player page lacks morale: %v", pl)
	}
	for _, k := range []string{"score", "label", "trend", "reasons", "tradeRequest"} {
		if _, ok := mo[k]; !ok {
			t.Errorf("morale lacks %q: %v", k, mo)
		}
	}
	if mo["score"] != 15.0 || mo["label"] != "Furious" || mo["tradeRequest"] != true {
		t.Errorf("morale object %v", mo)
	}
	if rs, ok := mo["reasons"].([]any); !ok || len(rs) > 4 {
		t.Errorf("reasons %v", mo["reasons"])
	}
	// A player on another team shows his mood too.
	po := c.call("GET", "/api/player/"+strconv.Itoa(other.ID), nil, 200)
	if m2, ok := po["morale"].(map[string]any); !ok || m2["score"] != 72.0 || m2["label"] != "Happy" || m2["tradeRequest"] != false {
		t.Errorf("other player morale %v", po["morale"])
	}

	// Roster rows: brief morale, the flag only while a request is active.
	team := c.call("GET", "/api/team/3", nil, 200)
	tm, ok := team["teamMorale"].(map[string]any)
	if !ok || tm["label"] == nil || tm["score"] == nil {
		t.Fatalf("team page lacks teamMorale: %v", team["teamMorale"])
	}
	flagged := 0
	for _, x := range team["roster"].([]any) {
		row := x.(map[string]any)
		m, ok := row["morale"].(map[string]any)
		if !ok || len(m) != 2 || m["score"] == nil || m["label"] == nil {
			t.Fatalf("roster row morale %v", row["morale"])
		}
		if row["id"] == float64(mine.ID) {
			if row["tradeRequest"] != true {
				t.Errorf("requester row lacks tradeRequest: %v", row)
			}
			flagged++
		} else if _, has := row["tradeRequest"]; has {
			t.Errorf("tradeRequest on a row without a request: %v", row)
		}
	}
	if flagged != 1 {
		t.Errorf("requester seen %d times in the roster", flagged)
	}

	// Trade screen rows.
	blk := c.call("GET", "/api/trade/block/3", nil, 200)
	seen := false
	for _, x := range blk["players"].([]any) {
		row := x.(map[string]any)
		if row["id"] == float64(mine.ID) && row["tradeRequest"] == true {
			seen = true
		}
	}
	if !seen {
		t.Error("trade block row lacks tradeRequest")
	}

	// Events appear in the transactions list.
	lg.Events = append(lg.Events, engine.LeagueEvent{Year: lg.Year, Day: 4, Type: engine.EvTradeRequest, PlayerID: mine.ID, Name: mine.Name, TeamID: 3})
	tx := c.call("GET", "/api/transactions", nil, 200)
	evs, ok := tx["events"].([]any)
	if !ok || len(evs) != 1 {
		t.Fatalf("events %v", tx["events"])
	}
	e := evs[0].(map[string]any)
	if e["type"] != "trade_request" || e["day"] != 5.0 || e["playerId"] != float64(mine.ID) || e["text"] == "" {
		t.Errorf("event %v", e)
	}

	// Free agents have no morale; prospects neither.
	fa := lg.Teams[5].Roster[12]
	lg.Teams[5].Roster = lg.Teams[5].Roster[:12]
	fa.TeamID = -1
	lg.FreeAgents = append(lg.FreeAgents, fa)
	pf := c.call("GET", "/api/player/"+strconv.Itoa(fa.ID), nil, 200)
	if _, has := pf["morale"]; has {
		t.Errorf("free agent has morale: %v", pf["morale"])
	}
	// The mood is never taken from the browser: there is no way to set it.
	c.call("POST", "/api/player/"+strconv.Itoa(mine.ID), M{"morale": 50}, 404)
}

func TestDraftNightAPI(t *testing.T) {
	sawOffer := false
	for seed := 1; seed <= 12 && !sawOffer; seed++ {
		c := newTC(t)
		toDraftStart(c, seed)
		st := c.call("GET", "/api/trade/status", nil, 200)
		if st["open"] != true || st["reason"] != "Draft night: trades are open." {
			t.Fatalf("status: %v", st)
		}
		blk := c.call("GET", "/api/trade/block/3", nil, 200)
		picks := blk["picks"].([]any)
		hasSlot := false
		for _, p := range picks {
			if l, _ := p.(M)["label"].(string); strings.Contains(l, " round, pick ") {
				hasSlot = true
			}
		}
		if !hasSlot {
			t.Fatalf("no slot label in %v", picks)
		}
		// step the draft to the user's turn (or the end)
		for i := 0; i < 70; i++ {
			d := c.call("GET", "/api/draft", nil, 200)
			if d["done"] == true || d["userOnClock"] == true {
				break
			}
			c.call("POST", "/api/draft/step", M{"max": 1}, 200)
		}
		for round := 0; round < 3; round++ {
			d := c.call("GET", "/api/draft", nil, 200)
			if d["done"] == true {
				break
			}
			if d["userOnClock"] != true {
				c.call("POST", "/api/draft/step", M{"max": 70}, 200)
				continue
			}
			offers := c.call("GET", "/api/offers", nil, 200)["offers"].([]any)
			if len(offers) > 0 {
				o := offers[0].(M)
				if num(o["draftPick"]) == 0 {
					t.Fatalf("offer has no draftPick: %v", o)
				}
				mv := o["moves"].([]any)[0].(M)["pick"].(M)
				id := itoa(num(o["id"]))
				c.call("POST", "/api/offers/"+id+"/accept", nil, 200)
				d2 := c.call("GET", "/api/draft", nil, 200)
				var found bool
				for _, p := range d2["picks"].([]any) {
					pm := p.(M)
					if num(pm["pick"]) == num(o["draftPick"]) {
						found = true
						if num(pm["team"].(M)["id"]) != num(o["team"].(M)["id"]) {
							t.Fatalf("pick %v did not move to the offering team: %v", mv, pm)
						}
					}
				}
				if !found {
					t.Fatal("pick not in draft list")
				}
				sawOffer = true
				break
			}
			// no offer here: make the pick and carry on
			pros := c.call("GET", "/api/draft", nil, 200)["prospects"].([]any)
			c.call("POST", "/api/draft/pick", M{"playerId": num(pros[0].(M)["id"])}, 200)
		}
	}
	if !sawOffer {
		t.Fatal("no trade-up offer in 12 seeds")
	}
}

func TestPlayersSearch(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	lg := c.s.lg
	// Make one free agent so team=fa has a result.
	t0 := lg.Teams[0]
	fa := t0.Roster[len(t0.Roster)-1]
	t0.Roster = t0.Roster[:len(t0.Roster)-1]
	fa.TeamID = -1
	lg.FreeAgents = append(lg.FreeAgents, fa)
	onTeams := 0
	for _, tm := range lg.Teams {
		onTeams += len(tm.Roster)
	}
	rows := func(m M) []M {
		out := []M{}
		for _, x := range m["players"].([]any) {
			out = append(out, x.(M))
		}
		return out
	}

	all := c.call("GET", "/api/players?limit=200", nil, 200)
	if num(all["total"]) != onTeams+len(lg.FreeAgents) || len(rows(all)) != 200 {
		t.Fatalf("total %v, want %d, got %d rows", all["total"], onTeams+len(lg.FreeAgents), len(rows(all)))
	}
	first := rows(all)[0]
	for _, k := range []string{"id", "name", "pos", "age", "ovr", "pot", "team", "salary", "years", "badgeCount", "morale"} {
		if _, ok := first[k]; !ok {
			t.Fatalf("row without %s: %v", k, first)
		}
	}
	b, _ := json.Marshal(first)
	t.Logf("example row: %s", b)
	// Default sort: overall, high to low.
	prev := 1000
	for _, r := range rows(all) {
		if num(r["ovr"]) > prev {
			t.Fatal("not sorted by overall")
		}
		prev = num(r["ovr"])
	}

	// Name match, case-insensitive.
	target := lg.Teams[5].Roster[0]
	part := strings.ToUpper(target.Name[1 : len(target.Name)-1])
	m := c.call("GET", "/api/players?q="+strings.ReplaceAll(part, " ", "%20"), nil, 200)
	found := false
	for _, r := range rows(m) {
		if !strings.Contains(strings.ToLower(r["name"].(string)), strings.ToLower(part)) {
			t.Fatalf("name %v does not contain %q", r["name"], part)
		}
		found = found || num(r["id"]) == target.ID
	}
	if !found {
		t.Fatalf("%q not found", target.Name)
	}
	if foldName("Dončić") != "doncic" {
		t.Fatal("accent folding")
	}

	// Team filter.
	tm := c.call("GET", "/api/players?team=5", nil, 200)
	if num(tm["total"]) != len(lg.Teams[5].Roster) {
		t.Fatalf("team total %v", tm["total"])
	}
	for _, r := range rows(tm) {
		if num(r["team"].(M)["id"]) != 5 {
			t.Fatalf("wrong team: %v", r)
		}
	}
	c.call("GET", "/api/players?team=999", nil, 400)

	// Free agents.
	fas := c.call("GET", "/api/players?team=fa", nil, 200)
	if num(fas["total"]) != len(lg.FreeAgents) {
		t.Fatalf("fa total %v", fas["total"])
	}
	for _, r := range rows(fas) {
		if r["team"] != nil {
			t.Fatalf("free agent with a team: %v", r)
		}
	}

	// Position (primary or alternate), overall range.
	pg := c.call("GET", "/api/players?pos=PG&limit=200", nil, 200)
	for _, r := range rows(pg) {
		if !strings.Contains(r["pos"].(string), "PG") {
			t.Fatalf("pos %v", r["pos"])
		}
	}
	c.call("GET", "/api/players?pos=XX", nil, 400)
	rg := c.call("GET", "/api/players?minOvr=60&maxOvr=70&limit=200", nil, 200)
	if num(rg["total"]) == 0 {
		t.Fatal("no players 60-70")
	}
	for _, r := range rows(rg) {
		if o := num(r["ovr"]); o < 60 || o > 70 {
			t.Fatalf("ovr %d outside range", o)
		}
	}
	ag := c.call("GET", "/api/players?minAge=30&maxAge=33&limit=200", nil, 200)
	for _, r := range rows(ag) {
		if a := num(r["age"]); a < 30 || a > 33 {
			t.Fatalf("age %d", a)
		}
	}
	yl := c.call("GET", "/api/players?yearsLeft=2&maxSalary=5000&limit=200", nil, 200)
	for _, r := range rows(yl) {
		if num(r["years"]) != 2 || num(r["salary"]) > 5000 {
			t.Fatalf("contract filter: %v", r)
		}
	}

	// Badge filter and unknown badge.
	c.call("GET", "/api/players?badge=nonsense", nil, 400)
	bd := c.call("GET", "/api/players?badge=star&limit=200", nil, 200)
	if num(bd["total"]) == 0 {
		t.Fatal("nobody has the star badge")
	}
	for _, r := range rows(bd) {
		if p := lg.FindPlayer(num(r["id"])); p == nil || !lg.HasBadge(p, "star") {
			t.Fatalf("row without the badge: %v", r)
		}
	}
	// Morale and trade request.
	c.call("GET", "/api/players?morale=nonsense", nil, 400)
	mo := c.call("GET", "/api/players?morale=content&limit=200", nil, 200)
	for _, r := range rows(mo) {
		if r["morale"].(M)["label"] != "Content" {
			t.Fatalf("morale %v", r["morale"])
		}
	}
	rq := lg.Teams[7].Roster[0]
	rq.TradeRequest = lg.Year*100 + 3
	tr := c.call("GET", "/api/players?tradeRequest=1", nil, 200)
	if num(tr["total"]) != 1 || num(rows(tr)[0]["id"]) != rq.ID || rows(tr)[0]["tradeRequest"] != true {
		t.Fatalf("tradeRequest: %v", tr)
	}

	// Sort and paging.
	var names []string
	for off := 0; ; off += 100 {
		pgm := c.call("GET", "/api/players?sort=name&dir=asc&limit=100&offset="+itoa(off), nil, 200)
		if num(pgm["total"]) != onTeams+len(lg.FreeAgents) {
			t.Fatalf("paged total %v", pgm["total"])
		}
		for _, r := range rows(pgm) {
			names = append(names, foldName(r["name"].(string)))
		}
		if len(rows(pgm)) < 100 {
			break
		}
	}
	if len(names) != onTeams+len(lg.FreeAgents) || !sort.StringsAreSorted(names) {
		t.Fatalf("paging: %d names, sorted %v", len(names), sort.StringsAreSorted(names))
	}
	ages := c.call("GET", "/api/players?sort=age&limit=5", nil, 200)
	if len(rows(ages)) != 5 || num(rows(ages)[0]["age"]) > num(rows(ages)[4]["age"]) {
		t.Fatalf("age sort: %v", ages)
	}
	c.call("GET", "/api/players?sort=impact", nil, 400)
	c.call("GET", "/api/players?sort=badges&dir=desc&limit=3", nil, 200)
	c.call("GET", "/api/players?sort=salary&limit=3", nil, 200)
	far := c.call("GET", "/api/players?offset=100000", nil, 200)
	if len(rows(far)) != 0 || num(far["total"]) != onTeams+len(lg.FreeAgents) {
		t.Fatalf("offset past the end: %v", far)
	}

	// Draft prospects are never listed.
	toDraftStart(c, 11)
	pool := c.s.lg.Draft.Pool
	if len(pool) == 0 {
		t.Fatal("no prospects")
	}
	want := 0
	for _, tm := range c.s.lg.Teams {
		want += len(tm.Roster)
	}
	want += len(c.s.lg.FreeAgents)
	ids := map[int]bool{}
	for _, p := range pool {
		ids[p.ID] = true
	}
	for off := 0; off < want; off += 200 {
		pr := c.call("GET", "/api/players?limit=200&offset="+itoa(off), nil, 200)
		if num(pr["total"]) != want {
			t.Fatalf("draft-time total %v, want %d", pr["total"], want)
		}
		for _, r := range rows(pr) {
			if ids[num(r["id"])] {
				t.Fatalf("prospect listed: %v", r)
			}
		}
	}
	byName := c.call("GET", "/api/players?q="+strings.ReplaceAll(pool[0].Name, " ", "%20"), nil, 200)
	for _, r := range rows(byName) {
		if ids[num(r["id"])] {
			t.Fatalf("prospect found by name: %v", r)
		}
	}
}

func TestMetaRealSeasons(t *testing.T) {
	c := newTC(t)
	meta := c.call("GET", "/api/meta", nil, 200)
	if meta["realAvailable"] != true {
		t.Skip("real data absent")
	}
	rs := meta["realSeasons"].([]any)
	if len(rs) != 16 {
		t.Fatalf("realSeasons has %d entries", len(rs))
	}
	has := func(e M, abbr string) bool {
		for _, x := range e["teams"].([]any) {
			if x.(M)["abbr"] == abbr {
				return true
			}
		}
		return false
	}
	for i, x := range rs {
		e := x.(M)
		if int(e["season"].(float64)) != 2026-i {
			t.Fatalf("entry %d is season %v, want newest first", i, e["season"])
		}
		if len(e["teams"].([]any)) != 30 {
			t.Fatalf("season %v has %d teams", e["season"], len(e["teams"].([]any)))
		}
		for id, tm := range e["teams"].([]any) {
			if int(tm.(M)["id"].(float64)) != id {
				t.Fatalf("season %v: team at position %d has id %v", e["season"], id, tm.(M)["id"])
			}
		}
	}
	if !has(rs[0].(M), "BRK") || has(rs[0].(M), "NJN") {
		t.Fatal("2026 should have BRK and not NJN")
	}
	if !has(rs[15].(M), "NJN") || has(rs[15].(M), "BRK") {
		t.Fatal("2011 should have NJN and not BRK")
	}
	if len(meta["realTeams"].([]any)) != 30 {
		t.Fatal("realTeams missing")
	}
}

func TestNewRealLeagueWithSeason(t *testing.T) {
	c := newTC(t)
	if c.call("GET", "/api/meta", nil, 200)["realAvailable"] != true {
		t.Skip("real data absent")
	}
	st := c.call("POST", "/api/new", M{"mode": "real", "seed": 2, "team": 3, "season": 2020}, 200)
	if int(st["realSeason"].(float64)) != 2020 || st["mode"] != "real" || int(st["year"].(float64)) != 2020 {
		t.Fatalf("state after season 2020: realSeason %v year %v", st["realSeason"], st["year"])
	}
	teams := c.call("GET", "/api/teams", nil, 200)["teams"].([]any)
	brk := false
	for _, x := range teams {
		if x.(M)["team"].(M)["abbr"] == "BRK" {
			brk = true
		}
	}
	if !brk {
		t.Fatal("no BRK team in the 2020 league")
	}
	if got := c.call("GET", "/api/state", nil, 200)["realSeason"]; int(got.(float64)) != 2020 {
		t.Fatalf("/api/state realSeason = %v", got)
	}
	// Default is 2011.
	st = c.call("POST", "/api/new", M{"mode": "real", "seed": 2, "team": 3}, 200)
	if int(st["realSeason"].(float64)) != 2011 {
		t.Fatalf("default realSeason = %v", st["realSeason"])
	}
	// Fictional league: 0.
	st = c.call("POST", "/api/new", M{"mode": "fictional", "seed": 2, "team": 3}, 200)
	if int(st["realSeason"].(float64)) != 0 {
		t.Fatalf("fictional realSeason = %v", st["realSeason"])
	}
	c.call("POST", "/api/new", M{"mode": "real", "seed": 2, "team": 3, "season": 1990}, 400)
	c.call("POST", "/api/new", M{"mode": "real", "seed": 2, "team": 3, "season": 2027}, 400)
}
