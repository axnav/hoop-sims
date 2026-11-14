package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
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
	c.call("POST", "/api/sim", M{"days": 30}, 200)
	for _, p := range []string{"/api/state", "/api/standings", "/api/leaders", "/api/team/0", "/api/player/1", "/api/schedule", "/api/box/last", "/api/history", "/api/playoffs"} {
		c.call("GET", p, nil, 200) // checkNoImpact runs on every response
	}
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
