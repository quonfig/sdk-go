package evalcore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// testConfigStore is a simple in-memory config store for tests.
type testConfigStore struct {
	configs map[string]*Config
}

func newTestConfigStore() *testConfigStore {
	return &testConfigStore{configs: make(map[string]*Config)}
}

func (s *testConfigStore) GetConfig(key string) (*Config, bool) {
	c, ok := s.configs[key]
	return c, ok
}

func (s *testConfigStore) addConfig(cfg *Config) {
	s.configs[cfg.Key] = cfg
}

func loadConfig(t *testing.T, path string) *Config {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal %s: %v", path, err)
	}
	return &cfg
}

func testdataPath(parts ...string) string {
	return filepath.Join(append([]string{"testdata"}, parts...)...)
}

// --- Basic evaluation tests ---

func TestEvaluateConfig_AlwaysTrue(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "test-flag-1.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	result := eval.EvaluateConfig(cfg, "", nil)

	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value == nil {
		t.Fatal("expected Value to not be nil")
	}
	if result.Value.Type != ValueTypeBool {
		t.Errorf("expected ValueTypeBool, got %s", result.Value.Type)
	}
	if result.Value.BoolValue() != false {
		t.Error("expected false")
	}
}

func TestEvaluateConfig_EnvironmentSpecific_Production(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "feature.new-ui.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	// Production should return true
	result := eval.EvaluateConfig(cfg, "Production", nil)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value == nil {
		t.Fatal("expected Value to not be nil")
	}
	if result.Value.BoolValue() != true {
		t.Error("expected true for Production")
	}

	// No environment (default) should return false
	result = eval.EvaluateConfig(cfg, "", nil)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value == nil {
		t.Fatal("expected Value to not be nil")
	}
	if result.Value.BoolValue() != false {
		t.Error("expected false for default")
	}
}

func TestEvaluateConfig_EnvironmentFallback(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "feature.new-ui.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	// Unknown environment should fall back to default
	result := eval.EvaluateConfig(cfg, "SomeOtherEnv", nil)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value == nil {
		t.Fatal("expected Value to not be nil")
	}
	if result.Value.BoolValue() != false {
		t.Error("expected false for unknown env")
	}
}

func TestEvaluateConfig_MultipleEnvironments(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "flag-with-variant.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	// Production returns "a"
	result := eval.EvaluateConfig(cfg, "Production", nil)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value.StringValue() != "a" {
		t.Errorf("expected 'a', got %q", result.Value.StringValue())
	}

	// Staging2 returns "other"
	result = eval.EvaluateConfig(cfg, "Staging2", nil)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value.StringValue() != "other" {
		t.Errorf("expected 'other', got %q", result.Value.StringValue())
	}

	// Default returns "a"
	result = eval.EvaluateConfig(cfg, "", nil)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value.StringValue() != "a" {
		t.Errorf("expected 'a', got %q", result.Value.StringValue())
	}
}

func TestEvaluateConfig_StringConfig(t *testing.T) {
	cfg := loadConfig(t, testdataPath("configs", "test-config.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	result := eval.EvaluateConfig(cfg, "", nil)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value.StringValue() != "test-value" {
		t.Errorf("expected 'test-value', got %q", result.Value.StringValue())
	}
}

// --- PROP_IS_ONE_OF tests ---

func TestEvaluateConfig_PropIsOneOf_Match(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "ben-test-1.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("identity", map[string]interface{}{
		"id": "ben",
	}))

	result := eval.EvaluateConfig(cfg, "", ctx)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value.BoolValue() != false {
		t.Error("expected false")
	}
	if result.RuleIndex != 0 {
		t.Errorf("expected RuleIndex 0, got %d", result.RuleIndex)
	}
}

func TestEvaluateConfig_PropIsOneOf_NoMatch(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "ben-test-1.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("identity", map[string]interface{}{
		"id": "alice",
	}))

	result := eval.EvaluateConfig(cfg, "", ctx)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value.BoolValue() != true {
		t.Error("expected true")
	}
	if result.RuleIndex != 1 {
		t.Errorf("expected RuleIndex 1, got %d", result.RuleIndex)
	}
}

func TestEvaluateConfig_PropIsOneOf_NoContext(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "ben-test-1.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	result := eval.EvaluateConfig(cfg, "", nil)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value.BoolValue() != true {
		t.Error("expected true")
	}
}

