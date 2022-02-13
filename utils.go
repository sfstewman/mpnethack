package mpnethack

import "fmt"

type Direction int16

const (
	NoDirection Direction = iota
	Left
	Right
	Up
	Down
)

func (direc Direction) Name() string {
	switch direc {
	case NoDirection:
		return "none"
	case Left:
		return "left"
	case Right:
		return "right"
	case Up:
		return "up"
	case Down:
		return "down"
	}

	return fmt.Sprintf("Direction[%d]", direc)
}

func (direc Direction) Vectors() (ui, uj, vi, vj int) {
	switch direc {
	case Up:
		ui, uj = -1, 0
		vi, vj = 0, 1
	case Down:
		ui, uj = 1, 0
		vi, vj = 0, -1
	case Left:
		ui, uj = 0, -1
		vi, vj = -1, 0
	case Right:
		ui, uj = 0, 1
		vi, vj = 1, 0
	}

	return
}

func (direc Direction) Mirror() Direction {
	switch direc {
	case Up:
		return Down
	case Down:
		return Up
	case Left:
		return Right
	case Right:
		return Left
	default:
		return NoDirection
	}
}

func MinInt(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

func MaxInt(a, b int) int {
	if a >= b {
		return a
	}
	return b
}

func ClipCoord(x, xMin, xMaxPlusOne int) int {
	if x < xMin {
		return xMin
	}

	if x >= xMaxPlusOne {
		return xMaxPlusOne - 1
	}

	return x
}

func SignAndMagnitude(val int) (sign int, magnitude int) {
	switch {
	case val > 0:
		sign = 1
		magnitude = val
	case val < 0:
		sign = -1
		magnitude = -val
	case val == 0:
		sign = 0
		magnitude = 0
	}

	return
}

type AABB struct {
	I0, J0, I1, J1 int
}

func (bb *AABB) Width() int {
	return bb.J1 - bb.J0
}

func (bb *AABB) Height() int {
	return bb.I1 - bb.I0
}

func (bb *AABB) Inside(i, j int) bool {
	return (i >= bb.I0) && (i < bb.I1) && (j >= bb.J0) && (j < bb.J1)
}

/*
func (bb *AABB) Intersect(other *AABB) (AABB, bool) {
	// check for no overlap
	if bb.I1 < other.I0 || other.I1 < bb.I0 || bb.J1 < other.J0 || other.J1 < bb.J0 {
		return AABB{}, false
	}

	// check for one AABB enclosing the other
	if bb.I0 <= other.I0 && bb.I1 >= other.I1 && bb.J0 <= other.J0 && bb.J1 >= other.J1 {
		return *other, true
	}

	// FIXME: test!
	i0, j0, i1, j1 := bb.I0, bb.J0, bb.I1, bb.J1
	if other.I0 > i0 {
		i0 = other.I0
	}

	if other.I1 < i1 {
		i1 = other.I1
	}

	if other.J0 > j0 {
		j0 = other.J0
	}

	if other.J1 < j1 {
		j1 = other.J1
	}

	return AABB{I0: i0, J0: j0, I1: i1, J1: j1}, (i0 < i1 && j0 < j1)
}
*/
