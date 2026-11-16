package main

import "testing"

func TestTradeFinderAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"seed": 5, "team": 3}, 200)

	block := c.call("GET", "/api/trade/block/3", nil, 200)
	mine := block["players"].([]any)
	if len(mine) < 3 {
		t.Fatal("small roster")
	}
	top := num(mine[0].(M)["id"])
	second := num(mine[1].(M)["id"])
	picks := block["picks"].([]any)
	pk := picks[0].(M)
	pickBody := M{"year": num(pk["year"]), "round": num(pk["round"]), "original": num(pk["original"].(M)["id"])}

	r := c.call("POST", "/api/trade/find", M{"players": []int{top}, "picks": []M{pickBody}, "want": "any"}, 200)
	for _, k := range []string{"possible", "reason", "want", "wants", "give", "giveValue", "offers", "passed"} {
		if _, ok := r[k]; !ok {
			t.Fatalf("missing %q", k)
		}
	}
	if len(r["wants"].([]any)) != 5 || len(r["give"].([]any)) != 2 {
		t.Fatalf("wants/give: %v", r)
	}
	offers := r["offers"].([]any)
	if r["possible"] != true || len(offers) == 0 {
		t.Fatalf("no offers: %v", r)
	}
	if len(offers)+len(r["passed"].([]any)) != 29 {
		t.Fatalf("offers + passed != 29")
	}
	first := offers[0].(M)
	for _, k := range []string{"team", "direction", "receive", "value", "salaryIn", "salaryOut", "note", "deal"} {
		if _, ok := first[k]; !ok {
			t.Fatalf("offer: missing %q", k)
		}
	}
	// the same call with only a default want, and another filter
	c.call("POST", "/api/trade/find", M{"players": []int{top, second}}, 200)
	pr := c.call("POST", "/api/trade/find", M{"players": []int{second}, "want": "picks"}, 200)
	for _, o := range pr["offers"].([]any) {
		for _, a := range o.(M)["receive"].([]any) {
			if a.(M)["kind"] != "pick" {
				t.Fatalf("picks-only offer has %v", a)
			}
		}
	}

	deal := first["deal"].(M)
	v := c.call("POST", "/api/deal/evaluate", deal, 200)
	if v["valid"] != true || v["accept"] != true {
		t.Fatalf("offer not accepted: %v", v)
	}
	c.call("POST", "/api/deal/execute", deal, 200)

	c.call("POST", "/api/trade/find", M{"players": []int{second}, "want": "bogus"}, 400)
	foreign := c.call("GET", "/api/trade/block/5", nil, 200)["players"].([]any)[0].(M)
	c.call("POST", "/api/trade/find", M{"players": []int{num(foreign["id"])}}, 400)
	c.call("POST", "/api/trade/find", M{"players": []int{999999}}, 400)
	c.call("POST", "/api/trade/find", M{"picks": []M{{"year": 1900, "round": 1, "original": 1}}}, 400)
	c.call("POST", "/api/trade/find", M{}, 400)
	c.call("POST", "/api/trade/find", M{"players": []int{1, 2, 3, 4}}, 400)
}
