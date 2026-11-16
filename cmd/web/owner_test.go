package main

import (
	"testing"

	"hoopsim/engine"
)

func TestOwnerAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)

	o := c.call("GET", "/api/owner", nil, 200)
	for _, k := range []string{"active", "owner", "trust", "goals", "money", "ledger", "messages", "fired", "firedText", "jobs", "firing"} {
		if _, ok := o[k]; !ok {
			t.Fatalf("missing %q", k)
		}
	}
	if o["active"] != true || o["fired"] != false || o["firing"] != true || o["firedText"] != "" {
		t.Fatalf("flags: %v", o)
	}
	ow := o["owner"].(M)
	pers := ow["personality"].(M)
	if ow["name"] == "" || pers["key"] == "" || pers["label"] == "" || pers["blurb"] == "" {
		t.Fatalf("owner: %v", ow)
	}
	tr := o["trust"].(M)
	if num(tr["score"]) != engine.OwnerTuning.StartTrust || tr["label"] != "Steady" || tr["trend"] != "flat" {
		t.Fatalf("trust: %v", tr)
	}
	goals := o["goals"].([]any)
	if len(goals) != 3 {
		t.Fatalf("goals: %v", goals)
	}
	kinds := []string{"results", "money", "building"}
	for i, g := range goals {
		g := g.(M)
		if g["kind"] != kinds[i] || g["title"] == "" || g["detail"] == "" || g["progress"] == "" || g["key"] == "" {
			t.Fatalf("goal %d: %v", i, g)
		}
		if w := g["weight"]; w != "Low" && w != "Medium" && w != "High" {
			t.Fatalf("weight %v", w)
		}
		if st := g["status"]; st != "on_track" && st != "at_risk" { // met and missed only after the season-end judging
			t.Fatalf("status %v", st)
		}
	}
	m := o["money"].(M)
	rev, costs := m["revenue"].(M), m["costs"].(M)
	if num(rev["total"]) != num(rev["market"])+num(rev["wins"])+num(rev["playoffs"]) ||
		num(costs["total"]) != num(costs["salaries"])+num(costs["deadMoney"])+num(costs["tax"]) ||
		num(m["profit"]) != num(rev["total"])-num(costs["total"]) || m["projected"] != true {
		t.Fatalf("money: %v", m)
	}
	if num(m["budget"]) <= 0 || num(m["overBudget"]) != max(0, num(m["payroll"])-num(m["budget"])) {
		t.Fatalf("budget: %v", m)
	}
	if ms := m["marketSize"].(M); ms["key"] == "" || ms["label"] == "" {
		t.Fatalf("market: %v", ms)
	}
	if len(o["ledger"].([]any)) != 0 || len(o["jobs"].([]any)) != 0 {
		t.Fatalf("ledger or jobs not empty: %v %v", o["ledger"], o["jobs"])
	}
	msgs := o["messages"].([]any)
	if len(msgs) < 2 || msgs[0].(M)["text"] == "" || num(msgs[0].(M)["day"]) != 1 {
		t.Fatalf("messages: %v", msgs)
	}

	// state key
	st := c.call("GET", "/api/state", nil, 200)
	so := st["owner"].(M)
	if num(so["trust"]) != engine.OwnerTuning.StartTrust || so["label"] != "Steady" || so["fired"] != false {
		t.Fatalf("state owner: %v", so)
	}
	if _, ok := so["alert"]; !ok {
		t.Fatalf("state owner has no alert key: %v", so)
	}

	// options round trip
	if op := c.call("GET", "/api/options", nil, 200); op["noFiring"] != false {
		t.Fatalf("options: %v", op)
	}
	if op := c.call("POST", "/api/options", M{"noFiring": true}, 200); op["noFiring"] != true {
		t.Fatalf("options: %v", op)
	}
	if o2 := c.call("GET", "/api/owner", nil, 200); o2["firing"] != false {
		t.Fatalf("firing should be false: %v", o2["firing"])
	}
	c.call("POST", "/api/options", M{"noFiring": false}, 200)

	// the budget is on the user's team page only
	if pay := c.call("GET", "/api/team/3", nil, 200)["payroll"].(M); num(pay["budget"]) != num(m["budget"]) {
		t.Fatalf("team page payroll: %v", pay)
	}
	if pay := c.call("GET", "/api/team/4", nil, 200)["payroll"].(M); pay["budget"] != nil {
		t.Fatalf("another team's payroll has a budget: %v", pay)
	}

	// not fired yet
	c.call("POST", "/api/owner/job", M{"teamId": 5}, 409)

	// in-season: a warning shows up as the state alert
	lg := c.s.lg
	lg.Owner.Budget = 1000
	c.call("POST", "/api/sim", M{"days": 7}, 200)
	st = c.call("GET", "/api/state", nil, 200)
	if a, _ := st["owner"].(M)["alert"].(string); a == "" {
		t.Fatalf("no alert after a week over budget: %v", st["owner"])
	}

	// get fired: a hopeless team, trust nearly gone
	for _, p := range lg.Teams[3].Roster {
		p.Ratings = engine.Ratings{Inside: 30, Mid: 30, Three: 30, Playmaking: 30, Rebounding: 30, Defense: 30, Athleticism: 30}
	}
	lg.Owner.Trust = 1
	c.call("POST", "/api/sim", M{"toEnd": true}, 200)
	c.call("POST", "/api/playoffs/start", nil, 200)
	c.call("POST", "/api/playoffs/all", nil, 200)
	c.call("POST", "/api/offseason/begin", nil, 200)
	o = c.call("GET", "/api/owner", nil, 200)
	if o["fired"] != true || o["firedText"] == "" || len(o["jobs"].([]any)) != 3 || len(o["ledger"].([]any)) != 1 {
		t.Fatalf("not fired as expected: %v", o)
	}
	if st := c.call("GET", "/api/state", nil, 200)["owner"].(M); st["fired"] != true || st["alert"] == nil {
		t.Fatalf("state owner after firing: %v", st)
	}
	job := o["jobs"].([]any)[0].(M)
	for _, k := range []string{"team", "owner", "marketSize", "record", "payroll", "direction"} {
		if _, ok := job[k]; !ok {
			t.Fatalf("job missing %q: %v", k, job)
		}
	}
	if job["direction"].(M)["label"] == "" || job["owner"].(M)["personality"].(M)["key"] == "" {
		t.Fatalf("job: %v", job)
	}
	// the offseason will not move on
	for _, p := range []string{"/api/offseason/to-draft", "/api/offseason/finish"} {
		r := c.call("POST", p, nil, 409)
		if r["error"] != engine.ErrOwnerFired.Error() {
			t.Fatalf("%s: %v", p, r)
		}
	}
	c.call("POST", "/api/draft/auto", nil, 409)
	c.call("POST", "/api/owner/job", M{"teamId": 3}, 400) // his old team
	c.call("POST", "/api/owner/job", M{"teamId": 99}, 400)
	newTeam := num(job["team"].(M)["id"])
	r := c.call("POST", "/api/owner/job", M{"teamId": newTeam}, 200)
	if r["message"] == "" || r["owner"].(M)["fired"] != false || num(r["owner"].(M)["trust"].(M)["score"]) != engine.OwnerTuning.StartTrust ||
		num(r["state"].(M)["user"].(M)["team"].(M)["id"]) != newTeam {
		t.Fatalf("job result: %v", r)
	}
	c.call("POST", "/api/owner/job", M{"teamId": newTeam}, 409)
	c.call("POST", "/api/offseason/to-draft", nil, 200)
}
