package evalcore

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Operator constants -- these match the string values in criterion JSON.
const (
	OpNotSet                    = "NOT_SET"
	OpAlwaysTrue                = "ALWAYS_TRUE"
	OpPropIsOneOf               = "PROP_IS_ONE_OF"
	OpPropIsNotOneOf            = "PROP_IS_NOT_ONE_OF"
	OpPropStartsWithOneOf       = "PROP_STARTS_WITH_ONE_OF"
	OpPropDoesNotStartWithOneOf = "PROP_DOES_NOT_START_WITH_ONE_OF"
	OpPropEndsWithOneOf         = "PROP_ENDS_WITH_ONE_OF"
	OpPropDoesNotEndWithOneOf   = "PROP_DOES_NOT_END_WITH_ONE_OF"
	OpPropContainsOneOf         = "PROP_CONTAINS_ONE_OF"
	OpPropDoesNotContainOneOf   = "PROP_DOES_NOT_CONTAIN_ONE_OF"
	OpPropMatches               = "PROP_MATCHES"
	OpPropDoesNotMatch          = "PROP_DOES_NOT_MATCH"
	OpHierarchicalMatch         = "HIERARCHICAL_MATCH"
	OpInIntRange                = "IN_INT_RANGE"
	OpPropGreaterThan           = "PROP_GREATER_THAN"
	OpPropGreaterThanOrEqual    = "PROP_GREATER_THAN_OR_EQUAL"
	OpPropLessThan              = "PROP_LESS_THAN"
	OpPropLessThanOrEqual       = "PROP_LESS_THAN_OR_EQUAL"
	OpPropBefore                = "PROP_BEFORE"
	OpPropAfter                 = "PROP_AFTER"
	OpPropSemverLessThan        = "PROP_SEMVER_LESS_THAN"
	OpPropSemverEqual           = "PROP_SEMVER_EQUAL"
	OpPropSemverGreaterThan     = "PROP_SEMVER_GREATER_THAN"
	OpInSeg                     = "IN_SEG"
	OpNotInSeg                  = "NOT_IN_SEG"
)

// SegmentResolver is the callback for resolving IN_SEG / NOT_IN_SEG operators.
// It evaluates the referenced segment config and returns (boolResult, found).
type SegmentResolver func(segmentKey string) (bool, bool)

// EvaluateCriterion evaluates a single criterion against a context value.
// segmentResolver is used for IN_SEG/NOT_IN_SEG and may be nil if segments are not needed.
func EvaluateCriterion(contextValue interface{}, contextExists bool, criterion Criterion, segmentResolver SegmentResolver) bool {
	matchValue := criterion.ValueToMatch

	switch criterion.Operator {
	case OpNotSet:
		return false

	case OpAlwaysTrue:
		return true

	case OpPropIsOneOf, OpPropIsNotOneOf:
		if contextExists && matchValue != nil {
			matchStrings := matchValue.StringListValue()
			if matchStrings != nil {
				contextStrings := toStringSlice(contextValue)
				matchFound := false
				for _, cv := range contextStrings {
					if stringInSlice(cv, matchStrings) {
						matchFound = true
						break
					}
				}
				return matchFound == (criterion.Operator == OpPropIsOneOf)
			}
		}
		return criterion.Operator == OpPropIsNotOneOf

	case OpPropStartsWithOneOf, OpPropDoesNotStartWithOneOf:
		if contextExists && matchValue != nil {
			matchStrings := matchValue.StringListValue()
			if matchStrings != nil {
				cv := ToString(contextValue)
				matchFound := startsWithAny(matchStrings, cv)
				return matchFound == (criterion.Operator == OpPropStartsWithOneOf)
			}
		}
		return criterion.Operator == OpPropDoesNotStartWithOneOf

	case OpPropEndsWithOneOf, OpPropDoesNotEndWithOneOf:
		if contextExists && matchValue != nil {
			matchStrings := matchValue.StringListValue()
			if matchStrings != nil {
				cv := ToString(contextValue)
				matchFound := endsWithAny(matchStrings, cv)
				return matchFound == (criterion.Operator == OpPropEndsWithOneOf)
			}
		}
		return criterion.Operator == OpPropDoesNotEndWithOneOf

	case OpPropContainsOneOf, OpPropDoesNotContainOneOf:
		if contextExists && matchValue != nil {
			matchStrings := matchValue.StringListValue()
			if matchStrings != nil {
				cv := ToString(contextValue)
				matchFound := containsAny(matchStrings, cv)
				return matchFound == (criterion.Operator == OpPropContainsOneOf)
			}
		}
		return criterion.Operator == OpPropDoesNotContainOneOf

	case OpPropMatches, OpPropDoesNotMatch:
		if contextExists && matchValue != nil && isString(contextValue) && isString(matchValue.Value) {
			re, err := regexp.Compile(matchValue.StringValue())
			if err == nil {
				matched := re.MatchString(contextValue.(string))
				return matched == (criterion.Operator == OpPropMatches)
			}
		}
		return false

	case OpHierarchicalMatch:
		if contextExists && matchValue != nil {
			cv := ToString(contextValue)
			mv := matchValue.StringValue()
			return strings.HasPrefix(cv, mv)
		}
		return false

	case OpInIntRange:
		if contextExists && matchValue != nil {
			start, end := extractIntRange(matchValue)
			if numVal, ok := ToFloat64(contextValue); ok {
				return numVal >= float64(start) && numVal < float64(end)
			}
		}
		return false

	case OpPropGreaterThan, OpPropGreaterThanOrEqual, OpPropLessThan, OpPropLessThanOrEqual:
		if contextExists && matchValue != nil && IsNumber(contextValue) && IsNumber(matchValue.Value) {
			cmp, err := compareNumbers(contextValue, matchValue.Value)
			if err == nil {
				switch criterion.Operator {
				case OpPropGreaterThan:
					return cmp > 0
				case OpPropGreaterThanOrEqual:
					return cmp >= 0
				case OpPropLessThan:
					return cmp < 0
				case OpPropLessThanOrEqual:
					return cmp <= 0
				}
			}
		}
		return false

	case OpPropBefore, OpPropAfter:
		if contextExists && matchValue != nil {
			contextMillis, err1 := dateToMillis(contextValue)
			matchMillis, err2 := dateToMillis(matchValue.Value)
			if err1 == nil && err2 == nil {
				if criterion.Operator == OpPropBefore {
					return contextMillis < matchMillis
				}
				return contextMillis > matchMillis
			}
		}
		return false

	case OpPropSemverLessThan, OpPropSemverEqual, OpPropSemverGreaterThan:
		if contextExists && matchValue != nil && isString(contextValue) && isString(matchValue.Value) {
			svContext := ParseSemverQuietly(contextValue.(string))
			svMatch := ParseSemverQuietly(matchValue.StringValue())
			if svContext != nil && svMatch != nil {
				cmp := svContext.Compare(*svMatch)
				switch criterion.Operator {
				case OpPropSemverLessThan:
					return cmp < 0
				case OpPropSemverEqual:
					return cmp == 0
				case OpPropSemverGreaterThan:
					return cmp > 0
				}
			}
		}
		return false

	case OpInSeg, OpNotInSeg:
		if matchValue != nil && segmentResolver != nil {
			segmentKey := matchValue.StringValue()
			result, found := segmentResolver(segmentKey)
			if !found {
				return criterion.Operator == OpNotInSeg
			}
			return result == (criterion.Operator == OpInSeg)
		}
		return criterion.Operator == OpNotInSeg
	}

	return false
}

