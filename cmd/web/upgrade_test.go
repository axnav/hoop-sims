package main

import "testing"

func TestTradeUpgradeAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)

	mine := c.call("GET", "/api/trade/block/3", nil, 200)["players"].([]any)
	if len(mine) < 3 {
		t.Fatalf("roster too small")
	}
	var possible, impossible int
	for _, e := range mine[:5] {
		id := num(e.(M)["id"])
		r := c.call("GET", "/api/trade/upgrade/"+itoa(id), nil, 200)
		for _, k := range []string{"player", "possible", "reason", "upgrades"} {
			if _, ok := r[k]; !ok {
				t.Fatalf("missing %q: %v", k, r)
			}
		}
		if num(r["player"].(M)["id"]) != id || r["possible"] != true {
			t.Fatalf("bad answer: %v", r)
		}
		ups, ok := r["upgrades"].([]any)
		if !ok {
			t.Fatalf("upgrades is not an array: %v", r["upgrades"])
		}
		for _, x := range ups {
			u := x.(M)
			for _, k := range []string{"player", "team", "direction", "gain", "shopped", "shopReason", "possible", "reason"} {
				if _, ok := u[k]; !ok {
					t.Fatalf("upgrade missing %q: %v", k, u)
				}
			}
			if d := u["direction"].(M); d["key"] == "" || d["label"] == "" {
				t.Fatalf("direction: %v", d)
			}
			if u["team"].(M)["abbr"] == nil || num(u["gain"]) < 2 {
				t.Fatalf("bad upgrade: %v", u)
			}
			pk, has := u["package"]
			if u["possible"] != true {
				impossible++
				if has || u["reason"] == "" {
					t.Fatalf("not possible: package must be absent and a reason given: %v", u)
				}
				continue
			}
			possible++
			p := pk.(M)
			for _, k := range []string{"label", "give", "receive", "note", "cost", "cheapest", "deal"} {
				if _, ok := p[k]; !ok {
					t.Fatalf("package missing %q: %v", k, p)
				}
			}
			in := false
			for _, g := range p["give"].([]any) {
				if row := g.(M); row["kind"] == "player" && num(row["id"]) == id {
					in = true
				}
			}
			if !in {
				t.Fatalf("package does not include the selected player: %v", p)
			}
			// the package loads as a deal the other team accepts
			v := c.call("POST", "/api/deal/evaluate", M{"moves": p["deal"].(M)["moves"]}, 200)
			if v["valid"] != true || v["accept"] != true {
				t.Fatalf("package not accepted: %v", v)
			}
		}
	}
	if possible == 0 {
		t.Fatalf("no possible upgrade in five players (impossible %d)", impossible)
	}

	// another team's player, an unknown id and a bad id
	other := c.call("GET", "/api/trade/block/5", nil, 200)["players"].([]any)[0].(M)
	c.call("GET", "/api/trade/upgrade/"+itoa(num(other["id"])), nil, 400)
	c.call("GET", "/api/trade/upgrade/-7", nil, 400)
	c.call("GET", "/api/trade/upgrade/x", nil, 400)

	// trades closed: not possible, an empty array
	c.s.lg.Season.Day = 200
	r := c.call("GET", "/api/trade/upgrade/"+itoa(num(mine[0].(M)["id"])), nil, 200)
	if r["possible"] != false || r["reason"] == "" {
		t.Fatalf("closed: %v", r)
	}
	if ups, ok := r["upgrades"].([]any); !ok || len(ups) != 0 {
		t.Fatalf("closed: upgrades must be []: %v", r["upgrades"])
	}
}
