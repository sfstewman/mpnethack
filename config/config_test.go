package config

import (
	"errors"
	"fmt"
	"math"
	"testing"
)

type testCase struct {
	expected interface{}
	value    interface{}
	err      error
}

type equalFunc func(interface{}, interface{}) bool
type converterFunc func(interface{}) (interface{}, error)

func runTestCases(t *testing.T, cases []testCase, converter converterFunc, isEqual equalFunc) {
	for _, tc := range cases {
		if tc.err == nil {
			t.Run(fmt.Sprintf("input=%v_expected=%v", tc.value, tc.expected), func(t *testing.T) {
				v, err := converter(tc.value)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				} else if !isEqual(v, tc.expected) {
					t.Errorf("expected value %v but found %v", tc.expected, v)
				}
			})
		} else {
			t.Run(fmt.Sprintf("input=%v_expected=error_%v", tc.value, tc.err), func(t *testing.T) {
				_, err := converter(tc.value)
				if err == nil {
					t.Errorf("no error, but expected error %v", tc.err)
				} else if err != tc.err {
					t.Errorf("expected error %v but found %v", tc.err, err)
				}
			})
		}
	}
}

func TestToInt(t *testing.T) {
	cases := []testCase{
		{int(5), int8(5), nil},
		{int(513), int16(513), nil},
		{int(5061732), int32(5061732), nil},
		{int(5061732431437132), int64(5061732431437132), nil},
		{int(313), float32(313.0), nil},
		{int(313), float64(313.0), nil},
		{int(9), uint64(9), nil},
		{int(151), uint32(151), nil},
		{int(4151), uint16(4151), nil},
		{int(32), uint8(32), nil},

		{0, uint64(math.MaxInt) + 1, ErrOutOfRange},
		{0, float32(313.3), ErrBadType},
		{0, float64(314.5), ErrBadType},
	}

	cvt := func(v interface{}) (interface{}, error) {
		v, err := toInt(v)
		return v, err
	}
	eq := func(a, b interface{}) bool {
		ai := a.(int)
		bi := b.(int)
		return ai == bi
	}

	runTestCases(t, cases, cvt, eq)
}

func TestToUint(t *testing.T) {
	cases := []testCase{
		{uint(5), int8(5), nil},
		{uint(513), int16(513), nil},
		{uint(5061732), int32(5061732), nil},
		{uint(5061732431437132), int64(5061732431437132), nil},
		{uint(313), float32(313.0), nil},
		{uint(313), float64(313.0), nil},
		{uint(0), int64(-1), ErrOutOfRange},
		{uint(0), float32(313.3), ErrBadType},
		{uint(0), float64(314.5), ErrBadType},
	}

	cvt := func(v interface{}) (interface{}, error) {
		v, err := toUint(v)
		return v, err
	}
	eq := func(a, b interface{}) bool {
		return a.(uint) == b.(uint)
	}

	runTestCases(t, cases, cvt, eq)
}

func TestToFloat(t *testing.T) {
	cases := []testCase{
		{float64(5), int8(5), nil},
		{float64(513), int16(513), nil},
		{float64(5061732), int32(5061732), nil},
		{float64(5061732431437132), int64(5061732431437132), nil},
		{float64(float32(313.3)), float32(313.3), nil},
		{float64(313.9), float64(313.9), nil},
		{float64(-1), int64(-1), nil},

		{float64(313.3), "foo", ErrBadType},
	}

	cvt := func(v interface{}) (interface{}, error) {
		v, err := toFloat(v)
		return v, err
	}
	eq := func(a, b interface{}) bool {
		return a.(float64) == b.(float64)
	}

	runTestCases(t, cases, cvt, eq)
}

func TestUnmarshalHelper(t *testing.T) {
	out := struct {
		s string
		i int
		u uint
		f float64
		x int32
		y uint32
		z float32
	}{}

	data := map[string]interface{}{
		"s": "test",
		"i": int(32),
		"u": int(64),
		"f": int(32),
		"x": int(9812),
		"y": int(432832),
		"z": int(9),
	}

	err := UnmarshalHelper(data, map[string]interface{}{
		"s": &out.s,
		"i": &out.i,
		"u": &out.u,
		"f": &out.f,
		"x": &out.x,
		"y": &out.y,
		"z": &out.z,
	}, UnknownKeyIsError)

	if err != nil {
		t.Errorf("error unmarshaling: %v", err)
	}

	if out.s != "test" {
		t.Errorf("out.s expected to be \"test\" but found \"%s\"", out.s)
	}

	if out.i != 32 {
		t.Errorf("out.i expected to be 32 but found %d", out.i)
	}

	if out.u != 64 {
		t.Errorf("out.i expected to be 32 but found %d", out.u)
	}

	if out.f != 32 {
		t.Errorf("out.f expected to be 32 but found %f", out.f)
	}

	if out.x != 9812 {
		t.Errorf("out.x expected to be 9812 but found %d", out.x)
	}

	if out.y != 432832 {
		t.Errorf("out.y expected to be 432832 but found %d", out.y)
	}

	if out.z != 9 {
		t.Errorf("out.z expected to be 9 but found %f", out.z)
	}
}

type testTextNumber int

func (n *testTextNumber) UnmarshalText(b []byte) error {
	s := string(b)
	switch s {
	case "one":
		*n = 1
	case "two":
		*n = 2
	case "three":
		*n = 3
	default:
		return errors.New("invalid text number")
	}

	fmt.Printf("\ns = \"%s\", n = %d\n\n", s, *n)
	return nil
}

func TestUnmarshalHelper_TextUnmarshaler(t *testing.T) {
	values := []testTextNumber{0, 0, 0}

	data := map[string]interface{}{
		"v1": "one",
		"v2": "two",
		"v3": "three",
	}

	err := UnmarshalHelper(data, map[string]interface{}{
		"v1": &values[0],
		"v2": &values[1],
		"v3": &values[2],
	}, UnknownKeyIsError)

	if err != nil {
		t.Errorf("error unmarshaling: %v", err)
	}

	if values[0] != 1 {
		t.Errorf("values[0] expected to be 1 but found %d", values[0])
	}

	if values[1] != 2 {
		t.Errorf("values[1] expected to be 2 but found %d", values[1])
	}

	if values[2] != 3 {
		t.Errorf("values[2] expected to be 3 but found %d", values[2])
	}
}