// extractIntRange extracts start and end from a Value that represents an int range.
func extractIntRange(v *Value) (int64, int64) {
	start := int64(math.MinInt64)
	end := int64(math.MaxInt64)

	if v == nil {
		return start, end
	}

	// If the value is a map (common from JSON unmarshaling of int_range)
	if m, ok := v.Value.(map[string]interface{}); ok {
		if s, ok := m["start"]; ok {
			start = toInt64FromAny(s)
		}
		if e, ok := m["end"]; ok {
			end = toInt64FromAny(e)
		}
		return start, end
	}

	return start, end
}

// toInt64FromAny converts various numeric types to int64.
func toInt64FromAny(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	}
	return 0
}

// ToString converts a context value to a string.
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// toStringSlice converts a context value to a string slice.
func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	if ss, ok := v.([]string); ok {
		return ss
	}
	if reflect.TypeOf(v).Kind() == reflect.Slice {
		rv := reflect.ValueOf(v)
		result := make([]string, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			result[i] = ToString(rv.Index(i).Interface())
		}
		return result
	}
	return []string{ToString(v)}
}

// isString checks if a value is a string.
func isString(v interface{}) bool {
	_, ok := v.(string)
	return ok
}

// IsNumber checks if a value is a numeric type.
func IsNumber(v interface{}) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, json.Number:
		return true
	}
	return false
}

// ToFloat64 converts a value to float64 if possible.
func ToFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	}
	return 0, false
}

// compareNumbers compares two numeric values. Returns -1, 0, or 1.
func compareNumbers(a, b interface{}) (int, error) {
	aFloat, aOk := ToFloat64(a)
	if !aOk {
		return 0, fmt.Errorf("cannot convert %v to float64", a)
	}
	bFloat, bOk := ToFloat64(b)
	if !bOk {
		return 0, fmt.Errorf("cannot convert %v to float64", b)
	}
	if aFloat < bFloat {
		return -1, nil
	}
	if aFloat > bFloat {
		return 1, nil
	}
	return 0, nil
}

// dateToMillis converts a value (int64 millis or RFC3339 string) to milliseconds since epoch.
func dateToMillis(val interface{}) (int64, error) {
	if IsNumber(val) {
		f, ok := ToFloat64(val)
		if ok {
			return int64(f), nil
		}
		return 0, fmt.Errorf("cannot convert %v to number", val)
	}
	if s, ok := val.(string); ok {
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			return parsed.UnixMilli(), nil
		}
		// Try parsing as a plain number string
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i, nil
		}
		return 0, fmt.Errorf("unable to parse date: %s", s)
	}
	return 0, fmt.Errorf("unsupported type for date: %T", val)
}

// String helper functions

func stringInSlice(s string, list []string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func startsWithAny(prefixes []string, target string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(target, p) {
			return true
		}
	}
	return false
}

func endsWithAny(suffixes []string, target string) bool {
	for _, s := range suffixes {
		if strings.HasSuffix(target, s) {
			return true
		}
	}
	return false
}

func containsAny(substrings []string, target string) bool {
	for _, s := range substrings {
		if strings.Contains(target, s) {
			return true
		}
	}
	return false
}
