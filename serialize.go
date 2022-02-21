package mpnethack

import (
	"encoding"
	"errors"
	"math"

	"github.com/BurntSushi/toml"
)

var ErrInvalidTOML = errors.New("invalid TOML")
var ErrUnknownKey = errors.New("unknown key")
var ErrBadType = errors.New("bad type")
var ErrMissingKey = errors.New("missing key")
var ErrOutOfRange = errors.New("value out of range")

type UnmarshalHelperFlags uint

const (
	ErrorOnUnknownKey UnmarshalHelperFlags = 1 << iota
	ErrorOnMissingKey
)

func toInt(v interface{}) (ival int, err error) {
	switch v := v.(type) {
	case int:
		return v, nil

	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil

	case uint:
		if v > math.MaxInt {
			return 0, ErrOutOfRange
		}
		return int(v), nil

	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		if math.MaxInt == math.MaxInt32 && v > math.MaxInt32 {
			return 0, ErrOutOfRange
		}
		return int(v), nil

	case uint64:
		if v > math.MaxInt {
			return 0, ErrOutOfRange
		}
		return int(v), nil

	case float32:
		v64 := float64(v)
		tr := math.Trunc(v64)
		if tr == v64 {
			return int(v64), nil
		} else {
			return 0, ErrBadType
		}
	case float64:
		tr := math.Trunc(v)
		if tr == v {
			return int(v), nil
		} else {
			return 0, ErrBadType
		}

	default:
		return 0, ErrBadType
	}
}

func toUint(v interface{}) (uval uint, err error) {
	switch v := v.(type) {
	case int:
		if v < 0 {
			return 0, ErrOutOfRange
		}
		return uint(v), nil

	case int8:
		if v < 0 {
			return 0, ErrOutOfRange
		}
		return uint(v), nil
	case int16:
		if v < 0 {
			return 0, ErrOutOfRange
		}
		return uint(v), nil
	case int32:
		if v < 0 {
			return 0, ErrOutOfRange
		}
		return uint(v), nil
	case int64:
		if v < 0 || (math.MaxUint == math.MaxUint32 && v > math.MaxUint32) {
			return 0, ErrOutOfRange
		}

		return uint(v), nil

	case uint:
		return v, nil

	case uint8:
		return uint(v), nil
	case uint16:
		return uint(v), nil
	case uint32:
		return uint(v), nil

	case uint64:
		if v > math.MaxUint {
			return 0, ErrOutOfRange
		}
		return uint(v), nil

	case float32:
		if v < 0 || v > math.MaxUint {
			return 0, ErrOutOfRange
		}

		v64 := float64(v)
		tr := math.Trunc(v64)
		if tr == v64 {
			return uint(v64), nil
		} else {
			return 0, ErrBadType
		}

	case float64:
		if v < 0 || v > math.MaxUint {
			return 0, ErrOutOfRange
		}

		tr := math.Trunc(v)
		if tr == v {
			return uint(v), nil
		} else {
			return 0, ErrBadType
		}

	default:
		return 0, ErrBadType
	}
}

func toFloat(v interface{}) (fval float64, err error) {
	switch v := v.(type) {
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil

	default:
		return 0, ErrBadType
	}
}

func unmarshalHelper(data interface{}, dest map[string]interface{}, flags UnmarshalHelperFlags) error {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return ErrInvalidTOML
	}

	type NameSet map[string]struct{}
	inSet := struct{}{}

	names := NameSet{}
	for k, v := range dataMap {
		destVal, ok := dest[k]
		if !ok && (flags&ErrorOnUnknownKey) != 0 {
			// FIXME: need to provide th key...
			return ErrUnknownKey
		}

		switch obj := destVal.(type) {
		case toml.Unmarshaler:
			err := obj.UnmarshalTOML(v)
			if err != nil {
				return err
			}

		case encoding.TextUnmarshaler:
			if s, ok := v.(string); ok {
				return obj.UnmarshalText([]byte(s))
			} else {
				return ErrBadType
			}

		case *int:
			ival, err := toInt(v)
			if err != nil {
				return ErrBadType
			}
			*obj = ival

		case *uint:
			uval, err := toUint(v)
			if err != nil {
				return ErrBadType
			}
			*obj = uval

		case *float32:
			fval, err := toFloat(v)
			if err != nil {
				return ErrBadType
			}
			*obj = float32(fval)

		case *float64:
			fval, err := toFloat(v)
			if err != nil {
				return ErrBadType
			}
			*obj = fval

		case *string:
			sval, ok := v.(string)
			if !ok {
				return ErrBadType
			}

			*obj = sval
		}

		names[k] = inSet
	}

	if (flags & ErrorOnMissingKey) != 0 {
		for k := range dataMap {
			if _, ok := names[k]; !ok {
				return ErrMissingKey
			}
		}
	}

	return nil
}
