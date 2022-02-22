package config

import (
	"fmt"
	"math"
	"testing"
)

func TestToInt(t *testing.T) {
	type testCase struct {
		expected int
		value    interface{}
		err      error
	}

	cases := []testCase{
		{5, int8(5), nil},
		{513, int16(513), nil},
		{5061732, int32(5061732), nil},
		{5061732431437132, int64(5061732431437132), nil},
		{313, float32(313.0), nil},
		{313, float64(313.0), nil},
		{9, uint64(9), nil},
		{151, uint32(151), nil},
		{4151, uint16(4151), nil},
		{32, uint8(32), nil},

		{0, uint64(math.MaxInt) + 1, ErrOutOfRange},
		{0, float32(313.3), ErrBadType},
		{0, float64(314.5), ErrBadType},
	}

	for _, tc := range cases {
		v, err := toInt(tc.value)
		if err != nil {
			if tc.err == nil {
				t.Errorf("unexpected error: %v", err)
			} else if err != tc.err {
				t.Errorf("expected error %v but found %v", tc.err, err)
			}
		} else if tc.err != nil {
			t.Errorf("no error, but expected error %v", tc.err)
		} else if v != tc.expected {
			t.Errorf("expected value %v but found %v", tc.expected, v)
		}
	}
}

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

func TestToInt2(t *testing.T) {
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

	runTestCases(t, cases, func(v interface{}) (interface{}, error) {
		v, err := toInt(v)
		return v, err
	},
		func(a, b interface{}) bool {
			ai := a.(int)
			bi := b.(int)
			return ai == bi
		})
}

func TestToUint(t *testing.T) {
	type testCase struct {
		expected uint
		value    interface{}
		err      error
	}

	cases := []testCase{
		{5, int8(5), nil},
		{513, int16(513), nil},
		{5061732, int32(5061732), nil},
		{5061732431437132, int64(5061732431437132), nil},
		{313, float32(313.0), nil},
		{313, float64(313.0), nil},
		{0, int64(-1), ErrOutOfRange},
		{0, float32(313.3), ErrBadType},
		{0, float64(314.5), ErrBadType},
	}

	for _, tc := range cases {
		v, err := toUint(tc.value)
		if err != nil {
			if tc.err == nil {
				t.Errorf("unexpected error: %v", err)
			} else if err != tc.err {
				t.Errorf("expected error %v but found %v", tc.err, err)
			}
		} else if tc.err != nil {
			t.Errorf("no error, but expected error %v", tc.err)
		} else if v != tc.expected {
			t.Errorf("expected value %v but found %v", tc.expected, v)
		}
	}
}

func TestToFloat(t *testing.T) {
}

func TestUnmarshalHelper(t *testing.T) {
}

func TestUnmarshalHelper_TextUnmarshaler(t *testing.T) {
}
