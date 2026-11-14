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
