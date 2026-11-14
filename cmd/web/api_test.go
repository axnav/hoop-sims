package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"hoopsim/engine"
)

type tc struct {
	t  *testing.T
	ts *httptest.Server
}

func newTC(t *testing.T) *tc {
	s := &server{savePath: filepath.Join(t.TempDir(), "league.json"), realDir: "../../data/real", webDir: t.TempDir()}
	ts := httptest.NewServer(s.routes())
	t.Cleanup(ts.Close)
	return &tc{t, ts}
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

func TestTrades(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	if c.call("GET", "/api/trade/status", nil, 200)["open"] != true {
		t.Fatal("window should be open at season start")
	}
	myBest, myWorst, _ := blockIDs(c, 3)
	_, theirWorst, theirBest := blockIDs(c, 4)
	// lopsided: my worst for their best
	v := c.call("POST", "/api/trade/evaluate", M{"team": 4, "give": []int{myWorst}, "get": []int{theirBest}}, 200)
	if v["valid"] != true || v["accept"] != false || v["reason"] == "" {
		t.Fatalf("lopsided offer: %v", v)
	}
	c.call("POST", "/api/trade/execute", M{"team": 4, "give": []int{myWorst}, "get": []int{theirBest}}, 409)
	c.call("POST", "/api/trade/counter", M{"team": 4, "give": []int{myWorst}, "get": []int{theirBest}}, 200)
	// generous: my best for their worst
	v = c.call("POST", "/api/trade/evaluate", M{"team": 4, "give": []int{myBest}, "get": []int{theirWorst}}, 200)
	if v["accept"] != true {
		t.Fatalf("generous offer refused: %v", v)
	}
	r := c.call("POST", "/api/trade/execute", M{"team": 4, "give": []int{myBest}, "get": []int{theirWorst}}, 200)
	if len(r["moved"].([]any)) != 2 || r["state"] == nil {
		t.Fatalf("execute: %v", r)
	}
	for _, id := range []int{3, 4} {
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
	_, theirWorst, _ := blockIDs(c, 4)
	_, _, third := blockIDs(c, 5)
	// two-team: my first-round pick for their worst player
	_, myWorst, _ := blockIDs(c, 3)
	moves := []M{{"from": 3, "to": 4, "pick": myPick(0)}, {"from": 3, "to": 4, "playerId": myWorst}, {"from": 4, "to": 3, "playerId": theirWorst}}
	v := c.call("POST", "/api/deal/evaluate", M{"moves": moves}, 200)
	if v["valid"] != true || len(v["teams"].([]any)) != 1 {
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
	if v["valid"] != true || len(v["teams"].([]any)) != 2 {
		t.Fatalf("three-team: %v", v)
	}
	// malformed moves
	c.call("POST", "/api/deal/evaluate", M{"moves": []M{{"from": 3, "to": 4}}}, 400)
	c.call("POST", "/api/deal/evaluate", M{"moves": []M{{"from": 3, "to": 4, "playerId": 1, "pick": myPick(0)}}}, 400)
	// generous: all my picks plus my best player for their worst player
	myBest, _, _ := blockIDs(c, 3)
	gen := []M{{"from": 4, "to": 3, "playerId": theirWorst}, {"from": 3, "to": 4, "playerId": myBest}}
	for i := 0; i < 4; i++ {
		gen = append(gen, M{"from": 3, "to": 4, "pick": myPick(i)})
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
	if n := len(c.call("GET", "/api/trade/block/4", nil, 200)["picks"].([]any)); n != 12 {
		t.Fatalf("team 4 picks %d", n)
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
	_, _, theirBest := blockIDs(c, 4)
	moves := []M{{"from": 3, "to": 4, "playerId": myWorst}, {"from": 4, "to": 3, "playerId": theirBest}}
	v := c.call("POST", "/api/deal/evaluate", M{"moves": moves}, 200)
	if v["accept"] != false {
		t.Fatalf("expected refusal: %v", v)
	}
	tv := v["teams"].([]any)[0].(M)
	if tv["mood"] == "" || tv["mood"] == nil || tv["askText"] == "" || tv["askText"] == nil {
		t.Fatalf("verdict: %v", tv)
	}
	if ask, ok := tv["ask"].(M); ok {
		if tv["askLabel"] == "" || tv["askLabel"] == nil {
			t.Fatalf("no askLabel: %v", tv)
		}
		v2 := c.call("POST", "/api/deal/evaluate", M{"moves": append(moves, ask)}, 200)
		if v2["valid"] != true || v2["teams"].([]any)[0].(M)["accept"] != true {
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
