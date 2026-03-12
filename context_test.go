package quonfig

import "testing"

func TestContextSetBasicOperations(t *testing.T) {
	cs := NewContextSet()
	cs.WithNamedContextValues("user", map[string]interface{}{
		"key":   "u123",
		"email": "me@example.com",
		"admin": true,
		"age":   int64(42),
	})
	cs.WithNamedContextValues("team", map[string]interface{}{
		"key":  "t123",
		"name": "dev ops",
	})

	tests := []struct {
		name     string
		property string
		wantVal  interface{}
		wantOK   bool
	}{
		{"dotted user key", "user.key", "u123", true},
		{"dotted user email", "user.email", "me@example.com", true},
		{"dotted user admin", "user.admin", true, true},
		{"dotted user age", "user.age", int64(42), true},
		{"dotted team name", "team.name", "dev ops", true},
		{"missing context", "missing.key", nil, false},
		{"missing key in context", "user.missing", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := cs.GetContextValue(tt.property)
			if ok != tt.wantOK {
				t.Errorf("GetContextValue(%q) ok = %v, want %v", tt.property, ok, tt.wantOK)
			}
			if val != tt.wantVal {
				t.Errorf("GetContextValue(%q) = %v, want %v", tt.property, val, tt.wantVal)
			}
		})
	}
}

func TestContextSetUnnamedContext(t *testing.T) {
	cs := NewContextSet()
	cs.WithNamedContextValues("", map[string]interface{}{
		"key": "anon123",
		"id":  int64(99),
	})

	val, ok := cs.GetContextValue("key")
	if !ok {
		t.Fatal("expected to find key in unnamed context")
	}
	if val != "anon123" {
		t.Errorf("got %v, want anon123", val)
	}

	val, ok = cs.GetContextValue("id")
	if !ok {
		t.Fatal("expected to find id in unnamed context")
	}
	if val != int64(99) {
		t.Errorf("got %v, want 99", val)
	}
}

func TestMerge(t *testing.T) {
	cs1 := NewContextSet()
	cs1.WithNamedContextValues("user", map[string]interface{}{"name": "alice"})
	cs1.WithNamedContextValues("team", map[string]interface{}{"name": "alpha"})

	cs2 := NewContextSet()
	cs2.WithNamedContextValues("user", map[string]interface{}{"name": "bob"})

	merged := Merge(cs1, cs2)

	// user should come from cs2 (later wins)
	val, ok := merged.GetContextValue("user.name")
	if !ok || val != "bob" {
		t.Errorf("expected user.name=bob after merge, got %v (ok=%v)", val, ok)
	}

	// team should still be from cs1
	val, ok = merged.GetContextValue("team.name")
	if !ok || val != "alpha" {
		t.Errorf("expected team.name=alpha after merge, got %v (ok=%v)", val, ok)
	}
}

func TestMergeWithNil(t *testing.T) {
	cs := NewContextSet()
	cs.WithNamedContextValues("user", map[string]interface{}{"name": "alice"})

	merged := Merge(nil, cs, nil)

	val, ok := merged.GetContextValue("user.name")
	if !ok || val != "alice" {
		t.Errorf("expected user.name=alice after merge with nils, got %v", val)
	}
}

func TestSetNamedContext(t *testing.T) {
	cs := NewContextSet()
	cs.SetNamedContext(&NamedContext{
		Name: "device",
		Data: map[string]interface{}{"os": "linux"},
	})

	val, ok := cs.GetContextValue("device.os")
	if !ok || val != "linux" {
		t.Errorf("expected device.os=linux, got %v", val)
	}
}
