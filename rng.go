package mpnethack

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
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

func (d Dice) Roll1dN(n int) int {
	return d.rng.Intn(n) + 1
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

	acc := d.Roll1dN(n)
	for i := 1; i < m; i++ {
		acc += d.Roll1dN(n)
	}

	return acc
}

type Roll struct {
	M int
	N int
}

var ErrInvalidRoll = errors.New("Invalid roll format, expected MdN")

func (r *Roll) UnmarshalText(text []byte) error {
	s := string(text)
	fields := strings.Split(s, "d")
	if len(fields) < 2 {
		return ErrInvalidRoll
	}

	var err error
	r.M, err = strconv.Atoi(fields[0])
	if err != nil {
		return ErrInvalidRoll
	}

	r.N, err = strconv.Atoi(fields[1])
	if err != nil {
		return ErrInvalidRoll
	}

	return nil
}

func (r *Roll) MarshalText() (text []byte, err error) {
	return []byte(fmt.Sprintf("%dd%d", r.M, r.N)), nil
}

func (r Roll) Roll(d Dice) int {
	return d.Roll(r.M, r.N)
}