func TestEvaluateConfig_PropIsOneOf_EnvOverride(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "ben-test-1.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("identity", map[string]interface{}{
		"id": "ben",
	}))

	result := eval.EvaluateConfig(cfg, "Production", ctx)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value.BoolValue() != true {
		t.Error("expected true for Production override")
	}
}

// --- Weighted values tests ---

func TestEvaluateConfig_WeightedValues_Deterministic(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "exp-example.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"key": "user-123",
	}))

	result1 := eval.EvaluateConfig(cfg, "Production", ctx)
	result2 := eval.EvaluateConfig(cfg, "Production", ctx)

	if !result1.IsMatch || !result2.IsMatch {
		t.Fatal("expected both results to match")
	}
	if result1.Value.StringValue() != result2.Value.StringValue() {
		t.Errorf("expected deterministic results, got %q and %q", result1.Value.StringValue(), result2.Value.StringValue())
	}
	if result1.WeightedValueIndex != result2.WeightedValueIndex {
		t.Errorf("expected same WeightedValueIndex, got %d and %d", result1.WeightedValueIndex, result2.WeightedValueIndex)
	}
}

func TestEvaluateConfig_WeightedValues_DefaultFallback(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "exp-example.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	result := eval.EvaluateConfig(cfg, "", nil)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value.StringValue() != "control" {
		t.Errorf("expected 'control', got %q", result.Value.StringValue())
	}
}

func TestEvaluateConfig_WeightedValues_DifferentUsers(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "exp-example.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	results := make(map[string]int)
	for i := 0; i < 300; i++ {
		ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
			"key": i,
		}))
		result := eval.EvaluateConfig(cfg, "Production", ctx)
		if !result.IsMatch || result.Value == nil {
			t.Fatalf("expected match for user %d", i)
		}
		results[result.Value.StringValue()]++
	}

	if results["control"] < 50 {
		t.Errorf("control should have significant representation, got %d", results["control"])
	}
	if results["arm-1"] < 50 {
		t.Errorf("arm-1 should have significant representation, got %d", results["arm-1"])
	}
	if results["arm-2"] < 50 {
		t.Errorf("arm-2 should have significant representation, got %d", results["arm-2"])
	}
}

func TestEvaluateConfig_WeightedValues_Bool(t *testing.T) {
	cfg := loadConfig(t, testdataPath("feature-flags", "a-new-flag.json"))
	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	results := make(map[bool]int)
	for i := 0; i < 1000; i++ {
		ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
			"key": i,
		}))
		result := eval.EvaluateConfig(cfg, "Production", ctx)
		if !result.IsMatch || result.Value == nil {
			t.Fatalf("expected match for user %d", i)
		}
		results[result.Value.BoolValue()]++
	}

	if results[false] < 400 {
		t.Errorf("false should dominate (59%% weight), got %d", results[false])
	}
	if results[true] < 200 {
		t.Errorf("true should have significant representation (41%% weight), got %d", results[true])
	}
}

// --- Segment tests ---

func TestEvaluateConfig_Segment(t *testing.T) {
	segmentCfg := loadConfig(t, testdataPath("segments", "segway.json"))
	store := newTestConfigStore()
	store.addConfig(segmentCfg)
	eval := NewEvaluatorWithSeed(store, 42)

	flagJSON := `{
		"id": "test-seg-flag",
		"key": "test-seg-flag",
		"type": "feature_flag",
		"valueType": "bool",
		"sendToClientSdk": false,
		"default": {
			"rules": [
				{
					"criteria": [
						{
							"propertyName": "segway",
							"operator": "IN_SEG",
							"valueToMatch": {
								"type": "string",
								"value": "segway"
							}
						}
					],
					"value": {
						"type": "bool",
						"value": true
					}
				},
				{
					"criteria": [
						{
							"propertyName": "",
							"operator": "ALWAYS_TRUE"
						}
					],
					"value": {
						"type": "bool",
						"value": false
					}
				}
			]
		}
	}`

	var flagCfg Config
	if err := json.Unmarshal([]byte(flagJSON), &flagCfg); err != nil {
		t.Fatalf("failed to unmarshal flag: %v", err)
	}
	store.addConfig(&flagCfg)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "foo@bar.com",
	}))
	result := eval.EvaluateConfig(&flagCfg, "", ctx)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value.BoolValue() != true {
		t.Error("expected true for matching segment user")
	}

	ctx2 := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "other@example.com",
	}))
	result2 := eval.EvaluateConfig(&flagCfg, "", ctx2)
	if !result2.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
}

