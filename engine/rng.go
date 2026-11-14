package engine

import "math/rand/v2"

// RNG is the only source of randomness in the engine. Everything takes one
// explicitly, so the same seed always produces the same league and results.
type RNG struct{ *rand.Rand }

func NewRNG(seed uint64) *RNG {
	return &RNG{rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

// Between returns an int in [lo, hi].
func (r *RNG) Between(lo, hi int) int { return lo + r.IntN(hi-lo+1) }

// Normal returns a normally distributed value.
func (r *RNG) Normal(mean, sd float64) float64 { return mean + r.NormFloat64()*sd }

// Chance returns true with probability p.
func (r *RNG) Chance(p float64) bool { return r.Float64() < p }

// Pick returns a random element of xs.
func Pick[T any](r *RNG, xs []T) T { return xs[r.IntN(len(xs))] }

// PickWeighted returns an index chosen in proportion to its weight.
func (r *RNG) PickWeighted(weights []float64) int {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	x := r.Float64() * total
	for i, w := range weights {
		x -= w
		if x < 0 {
			return i
		}
	}
	return len(weights) - 1
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
