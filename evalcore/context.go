package evalcore

import (
	"strings"
	"time"
)

// ContextValueGetter retrieves values from evaluation context.
type ContextValueGetter interface {
	GetContextValue(propertyName string) (interface{}, bool)
}

// EmptyContext is a ContextValueGetter that always returns (nil, false).
type EmptyContext struct{}

func (EmptyContext) GetContextValue(string) (interface{}, bool) {
	return nil, false
}

// ContextSet holds named contexts (e.g. "user", "device").
type ContextSet struct {
	Data map[string]*NamedContext
}

// NamedContext is a single named context (e.g. "user" with email, id, etc.).
type NamedContext struct {
	Name string
	Data map[string]interface{}
}

// NewContextSet creates an empty ContextSet.
func NewContextSet() *ContextSet {
	return &ContextSet{
		Data: make(map[string]*NamedContext),
	}
}

// NewNamedContext creates a NamedContext with the given name and data.
func NewNamedContext(name string, data map[string]interface{}) *NamedContext {
	return &NamedContext{
		Name: name,
		Data: data,
	}
}

// WithNamedContext adds a named context and returns the ContextSet for chaining.
func (cs *ContextSet) WithNamedContext(nc *NamedContext) *ContextSet {
	cs.Data[nc.Name] = nc
	return cs
}

// GetContextValue looks up a dotted property name (e.g. "user.email") by splitting
// at the first dot, finding the named context, and looking up the key.
// Special properties "prefab.current-time", "quonfig.current-time", and
// "reforge.current-time" return current UTC time in milliseconds since epoch.
func (cs *ContextSet) GetContextValue(propertyName string) (interface{}, bool) {
	// Handle magic current-time properties
	if propertyName == "prefab.current-time" || propertyName == "quonfig.current-time" || propertyName == "reforge.current-time" {
		return time.Now().UTC().UnixMilli(), true
	}

	contextName, key := splitAtFirstDot(propertyName)
	if contextName == "" {
		return nil, false
	}

	nc, ok := cs.Data[contextName]
	if !ok {
		return nil, false
	}

	value, exists := nc.Data[key]
	return value, exists
}

// splitAtFirstDot splits at the first "." and returns (before, after).
// If there is no dot, returns ("", input).
func splitAtFirstDot(input string) (string, string) {
	parts := strings.SplitN(input, ".", 2)
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}