func TestEvaluateConfig_NotInSeg(t *testing.T) {
	segmentCfg := &Config{
		Key:       "beta-users",
		Type:      ConfigTypeSegment,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{
							PropertyName: "user.email",
							Operator:     OpPropIsOneOf,
							ValueToMatch: &Value{
								Type:  ValueTypeStringList,
								Value: []string{"beta@example.com"},
							},
						},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{
						{Operator: OpAlwaysTrue},
					},
					Value: Value{Type: ValueTypeBool, Value: false},
				},
			},
		},
	}

	flagCfg := &Config{
		Key:       "show-banner",
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{
							Operator: OpNotInSeg,
							ValueToMatch: &Value{
								Type:  ValueTypeString,
								Value: "beta-users",
							},
						},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{
						{Operator: OpAlwaysTrue},
					},
					Value: Value{Type: ValueTypeBool, Value: false},
				},
			},
		},
	}

	store := newTestConfigStore()
	store.addConfig(segmentCfg)
	store.addConfig(flagCfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "beta@example.com",
	}))
	result := eval.EvaluateConfig(flagCfg, "", ctx)
	if !result.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result.Value.BoolValue() != false {
		t.Error("expected false for beta user")
	}

	ctx2 := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "regular@example.com",
	}))
	result2 := eval.EvaluateConfig(flagCfg, "", ctx2)
	if !result2.IsMatch {
		t.Fatal("expected IsMatch to be true")
	}
	if result2.Value.BoolValue() != true {
		t.Error("expected true for non-beta user")
	}
}

// --- Operator tests with inline configs ---

func TestEvaluateConfig_PropStartsWithOneOf(t *testing.T) {
	cfg := &Config{
		Key:       "starts-with-test",
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{
							PropertyName: "user.email",
							Operator:     OpPropStartsWithOneOf,
							ValueToMatch: &Value{
								Type:  ValueTypeStringList,
								Value: []string{"admin@", "root@"},
							},
						},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{{Operator: OpAlwaysTrue}},
					Value:    Value{Type: ValueTypeBool, Value: false},
				},
			},
		},
	}

	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "admin@example.com",
	}))
	result := eval.EvaluateConfig(cfg, "", ctx)
	if result.Value.BoolValue() != true {
		t.Error("expected true for admin@")
	}

	ctx2 := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "user@example.com",
	}))
	result2 := eval.EvaluateConfig(cfg, "", ctx2)
	if result2.Value.BoolValue() != false {
		t.Error("expected false for user@")
	}
}

func TestEvaluateConfig_PropEndsWithOneOf(t *testing.T) {
	cfg := &Config{
		Key:       "ends-with-test",
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{
							PropertyName: "user.email",
							Operator:     OpPropEndsWithOneOf,
							ValueToMatch: &Value{
								Type:  ValueTypeStringList,
								Value: []string{"@company.com", "@corp.com"},
							},
						},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{{Operator: OpAlwaysTrue}},
					Value:    Value{Type: ValueTypeBool, Value: false},
				},
			},
		},
	}

	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "alice@company.com",
	}))
	if eval.EvaluateConfig(cfg, "", ctx).Value.BoolValue() != true {
		t.Error("expected true for @company.com")
	}

	ctx2 := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "alice@gmail.com",
	}))
	if eval.EvaluateConfig(cfg, "", ctx2).Value.BoolValue() != false {
		t.Error("expected false for @gmail.com")
	}
}

func TestEvaluateConfig_PropContainsOneOf(t *testing.T) {
	cfg := &Config{
		Key:       "contains-test",
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{
							PropertyName: "user.name",
							Operator:     OpPropContainsOneOf,
							ValueToMatch: &Value{
								Type:  ValueTypeStringList,
								Value: []string{"admin", "root"},
							},
						},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{{Operator: OpAlwaysTrue}},
					Value:    Value{Type: ValueTypeBool, Value: false},
				},
			},
		},
	}

	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"name": "superadmin",
	}))
	if eval.EvaluateConfig(cfg, "", ctx).Value.BoolValue() != true {
		t.Error("expected true for superadmin")
	}

	ctx2 := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"name": "regular-user",
	}))
	if eval.EvaluateConfig(cfg, "", ctx2).Value.BoolValue() != false {
		t.Error("expected false for regular-user")
	}
}

