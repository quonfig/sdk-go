package quonfig

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// ValueType represents the type of a config value.
type ValueType string

const (
	ValueTypeBool           ValueType = "bool"
	ValueTypeInt            ValueType = "int"
	ValueTypeDouble         ValueType = "double"
	ValueTypeString         ValueType = "string"
	ValueTypeJSON           ValueType = "json"
	ValueTypeStringList     ValueType = "string_list"
	ValueTypeLogLevel       ValueType = "log_level"
	ValueTypeWeightedValues ValueType = "weighted_values"
	ValueTypeSchema         ValueType = "schema"
	ValueTypeProvided       ValueType = "provided"
	ValueTypeDuration       ValueType = "duration"
)

// ProvidedData holds the source info for ENV_VAR-provided values.
type ProvidedData struct {
	Source string `json:"source"`
	Lookup string `json:"lookup"`
}

// ConfigType represents the type of a config entry.
type ConfigType string

const (
	ConfigTypeFeatureFlag ConfigType = "feature_flag"
	ConfigTypeConfig      ConfigType = "config"
	ConfigTypeSegment     ConfigType = "segment"
	ConfigTypeLogLevel    ConfigType = "log_level_v2"
	ConfigTypeSchema      ConfigType = "schema"
)

// Value is the universal value wrapper.
type Value struct {
	Type         ValueType   `json:"type"`
	Value        interface{} `json:"value"`
	Confidential bool        `json:"confidential,omitempty"`
	DecryptWith  string      `json:"decryptWith,omitempty"`
}

// BoolValue returns the value as bool, or false.
func (v Value) BoolValue() bool {
	if b, ok := v.Value.(bool); ok {
		return b
	}
	return false
}

// StringValue returns the value as string, or "".
func (v Value) StringValue() string {
	if s, ok := v.Value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v.Value)
}

// IntValue returns the value as int64, or 0.
func (v Value) IntValue() int64 {
	switch n := v.Value.(type) {
	case int64:
		return n
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

// DoubleValue returns the value as float64, or 0.
func (v Value) DoubleValue() float64 {
	switch n := v.Value.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}

// StringListValue returns the value as []string, or nil.
func (v Value) StringListValue() []string {
	if sl, ok := v.Value.([]string); ok {
		return sl
	}
	if sl, ok := v.Value.([]interface{}); ok {
		result := make([]string, 0, len(sl))
		for _, item := range sl {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	}
	return nil
}

// WeightedValuesValue returns the weighted values data, or nil.
func (v Value) WeightedValuesValue() *WeightedValuesData {
	if wv, ok := v.Value.(*WeightedValuesData); ok {
		return wv
	}
	return nil
}

// ProvidedValue returns the provided data, or nil.
func (v Value) ProvidedValue() *ProvidedData {
	if pd, ok := v.Value.(*ProvidedData); ok {
		return pd
	}
	return nil
}

// UnmarshalJSON handles the polymorphic value field.
func (v *Value) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type         ValueType       `json:"type"`
		Value        json.RawMessage `json:"value"`
		Confidential bool            `json:"confidential,omitempty"`
		DecryptWith  string          `json:"decryptWith,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	v.Type = raw.Type
	v.Confidential = raw.Confidential
	v.DecryptWith = raw.DecryptWith

	if raw.Value == nil || string(raw.Value) == "null" {
		return nil
	}

	switch raw.Type {
	case ValueTypeBool:
		var b bool
		if err := json.Unmarshal(raw.Value, &b); err != nil {
			return err
		}
		v.Value = b
	case ValueTypeInt:
		var s string
		if err := json.Unmarshal(raw.Value, &s); err == nil {
			i, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid int value %q: %w", s, err)
			}
			v.Value = i
		} else {
			var n json.Number
			if err := json.Unmarshal(raw.Value, &n); err != nil {
				return err
			}
			i, err := n.Int64()
			if err != nil {
				return err
			}
			v.Value = i
		}
	case ValueTypeDouble:
		var f float64
		if err := json.Unmarshal(raw.Value, &f); err != nil {
			var s string
			if err := json.Unmarshal(raw.Value, &s); err != nil {
				return err
			}
			parsed, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return err
			}
			f = parsed
		}
		v.Value = f
	case ValueTypeString, ValueTypeJSON, ValueTypeLogLevel, ValueTypeDuration:
		var s string
		if err := json.Unmarshal(raw.Value, &s); err != nil {
			return err
		}
		v.Value = s
	case ValueTypeProvided:
		var pd ProvidedData
		if err := json.Unmarshal(raw.Value, &pd); err != nil {
			return err
		}
		v.Value = &pd
	case ValueTypeStringList:
		var sl []string
		if err := json.Unmarshal(raw.Value, &sl); err != nil {
			return err
		}
		v.Value = sl
	case ValueTypeWeightedValues:
		var wv WeightedValuesData
		if err := json.Unmarshal(raw.Value, &wv); err != nil {
			return err
		}
		v.Value = &wv
	case ValueTypeSchema:
		var sd SchemaData
		if err := json.Unmarshal(raw.Value, &sd); err != nil {
			return err
		}
		v.Value = &sd
	default:
		var val interface{}
		if err := json.Unmarshal(raw.Value, &val); err != nil {
			return err
		}
		v.Value = val
	}
	return nil
}

// WeightedValue is a single entry in a weighted distribution.
type WeightedValue struct {
	Weight int   `json:"weight"`
	Value  Value `json:"value"`
}

// WeightedValuesData holds weighted distribution data for A/B tests.
type WeightedValuesData struct {
	WeightedValues     []WeightedValue `json:"weightedValues"`
	HashByPropertyName string          `json:"hashByPropertyName,omitempty"`
}

// SchemaData holds schema validation data.
type SchemaData struct {
	SchemaType string `json:"schemaType"`
	Schema     string `json:"schema"`
}

// Criterion is a single condition in a rule.
type Criterion struct {
	PropertyName string `json:"propertyName,omitempty"`
	Operator     string `json:"operator"`
	ValueToMatch *Value `json:"valueToMatch,omitempty"`
}

// Rule is a set of criteria (AND logic) that produce a value.
type Rule struct {
	Criteria []Criterion `json:"criteria"`
	Value    Value       `json:"value"`
}

// RuleSet is a collection of rules (tried top to bottom, first match wins).
type RuleSet struct {
	Rules []Rule `json:"rules"`
}

// Environment is an environment-specific rule set.
type Environment struct {
	ID    string `json:"id"`
	Rules []Rule `json:"rules"`
}

// ConfigResponse is a single config in the download response, filtered to one environment.
type ConfigResponse struct {
	ID              string       `json:"id"`
	Key             string       `json:"key"`
	Type            ConfigType   `json:"type"`
	ValueType       ValueType    `json:"valueType"`
	SendToClientSDK bool         `json:"sendToClientSdk"`
	Default         RuleSet      `json:"default"`
	Environment     *Environment `json:"environment,omitempty"`
}

// ConfigEnvelope is the response wrapper for config downloads.
type ConfigEnvelope struct {
	Configs []ConfigResponse `json:"configs"`
	Meta    Meta             `json:"meta"`
}

// Meta holds response metadata.
type Meta struct {
	Version     string `json:"version"`
	Environment string `json:"environment"`
	WorkspaceID string `json:"workspaceId,omitempty"`
}
