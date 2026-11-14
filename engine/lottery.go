package engine

// Draft lottery. The 14 teams that missed the playoffs enter a lottery for the
// top 4 first-round slots. RNG stream: lg.dayRNG(year, lotteryOffset).
//
// RNG stream offsets in use (see also negotiate.go, injury.go, contract.go,
// freeagency.go): 20000 proposals, 30000 injuries, 40000 contracts, 50000 free
// agency, 60000 draft lottery, 70000 scouting errors and computer draft noise.
const lotteryOffset = 60000

// LotteryOdds is each lottery team's chance (tenths of a percent) at winning
// the lottery, by pre-lottery position (worst record first). Sums to 1000.
var LotteryOdds = []int{140, 140, 140, 125, 105, 90, 75, 60, 45, 30, 20, 15, 10, 5}

// LotteryDrawn is how many top slots the lottery decides.
const LotteryDrawn = 4

// LotteryRow is one lottery team's result.
type LotteryRow struct {
	Team   int // original team ID
	Seed   int // pre-lottery position, 1 = worst record
	Odds   int // tenths of a percent
	Slot   int // final first-round slot, 1-based
	Owner  int // current owner of that team's first-round pick
	Change int // Seed - Slot: positive means moved up
}

// drawLottery draws the winners of the top slots one at a time without
// replacement, weighted by odds. It returns indexes into odds (0 = worst
// record), best slot first.
func drawLottery(r *RNG, odds []int) []int {
	left := make([]int, len(odds))
	for i := range left {
		left[i] = i
	}
	var out []int
	for len(out) < LotteryDrawn && len(left) > 0 {
		w := make([]float64, len(left))
		for i, idx := range left {
			w[i] = float64(odds[idx])
		}
		k := r.PickWeighted(w)
		out = append(out, left[k])
		left = append(left[:k], left[k+1:]...)
	}
	return out
}

// lotteryOrder builds the first-round order (original team IDs, slot 1
// first) from the reversed standings `rev` (worst record first) and a
// playoff test. It also returns the lottery rows (nil if nobody is in it).
func (lg *League) lotteryOrder(r *RNG, rev []int, madePlayoffs func(int) bool) ([]int, []LotteryRow) {
	var lot, rest []int
	for _, t := range rev {
		if madePlayoffs(t) {
			rest = append(rest, t)
		} else {
			lot = append(lot, t)
		}
	}
	if len(lot) > len(LotteryOdds) { // cannot happen with 16 playoff teams
		rest = append(append([]int(nil), lot[len(LotteryOdds):]...), rest...)
		lot = lot[:len(LotteryOdds)]
	}
	odds := LotteryOdds[:len(lot)]
	won := drawLottery(r, odds)
	isWinner := map[int]bool{}
	var order []int
	slotOf := map[int]int{} // team -> slot
	for _, idx := range won {
		isWinner[idx] = true
		order = append(order, lot[idx])
	}
	for i, t := range lot {
		if !isWinner[i] {
			order = append(order, t)
		}
	}
	order = append(order, rest...)
	for i, t := range order {
		slotOf[t] = i + 1
	}
	var rows []LotteryRow
	for i, t := range lot {
		owner := t
		if o, ok := lg.PickOwner(lg.Year, 1, t); ok {
			owner = o
		}
		rows = append(rows, LotteryRow{Team: t, Seed: i + 1, Odds: odds[i], Slot: slotOf[t], Owner: owner, Change: i + 1 - slotOf[t]})
	}
	return order, rows
}