func TestEvaluateConfig_PropMatches(t *testing.T) {
	cfg := &Config{
		Key:       "regex-test",
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{
							PropertyName: "user.email",
							Operator:     OpPropMatches,
							ValueToMatch: &Value{
								Type:  ValueTypeString,
								Value: `^[a-z]+@example\.com$`,
							},
						},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{{Operator: OpAlwaysTrue}},
					Value:    Value{Type: ValueTypeBool, Value: false},
				},
			},
		},
	}

	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "alice@example.com",
	}))
	if eval.EvaluateConfig(cfg, "", ctx).Value.BoolValue() != true {
		t.Error("expected true for alice@example.com")
	}

	ctx2 := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "ALICE@example.com",
	}))
	if eval.EvaluateConfig(cfg, "", ctx2).Value.BoolValue() != false {
		t.Error("expected false for ALICE@example.com")
	}
}

func TestEvaluateConfig_NumericComparisons(t *testing.T) {
	makeConfig := func(op string) *Config {
		return &Config{
			Key:       "numeric-test",
			Type:      ConfigTypeFeatureFlag,
			ValueType: ValueTypeBool,
			Default: RuleSet{
				Rules: []Rule{
					{
						Criteria: []Criterion{
							{
								PropertyName: "user.age",
								Operator:     op,
								ValueToMatch: &Value{
									Type:  ValueTypeInt,
									Value: int64(18),
								},
							},
						},
						Value: Value{Type: ValueTypeBool, Value: true},
					},
					{
						Criteria: []Criterion{{Operator: OpAlwaysTrue}},
						Value:    Value{Type: ValueTypeBool, Value: false},
					},
				},
			},
		}
	}

	store := newTestConfigStore()
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := func(age interface{}) ContextValueGetter {
		return NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
			"age": age,
		}))
	}

	tests := []struct {
		op   string
		age  interface{}
		want bool
	}{
		{OpPropGreaterThan, int64(20), true},
		{OpPropGreaterThan, int64(18), false},
		{OpPropGreaterThan, int64(10), false},
		{OpPropGreaterThanOrEqual, int64(18), true},
		{OpPropGreaterThanOrEqual, int64(17), false},
		{OpPropLessThan, int64(10), true},
		{OpPropLessThan, int64(18), false},
		{OpPropLessThan, int64(20), false},
		{OpPropLessThanOrEqual, int64(18), true},
		{OpPropLessThanOrEqual, int64(19), false},
		{OpPropGreaterThan, float64(18.5), true},
		{OpPropLessThan, float64(17.5), true},
	}

	for _, tt := range tests {
		t.Run(tt.op+"_"+ToString(tt.age), func(t *testing.T) {
			cfg := makeConfig(tt.op)
			store.addConfig(cfg)
			result := eval.EvaluateConfig(cfg, "", ctx(tt.age))
			if result.Value.BoolValue() != tt.want {
				t.Errorf("%s with age=%v: expected %v, got %v", tt.op, tt.age, tt.want, result.Value.BoolValue())
			}
		})
	}
}

func TestEvaluateConfig_InIntRange(t *testing.T) {
	cfg := &Config{
		Key:       "int-range-test",
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{
							PropertyName: "user.age",
							Operator:     OpInIntRange,
							ValueToMatch: &Value{
								Type: "int_range",
								Value: map[string]interface{}{
									"start": float64(18),
									"end":   float64(65),
								},
							},
						},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{{Operator: OpAlwaysTrue}},
					Value:    Value{Type: ValueTypeBool, Value: false},
				},
			},
		},
	}

	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	tests := []struct {
		age  interface{}
		want bool
	}{
		{int64(25), true},
		{int64(18), true},
		{int64(64), true},
		{int64(65), false},
		{int64(17), false},
		{float64(30.5), true},
	}

	for _, tt := range tests {
		ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
			"age": tt.age,
		}))
		result := eval.EvaluateConfig(cfg, "", ctx)
		if result.Value.BoolValue() != tt.want {
			t.Errorf("age=%v: expected %v, got %v", tt.age, tt.want, result.Value.BoolValue())
		}
	}
}

