package engine

// StatusBands are the cut-offs on the contend factor (0 to 1, see
// contendFactors) that give a team its direction. Tunable.
var StatusBands = struct {
	Contender float64 // at or above: chasing a title
	Buyer     float64 // at or above: in the race. Also the line the proposal code uses to split buyers from sellers
	Seller    float64 // at or above: falling out of the race; below: rebuilding
}{Contender: 0.8, Buyer: 0.5, Seller: 0.2}

// TeamStatus is a team's direction, from visible strength and record only.
type TeamStatus struct{ Key, Label, Wants string }

func statusFor(cf float64) TeamStatus {
	switch {
	case cf >= StatusBands.Contender:
		return TeamStatus{"contender", "Contender", "Chasing a title. Wants proven players now and will pay with picks and young players."}
	case cf >= StatusBands.Buyer:
		return TeamStatus{"buyer", "Buyer", "In the playoff race. Looking to add help."}
	case cf >= StatusBands.Seller:
		return TeamStatus{"seller", "Seller", "Falling out of the race. Open to moving veterans for picks and young players."}
	}
	return TeamStatus{"rebuilding", "Rebuilding", "Building for the future. Wants picks and young players with potential."}
}

// TeamStatuses returns every team's status, indexed by team ID.
func (lg *League) TeamStatuses() []TeamStatus {
	cf := lg.contendFactors()
	out := make([]TeamStatus, len(cf))
	for i, c := range cf {
		out[i] = statusFor(c)
	}
	return out
}

// TeamStatusOf is one team's status.
func (lg *League) TeamStatusOf(teamID int) TeamStatus {
	return lg.TeamStatuses()[teamID]
}

// TeamTop8 is the average overall of a team's best eight players.
func (lg *League) TeamTop8(teamID int) float64 { return teamTop8(lg.Teams[teamID]) }
