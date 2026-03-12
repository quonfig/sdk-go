package quonfig

import "testing"

// memStore is a simple in-memory store for testing.
type memStore struct {
	configs map[string]*ConfigResponse
}

func (m *memStore) Get(key string) (*ConfigResponse, bool) {
	c, ok := m.configs[key]
	return c, ok
}

func (m *memStore) Keys() []string {
	keys := make([]string, 0, len(m.configs))
	for k := range m.configs {
		keys = append(keys, k)
	}
	return keys
}

func newMemStore(configs map[string]*ConfigResponse) *memStore {
	return &memStore{configs: configs}
}

func TestClientGetStringValue(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	client.SetStore(newMemStore(map[string]*ConfigResponse{
		"app.name": {
			Key:       "app.name",
			ValueType: ValueTypeString,
			Default: RuleSet{
				Rules: []Rule{
					{Value: Value{Type: ValueTypeString, Value: "myapp"}},
				},
			},
		},
	}))

	val, ok, err := client.GetStringValue("app.name", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if val != "myapp" {
		t.Errorf("expected myapp, got %s", val)
	}
}

func TestClientGetBoolValue(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	client.SetStore(newMemStore(map[string]*ConfigResponse{
		"feature.on": {
			Key:       "feature.on",
			ValueType: ValueTypeBool,
			Default: RuleSet{
				Rules: []Rule{
					{Value: Value{Type: ValueTypeBool, Value: true}},
				},
			},
		},
	}))

	val, ok, err := client.GetBoolValue("feature.on", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !val {
		t.Errorf("expected true, got %v (ok=%v)", val, ok)
	}
}

func TestClientGetIntValue(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	client.SetStore(newMemStore(map[string]*ConfigResponse{
		"max.retries": {
			Key:       "max.retries",
			ValueType: ValueTypeInt,
			Default: RuleSet{
				Rules: []Rule{
					{Value: Value{Type: ValueTypeInt, Value: int64(5)}},
				},
			},
		},
	}))

	val, ok, err := client.GetIntValue("max.retries", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || val != 5 {
		t.Errorf("expected 5, got %d (ok=%v)", val, ok)
	}
}

func TestClientGetFloatValue(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	client.SetStore(newMemStore(map[string]*ConfigResponse{
		"rate.limit": {
			Key:       "rate.limit",
			ValueType: ValueTypeDouble,
			Default: RuleSet{
				Rules: []Rule{
					{Value: Value{Type: ValueTypeDouble, Value: 0.75}},
				},
			},
		},
	}))

	val, ok, err := client.GetFloatValue("rate.limit", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || val != 0.75 {
		t.Errorf("expected 0.75, got %f (ok=%v)", val, ok)
	}
}

func TestClientFeatureIsOn(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	client.SetStore(newMemStore(map[string]*ConfigResponse{
		"feature.dark-mode": {
			Key:       "feature.dark-mode",
			ValueType: ValueTypeBool,
			Default: RuleSet{
				Rules: []Rule{
					{Value: Value{Type: ValueTypeBool, Value: true}},
				},
			},
		},
	}))

	on, found := client.FeatureIsOn("feature.dark-mode", nil)
	if !found || !on {
		t.Errorf("expected feature to be on, got on=%v found=%v", on, found)
	}

	on, found = client.FeatureIsOn("nonexistent", nil)
	if found || on {
		t.Errorf("expected not found, got on=%v found=%v", on, found)
	}
}

func TestClientNotFound(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	client.SetStore(newMemStore(map[string]*ConfigResponse{}))

	_, _, err = client.GetStringValue("missing", nil)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestClientNoStore(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = client.GetStringValue("anything", nil)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestContextBoundClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	client.SetStore(newMemStore(map[string]*ConfigResponse{
		"app.name": {
			Key:       "app.name",
			ValueType: ValueTypeString,
			Default: RuleSet{
				Rules: []Rule{
					{Value: Value{Type: ValueTypeString, Value: "bound-app"}},
				},
			},
		},
	}))

	ctx := NewContextSet().WithNamedContextValues("user", map[string]interface{}{"id": "u1"})
	bound := client.WithContext(ctx)

	val, ok, err := bound.GetStringValue("app.name")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || val != "bound-app" {
		t.Errorf("expected bound-app, got %s (ok=%v)", val, ok)
	}
}

func TestClientKeys(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	client.SetStore(newMemStore(map[string]*ConfigResponse{
		"a": {},
		"b": {},
	}))

	keys := client.Keys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}