func TestEvaluateConfig_HierarchicalMatch(t *testing.T) {
	cfg := &Config{
		Key:       "hierarchical-test",
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{
							PropertyName: "user.team",
							Operator:     OpHierarchicalMatch,
							ValueToMatch: &Value{
								Type:  ValueTypeString,
								Value: "engineering.backend",
							},
						},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{{Operator: OpAlwaysTrue}},
					Value:    Value{Type: ValueTypeBool, Value: false},
				},
			},
		},
	}

	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	tests := []struct {
		team string
		want bool
	}{
		{"engineering.backend.api", true},
		{"engineering.backend", true},
		{"engineering.frontend", false},
		{"marketing", false},
	}

	for _, tt := range tests {
		ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
			"team": tt.team,
		}))
		result := eval.EvaluateConfig(cfg, "", ctx)
		if result.Value.BoolValue() != tt.want {
			t.Errorf("team=%s: expected %v, got %v", tt.team, tt.want, result.Value.BoolValue())
		}
	}
}

func TestEvaluateConfig_SemverOperators(t *testing.T) {
	makeConfig := func(op string, version string) *Config {
		return &Config{
			Key:       "semver-test",
			Type:      ConfigTypeFeatureFlag,
			ValueType: ValueTypeBool,
			Default: RuleSet{
				Rules: []Rule{
					{
						Criteria: []Criterion{
							{
								PropertyName: "device.version",
								Operator:     op,
								ValueToMatch: &Value{
									Type:  ValueTypeString,
									Value: version,
								},
							},
						},
						Value: Value{Type: ValueTypeBool, Value: true},
					},
					{
						Criteria: []Criterion{{Operator: OpAlwaysTrue}},
						Value:    Value{Type: ValueTypeBool, Value: false},
					},
				},
			},
		}
	}

	store := newTestConfigStore()
	eval := NewEvaluatorWithSeed(store, 42)

	tests := []struct {
		op      string
		match   string
		context string
		want    bool
	}{
		{OpPropSemverGreaterThan, "1.0.0", "2.0.0", true},
		{OpPropSemverGreaterThan, "1.0.0", "1.0.0", false},
		{OpPropSemverGreaterThan, "1.0.0", "0.9.0", false},
		{OpPropSemverEqual, "1.2.3", "1.2.3", true},
		{OpPropSemverEqual, "1.2.3", "1.2.4", false},
		{OpPropSemverLessThan, "2.0.0", "1.9.9", true},
		{OpPropSemverLessThan, "2.0.0", "2.0.0", false},
		{OpPropSemverLessThan, "2.0.0", "2.0.1", false},
		{OpPropSemverLessThan, "1.0.0", "1.0.0-alpha", true},
		{OpPropSemverGreaterThan, "1.0.0-alpha", "1.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.op+"_"+tt.match+"_vs_"+tt.context, func(t *testing.T) {
			cfg := makeConfig(tt.op, tt.match)
			store.addConfig(cfg)
			ctx := NewContextSet().WithNamedContext(NewNamedContext("device", map[string]interface{}{
				"version": tt.context,
			}))
			result := eval.EvaluateConfig(cfg, "", ctx)
			if result.Value.BoolValue() != tt.want {
				t.Errorf("expected %v, got %v", tt.want, result.Value.BoolValue())
			}
		})
	}
}

func TestEvaluateConfig_PropBefore_PropAfter(t *testing.T) {
	makeConfig := func(op string, millis int64) *Config {
		return &Config{
			Key:       "time-test",
			Type:      ConfigTypeFeatureFlag,
			ValueType: ValueTypeBool,
			Default: RuleSet{
				Rules: []Rule{
					{
						Criteria: []Criterion{
							{
								PropertyName: "event.timestamp",
								Operator:     op,
								ValueToMatch: &Value{
									Type:  ValueTypeInt,
									Value: millis,
								},
							},
						},
						Value: Value{Type: ValueTypeBool, Value: true},
					},
					{
						Criteria: []Criterion{{Operator: OpAlwaysTrue}},
						Value:    Value{Type: ValueTypeBool, Value: false},
					},
				},
			},
		}
	}

	store := newTestConfigStore()
	eval := NewEvaluatorWithSeed(store, 42)

	cfg := makeConfig(OpPropBefore, int64(1700000000000))
	store.addConfig(cfg)
	ctx := NewContextSet().WithNamedContext(NewNamedContext("event", map[string]interface{}{
		"timestamp": int64(1699999999000),
	}))
	if eval.EvaluateConfig(cfg, "", ctx).Value.BoolValue() != true {
		t.Error("expected true for PROP_BEFORE")
	}

	ctx2 := NewContextSet().WithNamedContext(NewNamedContext("event", map[string]interface{}{
		"timestamp": int64(1700000001000),
	}))
	if eval.EvaluateConfig(cfg, "", ctx2).Value.BoolValue() != false {
		t.Error("expected false for PROP_BEFORE with later time")
	}

	cfg2 := makeConfig(OpPropAfter, int64(1700000000000))
	store.addConfig(cfg2)
	if eval.EvaluateConfig(cfg2, "", ctx).Value.BoolValue() != false {
		t.Error("expected false for PROP_AFTER with earlier time")
	}
	if eval.EvaluateConfig(cfg2, "", ctx2).Value.BoolValue() != true {
		t.Error("expected true for PROP_AFTER")
	}
}

