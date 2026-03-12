package quonfig

import "strings"

// NamedContext is a named context providing key-value data about a user, team, device, etc.
type NamedContext struct {
	Name string
	Data map[string]interface{}
}

// ContextSet is a set of named contexts used for config evaluation.
type ContextSet struct {
	data map[string]*NamedContext
}

// NewContextSet creates a new empty ContextSet.
func NewContextSet() *ContextSet {
	return &ContextSet{
		data: make(map[string]*NamedContext),
	}
}

// WithNamedContextValues adds or replaces a named context with the given values, returning the ContextSet for chaining.
func (cs *ContextSet) WithNamedContextValues(name string, values map[string]interface{}) *ContextSet {
	cs.data[name] = &NamedContext{
		Name: name,
		Data: values,
	}
	return cs
}

// SetNamedContext adds or replaces a named context.
func (cs *ContextSet) SetNamedContext(nc *NamedContext) {
	cs.data[nc.Name] = nc
}

// GetContextValue looks up a value by dotted property name.
// The part before the first dot is the context name; the part after is the key within that context.
// If there is no dot, the unnamed ("") context is searched.
func (cs *ContextSet) GetContextValue(propertyName string) (interface{}, bool) {
	contextName, key := splitAtFirstDot(propertyName)
	if nc, ok := cs.data[contextName]; ok {
		val, exists := nc.Data[key]
		return val, exists
	}
	return nil, false
}

// Merge returns a new ContextSet that combines all provided context sets.
// Later sets take precedence over earlier ones for the same context name.
func Merge(sets ...*ContextSet) *ContextSet {
	result := NewContextSet()
	for _, cs := range sets {
		if cs == nil {
			continue
		}
		for name, nc := range cs.data {
			result.data[name] = nc
		}
	}
	return result
}

// splitAtFirstDot splits on the first "." and returns (before, after).
// If no dot is found, returns ("", input).
func splitAtFirstDot(input string) (string, string) {
	parts := strings.SplitN(input, ".", 2)
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}
