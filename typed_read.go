package quonfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

// maxSafeInt is the largest integer a double holds exactly (2^53-1). An int
// read of a stored double succeeds only within +/-maxSafeInt, matching the
// int grammar `qfg verify` enforces.
const maxSafeInt = 1<<53 - 1

// readTyped is the typed-getter contract (qfg-zvyy), shared by every
// Get*Value method: a value of the requested type returns (value, true, nil);
// a wrong-type, malformed or unresolvable value returns (zero, false, err)
// and logs once per key, so the caller's default applies. A missing key is
// (zero, false, ErrNotFound) and is not logged.
func readTyped[T any](c *Client, key string, ctx *ContextSet, convert func(key string, v *Value) (T, error)) (T, bool, error) {
	var zero T
	val, ok, err := c.resolve(key, ctx)
	if err == nil && !ok {
		return zero, false, nil
	}
	if err == nil {
		var out T
		if out, err = convert(key, val); err == nil {
			return out, true, nil
		}
	}
	if !errors.Is(err, ErrNotFound) && !isNotReady(err) {
		c.warnReadFailure(key, err.Error())
	}
	return zero, false, err
}

// warnReadFailure logs once per config key per client that a read returned
// no value, so a broken config is not mistaken for "using the default".
func (c *Client) warnReadFailure(key, reason string) {
	if _, warned := c.readFailureWarned.LoadOrStore(key, struct{}{}); warned {
		return
	}
	if c.opts.Logger != nil {
		c.opts.Logger.Warn(fmt.Sprintf("quonfig: read of %q returned no value; the caller's default applies: %s", key, reason))
	}
}

func typeMismatch(key string, v *Value, want ValueType) error {
	return fmt.Errorf("%w: config %q has type %s, expected %s", ErrTypeMismatch, key, v.Type, want)
}

// malformed reports a stored number that did not parse; config.go keeps it
// as its raw string. Never echoes the value.
func malformed(key string, want ValueType) error {
	return fmt.Errorf("%w: config %q has a malformed %s value", ErrUnableToCoerce, key, want)
}

func stringOf(key string, v *Value) (string, error) {
	if v.Type == ValueTypeString || v.Type == ValueTypeLogLevel {
		if s, ok := v.Value.(string); ok {
			return s, nil
		}
	}
	return "", typeMismatch(key, v, ValueTypeString)
}

func boolOf(key string, v *Value) (bool, error) {
	if v.Type == ValueTypeBool {
		if b, ok := v.Value.(bool); ok {
			return b, nil
		}
	}
	return false, typeMismatch(key, v, ValueTypeBool)
}

// intOf reads an int config, or a double config holding a whole number
// within the safe int range. Fractional, out-of-range and non-finite doubles
// are type mismatches: an int read never truncates.
func intOf(key string, v *Value) (int64, error) {
	switch v.Type {
	case ValueTypeInt:
		switch n := v.Value.(type) {
		case int64:
			return n, nil
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return i, nil
			}
		}
		return 0, malformed(key, ValueTypeInt)
	case ValueTypeDouble:
		f, err := floatOf(key, v)
		if err != nil {
			return 0, err
		}
		if f != math.Trunc(f) || math.Abs(f) > maxSafeInt {
			return 0, typeMismatch(key, v, ValueTypeInt)
		}
		return int64(f), nil
	}
	return 0, typeMismatch(key, v, ValueTypeInt)
}

// floatOf reads a double config, or an int config (always exact as a
// double within the int grammar).
func floatOf(key string, v *Value) (float64, error) {
	switch v.Type {
	case ValueTypeDouble:
		switch n := v.Value.(type) {
		case float64:
			return n, nil
		case json.Number:
			if f, err := n.Float64(); err == nil {
				return f, nil
			}
		}
		return 0, malformed(key, ValueTypeDouble)
	case ValueTypeInt:
		i, err := intOf(key, v)
		if err != nil {
			return 0, err
		}
		return float64(i), nil
	}
	return 0, typeMismatch(key, v, ValueTypeDouble)
}

func stringSliceOf(key string, v *Value) ([]string, error) {
	if v.Type == ValueTypeStringList {
		switch l := v.Value.(type) {
		case []string:
			return l, nil
		case []interface{}:
			return v.StringListValue(), nil
		}
	}
	return nil, typeMismatch(key, v, ValueTypeStringList)
}

func jsonOf(key string, v *Value) (interface{}, error) {
	if v.Type == ValueTypeJSON {
		return v.Value, nil
	}
	return nil, typeMismatch(key, v, ValueTypeJSON)
}

func durationOf(key string, v *Value) (time.Duration, error) {
	s, isString := v.Value.(string)
	if v.Type != ValueTypeDuration || !isString {
		return 0, typeMismatch(key, v, ValueTypeDuration)
	}
	d, err := ParseISO8601Duration(s)
	if err != nil {
		return 0, fmt.Errorf("%w: config %q: %v", ErrUnableToCoerce, key, err)
	}
	return d, nil
}
