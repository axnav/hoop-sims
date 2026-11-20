package engine

import "testing"

func TestEditPlayer(t *testing.T) {
	lg := GenerateLeague(3)
	p := lg.Teams[0].Roster[0]
	age, bad := 30, 12
	if err := lg.EditPlayer(p.ID, PlayerEdit{Age: &age}); err == nil {
		t.Fatal("expected refusal with commissioner off")
	}
	lg.Options.Commissioner = true
	impact, oldAge, oldIn := p.Impact, p.Age, p.Ratings.Inside
	if err := lg.EditPlayer(p.ID, PlayerEdit{Age: &age, Ratings: map[string]int{"three": 99, "inside": bad}}); err == nil {
		t.Fatal("expected range error")
	}
	if p.Age != oldAge || p.Ratings.Inside != oldIn {
		t.Fatal("partial change after rejected edit")
	}
	if lg.EditPlayer(-5, PlayerEdit{}) == nil || lg.EditPlayer(p.ID, PlayerEdit{Age: &bad}) == nil ||
		lg.EditPlayer(p.ID, PlayerEdit{Ratings: map[string]int{"nope": 50}}) == nil {
		t.Fatal("expected errors")
	}
	p.Injury = &Injury{Name: "x", GamesLeft: 5}
	heal, pot := true, 40
	err := lg.EditPlayer(p.ID, PlayerEdit{Age: &age, Potential: &pot, Heal: &heal,
		Ratings: map[string]int{"three": 95, "inside": 90, "mid": 90}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Age != 30 || p.Ratings.Three != 95 || p.Injury != nil || p.Impact != impact {
		t.Fatal("edit not applied right")
	}
	if p.Potential < p.Overall() {
		t.Fatal("potential below overall")
	}
	// free agent and draft pool are found too
	fa := &Player{ID: 9999, Age: 25, Ratings: Ratings{50, 50, 50, 50, 50, 50, 50}, Potential: 60}
	lg.FreeAgents = append(lg.FreeAgents, fa)
	if err := lg.EditPlayer(9999, PlayerEdit{Age: &age}); err != nil || fa.Age != 30 {
		t.Fatal("free agent edit failed")
	}
}

func TestEditPlayerOverall(t *testing.T) {
	lg := GenerateLeague(3)
	lg.Options.Commissioner = true
	p := lg.Teams[0].Roster[5]
	impact := p.Impact
	start := p.Overall()
	for _, target := range []int{start + 8, start - 12, start} {
		if err := lg.EditPlayer(p.ID, PlayerEdit{Overall: &target}); err != nil {
			t.Fatal(err)
		}
		if p.Overall() != target {
			t.Fatalf("overall %d, want %d", p.Overall(), target)
		}
		if p.Potential < p.Overall() {
			t.Fatal("potential below overall")
		}
	}
	// combined with a rating edit: ratings first, then the shift
	target := 70
	if err := lg.EditPlayer(p.ID, PlayerEdit{Overall: &target, Ratings: map[string]int{"three": 90}}); err != nil {
		t.Fatal(err)
	}
	if p.Overall() != 70 {
		t.Fatalf("combined: overall %d", p.Overall())
	}
	// near the cap: closest reachable
	star := &Player{ID: 9998, Age: 25, Ratings: Ratings{99, 99, 99, 99, 99, 99, 60}, Potential: 99}
	lg.FreeAgents = append(lg.FreeAgents, star)
	top := 99
	if err := lg.EditPlayer(star.ID, PlayerEdit{Overall: &top}); err != nil {
		t.Fatal(err)
	}
	if star.Overall() < 97 {
		t.Fatalf("cap player overall %d", star.Overall())
	}
	// out of range: error, nothing changes
	before, ovr := p.Ratings, p.Overall()
	for _, bad := range []int{MinRating - 1, MaxRating + 1} {
		if lg.EditPlayer(p.ID, PlayerEdit{Overall: &bad, Ratings: map[string]int{"mid": 40}}) == nil {
			t.Fatal("expected range error")
		}
	}
	if p.Ratings != before || p.Overall() != ovr || p.Impact != impact {
		t.Fatal("changed after rejected edit")
	}
	lg.Options.Commissioner = false
	if lg.EditPlayer(p.ID, PlayerEdit{Overall: &target}) == nil {
		t.Fatal("expected refusal with commissioner off")
	}
}
