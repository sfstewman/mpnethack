package mpnethack

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
)

type Dice struct {
	rng  *rand.Rand
	seed int64
}

func NewDiceFromSeed(seed int64) Dice {
	source := rand.NewSource(seed)
	return Dice{
		rng:  rand.New(source),
		seed: seed,
	}
}

func NewDice() (Dice, error) {
	var randBytes [8]byte

	_, err := crand.Read(randBytes[:])
	if err != nil {
		return Dice{}, err
	}

	seed := int64(binary.LittleEndian.Uint64(randBytes[:]))
	return NewDiceFromSeed(seed), nil
}

func (d Dice) RNG() *rand.Rand {
	return d.rng
}

func (d Dice) Seed() int64 {
	return d.seed
}

func (d Dice) RollD20() int {
	return d.rng.Intn(20) + 1
}

// Rolls M d N
func (d Dice) Roll(m, n int) int {
	if n <= 0 || m <= 0 {
		return 0
	}

	// Md1 is always M
	if n == 1 {
		return m
	}

	acc := 0
	for i := 0; i < m; i++ {
		num := d.rng.Intn(n) + 1
		acc += num
	}

	return acc
}

type Roll struct {
	M int
	N int
}

func (r Roll) Roll(d Dice) int {
	return d.Roll(r.M, r.N)
}
