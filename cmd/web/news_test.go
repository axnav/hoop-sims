package main

import (
	"testing"
)

func TestNewsAndAwardsAPI(t *testing.T) {
	c := newTC(t)
	c.call("POST", "/api/new", M{"mode": "fictional", "seed": 3, "team": 2}, 200)
	c.call("POST", "/api/options", M{"noDeadlineStop": true, "noOfferStop": true}, 200)

	// Before the season is decided: no current awards.
	if h := c.call("GET", "/api/history", nil, 200); h["current"] != nil {
		t.Fatalf("current before the season ends: %v", h["current"])
	}
	c.call("POST", "/api/sim", M{"days": 20}, 200)
	n := c.call("GET", "/api/news", nil, 200)
	items := n["items"].([]any)
	if len(items) == 0 {
		t.Fatal("no news after 20 days")
	}
	first := items[0].(M)
	for _, k := range []string{"id", "year", "day", "stage", "kind", "text", "big", "mine", "teams", "players"} {
		if _, ok := first[k]; !ok {
			t.Fatalf("news item lacks %q: %v", k, first)
		}
	}
	if d := num(first["day"]); d < 0 || d > 82 {
		t.Fatalf("day %d", d)
	}
	for i := 1; i < len(items); i++ {
		if num(items[i].(M)["id"]) >= num(items[i-1].(M)["id"]) {
			t.Fatal("news is not newest first")
		}
	}
	fl := n["filters"].([]any)
	keys := []string{"all", "mine", "trades", "signings", "injuries", "games", "awards", "draft"}
	labels := []string{"All", "My team", "Trades", "Signings", "Injuries", "Games", "Awards", "Draft"}
	if len(fl) != 8 {
		t.Fatalf("filters: %v", fl)
	}
	for i, f := range fl {
		m := f.(M)
		if m["key"] != keys[i] || m["label"] != labels[i] {
			t.Fatalf("filter %d: %v", i, m)
		}
	}
	if num(n["total"]) != num(fl[0].(M)["count"]) {
		t.Fatalf("total %v vs all count %v", n["total"], fl[0].(M)["count"])
	}
	lim := c.call("GET", "/api/news?filter=all&limit=3", nil, 200)
	if len(lim["items"].([]any)) > 3 {
		t.Fatal("limit ignored")
	}
	c.call("GET", "/api/news?filter=mine", nil, 200)
	mine := c.call("GET", "/api/news?filter=injuries&limit=400", nil, 200)
	for _, it := range mine["items"].([]any) {
		if it.(M)["kind"] != "injury" {
			t.Fatalf("injury filter returned %v", it)
		}
	}
	c.call("GET", "/api/news?filter=bogus", nil, 400)
	c.call("GET", "/api/news?limit=abc", nil, 400)
	c.call("GET", "/api/news?limit=0", nil, 400)
	big := c.call("GET", "/api/news?limit=9999", nil, 200)
	if len(big["items"].([]any)) > 400 {
		t.Fatal("limit above 400 not capped")
	}

	// Season over: current awards appear, in the contract shape.
	c.call("POST", "/api/sim", M{"toEnd": true}, 200)
	h := c.call("GET", "/api/history", nil, 200)
	cur, ok := h["current"].(M)
	if !ok {
		t.Fatalf("no current awards at the end of the regular season: %v", h["current"])
	}
	aw := cur["awards"].(M)
	for _, k := range []string{"roy", "dpoy", "sixthMan", "mip", "coach", "allLeague", "allRookie"} {
		if _, ok := aw[k]; !ok {
			t.Fatalf("awards lacks %q", k)
		}
	}
	if cur["mvp"] == nil || cur["scoringLeader"] == nil || num(cur["year"]) != 2025 {
		t.Fatalf("current: %v", cur)
	}
	al := aw["allLeague"].([]any)
	if len(al) != 3 || len(al[0].([]any)) != 5 {
		t.Fatalf("allLeague: %v", al)
	}
	if aw["dpoy"] == nil || aw["dpoy"].(M)["playerId"] == nil || aw["coach"].(M)["team"] == nil {
		t.Fatalf("awards: %v", aw)
	}
	// The MVP's page lists the award.
	mvp := num(cur["mvp"].(M)["playerId"])
	pl := c.call("GET", "/api/player/"+itoa(mvp), nil, 200)
	list, ok := pl["awards"].([]any)
	if !ok || len(list) == 0 || list[0].(M)["title"] == nil || num(list[0].(M)["year"]) != 2025 {
		t.Fatalf("player awards: %v", pl["awards"])
	}
	// Any player has a (possibly empty) awards array, never null.
	team := c.call("GET", "/api/team/5", nil, 200)
	other := num(team["roster"].([]any)[0].(M)["id"])
	if po := c.call("GET", "/api/player/"+itoa(other), nil, 200); po["awards"] == nil {
		t.Fatal("awards must be an array, not null")
	}
	ah := c.call("GET", "/api/news?filter=awards", nil, 200)
	if num(ah["total"]) < 5 {
		t.Fatalf("expected award stories, got %v", ah["total"])
	}

	// Archived: the season row carries the awards and current goes away.
	c.call("POST", "/api/playoffs/start", nil, 200)
	c.call("POST", "/api/playoffs/all", nil, 200)
	c.call("POST", "/api/offseason/begin", nil, 200)
	h = c.call("GET", "/api/history", nil, 200)
	if h["current"] != nil {
		t.Fatalf("current should be null once archived: %v", h["current"])
	}
	row := h["seasons"].([]any)[0].(M)
	ra := row["awards"].(M)
	if ra["dpoy"] == nil || len(ra["allLeague"].([]any)) != 3 || ra["roy"] != nil || ra["allRookie"] == nil {
		t.Fatalf("season row awards: %v", ra)
	}
	if len(ra["allRookie"].([]any)) != 0 {
		t.Fatalf("first season has no rookies: %v", ra["allRookie"])
	}
	// The lottery story exists once the offseason begins.
	d := c.call("GET", "/api/news?filter=draft", nil, 200)
	if num(d["total"]) < 1 {
		t.Fatalf("draft stories: %v", d["total"])
	}
}
