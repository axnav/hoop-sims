package main

import "testing"

func TestTradeAssistantAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)

	n := c.call("GET", "/api/trade/needs/5", nil, 200)
	for _, k := range []string{"team", "direction", "values", "lacks", "summary"} {
		if _, ok := n[k]; !ok {
			t.Fatalf("needs: missing %q: %v", k, n)
		}
	}
	if d := n["direction"].(M); d["key"] == "" || d["label"] == "" {
		t.Fatalf("direction: %v", d)
	}
	if len(n["values"].([]any)) == 0 || n["summary"] == "" {
		t.Fatalf("needs empty: %v", n)
	}
	c.call("GET", "/api/trade/needs/99", nil, 404)
	c.call("GET", "/api/trade/needs/x", nil, 400)

	// a player on team 5 with a package: load-ready deal, accepted by the real evaluation
	block := c.call("GET", "/api/trade/block/5", nil, 200)
	var done bool
	for _, e := range block["players"].([]any) {
		id := num(e.(M)["id"])
		w := c.call("GET", "/api/trade/whatwouldittake/"+itoa(id), nil, 200)
		for _, k := range []string{"player", "team", "possible", "reason", "packages"} {
			if _, ok := w[k]; !ok {
				t.Fatalf("whatwouldittake: missing %q", k)
			}
		}
		pk := w["packages"].([]any)
		if w["possible"] != true {
			if w["reason"] == "" || len(pk) != 0 {
				t.Fatalf("impossible needs a reason and no packages: %v", w)
			}
			continue
		}
		if len(pk) == 0 {
			t.Fatalf("possible without packages")
		}
		first := pk[0].(M)
		for _, k := range []string{"label", "give", "receive", "note", "deal", "cheapest"} {
			if _, ok := first[k]; !ok {
				t.Fatalf("package: missing %q", k)
			}
		}
		for _, g := range first["give"].([]any) {
			row := g.(M)
			if row["kind"] != "player" && row["kind"] != "pick" {
				t.Fatalf("bad give row %v", row)
			}
			if row["kind"] == "player" && (row["id"] == nil || row["name"] == nil) {
				t.Fatalf("player row %v", row)
			}
			if row["kind"] == "pick" && (row["year"] == nil || row["round"] == nil || row["original"] == nil || row["label"] == nil) {
				t.Fatalf("pick row %v", row)
			}
		}
		rec := first["receive"].([]any)
		if len(rec) != 1 || num(rec[0].(M)["id"]) != id {
			t.Fatalf("receive: %v", rec)
		}
		deal := first["deal"].(M)
		v := c.call("POST", "/api/deal/evaluate", deal, 200)
		if v["valid"] != true || v["accept"] != true {
			t.Fatalf("suggested deal not accepted: %v", v)
		}
		if !done {
			c.call("POST", "/api/deal/execute", deal, 200)
			done = true
		}
		break
	}
	if !done {
		t.Fatal("no player with a package")
	}
	// own player and unknown player
	own := c.call("GET", "/api/trade/block/3", nil, 200)["players"].([]any)[0].(M)
	c.call("GET", "/api/trade/whatwouldittake/"+itoa(num(own["id"])), nil, 400)
	c.call("GET", "/api/trade/whatwouldittake/-7", nil, 404)

	a := c.call("GET", "/api/trade/advisor", nil, 200)
	rows := a["players"].([]any)
	if len(rows) == 0 || a["summary"] == "" {
		t.Fatalf("advisor: %v", a)
	}
	for _, e := range rows {
		r := e.(M)
		if adv := r["advice"]; adv != "keep" && adv != "shop" && adv != "sell" {
			t.Fatalf("advice %v", adv)
		}
		if len(r["reasons"].([]any)) == 0 {
			t.Fatalf("no reasons: %v", r)
		}
		for _, k := range []string{"id", "name", "pos", "age", "ovr"} {
			if r[k] == nil {
				t.Fatalf("advisor row missing %q", k)
			}
		}
	}
}