func TestEvaluateConfig_MultipleCriteria_AND(t *testing.T) {
	cfg := &Config{
		Key:       "and-test",
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{
							PropertyName: "user.country",
							Operator:     OpPropIsOneOf,
							ValueToMatch: &Value{
								Type:  ValueTypeStringList,
								Value: []string{"US", "CA"},
							},
						},
						{
							PropertyName: "user.age",
							Operator:     OpPropGreaterThanOrEqual,
							ValueToMatch: &Value{
								Type:  ValueTypeInt,
								Value: int64(21),
							},
						},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{{Operator: OpAlwaysTrue}},
					Value:    Value{Type: ValueTypeBool, Value: false},
				},
			},
		},
	}

	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"country": "US",
		"age":     int64(25),
	}))
	if eval.EvaluateConfig(cfg, "", ctx).Value.BoolValue() != true {
		t.Error("expected true when both criteria match")
	}

	ctx2 := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"country": "US",
		"age":     int64(18),
	}))
	if eval.EvaluateConfig(cfg, "", ctx2).Value.BoolValue() != false {
		t.Error("expected false when age doesn't match")
	}

	ctx3 := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"country": "UK",
		"age":     int64(25),
	}))
	if eval.EvaluateConfig(cfg, "", ctx3).Value.BoolValue() != false {
		t.Error("expected false when country doesn't match")
	}
}

func TestEvaluateConfig_NegatedOperators(t *testing.T) {
	cfg := &Config{
		Key:       "negated-test",
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{
							PropertyName: "user.email",
							Operator:     OpPropIsNotOneOf,
							ValueToMatch: &Value{
								Type:  ValueTypeStringList,
								Value: []string{"blocked@evil.com"},
							},
						},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{{Operator: OpAlwaysTrue}},
					Value:    Value{Type: ValueTypeBool, Value: false},
				},
			},
		},
	}

	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "good@example.com",
	}))
	if eval.EvaluateConfig(cfg, "", ctx).Value.BoolValue() != true {
		t.Error("expected true for non-blocked user")
	}

	ctx2 := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "blocked@evil.com",
	}))
	if eval.EvaluateConfig(cfg, "", ctx2).Value.BoolValue() != false {
		t.Error("expected false for blocked user")
	}

	if eval.EvaluateConfig(cfg, "", nil).Value.BoolValue() != true {
		t.Error("expected true with no context (negated default)")
	}
}

func TestEvaluateConfig_PropDoesNotMatch(t *testing.T) {
	cfg := &Config{
		Key:       "not-match-test",
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{
							PropertyName: "user.email",
							Operator:     OpPropDoesNotMatch,
							ValueToMatch: &Value{
								Type:  ValueTypeString,
								Value: `.*@spam\.com$`,
							},
						},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{{Operator: OpAlwaysTrue}},
					Value:    Value{Type: ValueTypeBool, Value: false},
				},
			},
		},
	}

	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "alice@example.com",
	}))
	if eval.EvaluateConfig(cfg, "", ctx).Value.BoolValue() != true {
		t.Error("expected true for non-spam")
	}

	ctx2 := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "spammer@spam.com",
	}))
	if eval.EvaluateConfig(cfg, "", ctx2).Value.BoolValue() != false {
		t.Error("expected false for spam")
	}
}

func TestEvaluateConfig_NoRulesMatch(t *testing.T) {
	cfg := &Config{
		Key:       "no-match-test",
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeBool,
		Default: RuleSet{
			Rules: []Rule{
				{
					Criteria: []Criterion{
						{Operator: OpNotSet},
					},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
			},
		},
	}

	store := newTestConfigStore()
	store.addConfig(cfg)
	eval := NewEvaluatorWithSeed(store, 42)

	result := eval.EvaluateConfig(cfg, "", nil)
	if result.IsMatch {
		t.Error("expected IsMatch to be false")
	}
}
