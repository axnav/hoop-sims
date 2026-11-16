package main

import (
	"strings"
	"testing"

	"hoopsim/engine"
)

func TestGMAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)

	g := c.call("GET", "/api/gm", nil, 200)
	for _, k := range []string{"plan", "plans", "auto", "areas", "suggestions", "counts", "log"} {
		if _, ok := g[k]; !ok {
			t.Fatalf("missing %q", k)
		}
	}
	if g["plan"] != "balanced" || len(g["plans"].([]any)) != 3 || len(g["areas"].([]any)) != 6 || g["log"] == nil || g["suggestions"] == nil {
		t.Fatalf("shape: %v", g)
	}
	auto := g["auto"].(M)
	for _, k := range []string{"lineup", "training", "contracts", "freeAgency", "draft", "tradeAlerts"} {
		if auto[k] != false {
			t.Fatalf("auto %s: %v", k, auto[k])
		}
	}
	trades := g["areas"].([]any)[5].(M)
	if trades["key"] != "trades" || trades["canAuto"] != false || trades["autoKey"] != "tradeAlerts" {
		t.Fatalf("trades area: %v", trades)
	}
	if st := c.call("GET", "/api/state", nil, 200); st["gm"] == nil || num(st["gm"].(M)["count"]) != num(g["counts"].(M)["total"]) {
		t.Fatalf("state gm: %v vs %v", st["gm"], g["counts"])
	}

	// settings round trip
	r := c.call("POST", "/api/gm/settings", M{"plan": "rebuild", "auto": M{"lineup": true, "draft": true}}, 200)
	if r["plan"] != "rebuild" || r["auto"].(M)["lineup"] != true || r["auto"].(M)["draft"] != true || r["auto"].(M)["training"] != false {
		t.Fatalf("settings: %v", r)
	}
	r = c.call("POST", "/api/gm/settings", M{"auto": M{"lineup": false}}, 200)
	if r["plan"] != "rebuild" || r["auto"].(M)["lineup"] != false || r["auto"].(M)["draft"] != true {
		t.Fatalf("partial settings: %v", r)
	}
	c.call("POST", "/api/gm/settings", M{"plan": "tank"}, 400)
	c.call("POST", "/api/gm/settings", M{"auto": M{"trades": true}}, 400)
	c.call("POST", "/api/gm/settings", M{"plan": "balanced", "auto": M{"draft": false}}, 200)

	// apply: training suggestions exist once a focus is off
	for _, p := range c.s.lg.Teams[3].Roster {
		p.Focus = engine.FocusBody
	}
	g = c.call("GET", "/api/gm", nil, 200)
	var pick M
	for _, x := range g["suggestions"].([]any) {
		if x.(M)["area"] == "training" {
			pick = x.(M)
			break
		}
	}
	if pick == nil {
		t.Fatalf("no training suggestion: %v", g["counts"])
	}
	for _, k := range []string{"id", "area", "title", "reasons", "action", "playerId", "link"} {
		if _, ok := pick[k]; !ok {
			t.Fatalf("suggestion missing %q", k)
		}
	}
	ap := c.call("POST", "/api/gm/apply", M{"id": pick["id"]}, 200)
	if ap["message"] == "" || ap["gm"] == nil || ap["state"] == nil {
		t.Fatalf("apply: %v", ap)
	}
	log := ap["gm"].(M)["log"].([]any)
	if len(log) != 1 || log[0].(M)["auto"] != false || num(log[0].(M)["day"]) != 1 {
		t.Fatalf("log: %v", log)
	}
	c.call("POST", "/api/gm/apply", M{"id": pick["id"]}, 409)
	c.call("POST", "/api/gm/apply", M{"id": "bogus"}, 400)
	c.call("POST", "/api/gm/apply", M{}, 400)
	c.call("POST", "/api/gm/apply", M{"id": "fa:3"}, 409)

	// a trade alert pauses a multi-day sim with the reason "assistant"
	c.call("POST", "/api/gm/settings", M{"auto": M{"tradeAlerts": true}}, 200)
	sim := c.call("POST", "/api/sim", M{"days": 30}, 200)
	if p, ok := sim["pause"].(M); ok {
		if p["reason"] != "assistant" || p["text"] == "" {
			t.Fatalf("pause: %v", p)
		}
		if g := c.call("GET", "/api/gm", nil, 200); len(g["suggestions"].([]any)) == 0 {
			t.Fatal("alert without a suggestion")
		}
	} else {
		t.Log("no alert in 30 days for this fixture")
	}
}

// Draft suggestions carry scouting ranges only: the title and reasons never
// hold a true value, and the choice matches the range midpoints.
func TestGMDraftSecrecy(t *testing.T) {
	c := newTC(t)
	for _, seed := range []int{11, 12} {
		toDraftStart(c, seed)
		lg := c.s.lg
		for lg.DraftOnClock() != lg.UserTeamID {
			lg.DraftPick(-1)
		}
		g := c.call("GET", "/api/gm", nil, 200)
		var d M
		for _, x := range g["suggestions"].([]any) {
			if x.(M)["area"] == "draft" {
				d = x.(M)
			}
		}
		if d == nil {
			t.Fatal("no draft suggestion")
		}
		id := num(d["playerId"])
		var p *engine.Player
		for _, q := range lg.Draft.Pool {
			if q.ID == id {
				p = q
			}
		}
		v, _ := lg.ScoutView(id)
		text := d["title"].(string) + " " + strings.Join(strFlat(d["reasons"].([]any)), " ")
		want := "at " + itoa(v.OvrLo) + " to " + itoa(v.OvrHi) + " overall and " + itoa(v.PotLo) + " to " + itoa(v.PotHi) + " potential"
		if !strings.Contains(text, want) {
			t.Fatalf("draft text %q should hold the range %q", text, want)
		}
		_ = p
		c.call("POST", "/api/gm/settings", M{"auto": M{"draft": true}}, 200)
		st := c.call("POST", "/api/draft/step", M{"max": 100}, 200)
		if st["state"].(M)["draft"].(M)["userOnClock"] == true {
			t.Fatal("auto draft stopped on the user")
		}
	}
}

func strFlat(xs []any) []string {
	var out []string
	for _, x := range xs {
		out = append(out, x.(string))
	}
	return out
}
