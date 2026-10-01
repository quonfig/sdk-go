package quonfig

import (
	"fmt"
	"testing"
)

// Pins the documented context-merge rule (qfg-2agi.24, qfg-2agi.38): a newer
// tier's named context REPLACES the whole same-named context from an older
// tier, and named contexts the newer tier does not mention survive. Each test
// uses disjoint attributes inside the same named context so it would fail
// under a property-by-property merge, which the older same-key override
// tests cannot tell apart.

// mergeRuleFlag is a bool flag that is on when propertyName is one of want.
func mergeRuleFlag(key, propertyName, want string) string {
	return fmt.Sprintf(`{
		"id":"cfg-%[1]s","key":"%[1]s","type":"feature_flag","valueType":"bool","sendToClientSdk":false,
		"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":false}}]},
		"environments":[{"id":"Production","rules":[
			{"criteria":[{"propertyName":"%[2]s","operator":"PROP_IS_ONE_OF","valueToMatch":{"type":"string_list","value":["%[3]s"]}}],"value":{"type":"bool","value":true}},
			{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":false}}
		]}]
	}`, key, propertyName, want)
}

func mergeRuleWorkspace(t *testing.T) string {
	t.Helper()
	return writeWorkspaceFiles(t, `{"prod":"Production"}`, map[string]string{
		"feature-flags/m.user-email.json": mergeRuleFlag("m.user-email", "user.email", "a@example.org"),
		"feature-flags/m.user-plan.json":  mergeRuleFlag("m.user-plan", "user.plan", "pro"),
		"feature-flags/m.user-tier.json":  mergeRuleFlag("m.user-tier", "user.tier", "gold"),
		"feature-flags/m.team-key.json":   mergeRuleFlag("m.team-key", "team.key", "t1"),
		"feature-flags/m.qu-email.json":   mergeRuleFlag("m.qu-email", "quonfig-user.email", "bob@foo.com"),
		"feature-flags/m.qu-plan.json":    mergeRuleFlag("m.qu-plan", "quonfig-user.plan", "pro"),
	})
}

func mergeRuleClient(t *testing.T, dir string, opts ...Option) *Client {
	t.Helper()
	base := []Option{WithDataDir(dir), WithEnvironment("Production"), WithAllTelemetryDisabled()}
	c, err := NewClient(append(base, opts...)...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

func mergeRuleGlobal() *ContextSet {
	return NewContextSet().
		WithNamedContextValues("user", map[string]interface{}{"email": "a@example.org"}).
		WithNamedContextValues("team", map[string]interface{}{"key": "t1"})
}

func userCtx(k, v string) *ContextSet {
	return NewContextSet().WithNamedContextValues("user", map[string]interface{}{k: v})
}

// assertFlags checks each flag key against its expected on/off value.
func assertFlags(t *testing.T, label string, get func(string) (bool, bool), want map[string]bool) {
	t.Helper()
	for key, wantOn := range want {
		on, found := get(key)
		if !found {
			t.Errorf("%s: flag %s not found", label, key)
			continue
		}
		if on != wantOn {
			t.Errorf("%s: %s = %v, want %v", label, key, on, wantOn)
		}
	}
}

func TestContextMergeRule_GlobalPlusPerCall(t *testing.T) {
	c := mergeRuleClient(t, mergeRuleWorkspace(t), WithQuonfigUserContext(false), WithGlobalContext(mergeRuleGlobal()))

	get := func(k string) (bool, bool) { return c.FeatureIsOn(k, userCtx("plan", "pro")) }
	assertFlags(t, "global{user.email,team.key} + per-call{user.plan}", get, map[string]bool{
		"m.user-plan":  true,  // per-call user wins
		"m.user-email": false, // global user replaced whole, email gone
		"m.team-key":   true,  // team not mentioned per-call, survives
	})
}

func TestContextMergeRule_GlobalPlusBound(t *testing.T) {
	c := mergeRuleClient(t, mergeRuleWorkspace(t), WithQuonfigUserContext(false), WithGlobalContext(mergeRuleGlobal()))

	bound := c.WithContext(userCtx("plan", "pro"))
	assertFlags(t, "global{user.email,team.key} + WithContext{user.plan}", bound.FeatureIsOn, map[string]bool{
		"m.user-plan":  true,
		"m.user-email": false,
		"m.team-key":   true,
	})
}

func TestContextMergeRule_NestedBound(t *testing.T) {
	c := mergeRuleClient(t, mergeRuleWorkspace(t), WithQuonfigUserContext(false))

	outer := c.WithContext(mergeRuleGlobal())
	inner := outer.WithContext(userCtx("plan", "pro"))
	assertFlags(t, "WithContext{user.email,team.key} -> WithContext{user.plan}", inner.FeatureIsOn, map[string]bool{
		"m.user-plan":  true,
		"m.user-email": false,
		"m.team-key":   true,
	})

	innermost := inner.WithContext(userCtx("tier", "gold"))
	assertFlags(t, "... -> WithContext{user.tier}", innermost.FeatureIsOn, map[string]bool{
		"m.user-tier":  true,
		"m.user-plan":  false,
		"m.user-email": false,
		"m.team-key":   true,
	})

	// The outer bound client is unchanged by deriving inner ones.
	assertFlags(t, "outer after nesting", outer.FeatureIsOn, map[string]bool{
		"m.user-email": true,
		"m.user-plan":  false,
		"m.team-key":   true,
	})
}

// Dev context (quonfig.go NewClient): the injected quonfig-user context is
// replaced whole by a customer GlobalContext that names quonfig-user.
func TestContextMergeRule_DevContextReplacedByCustomerQuonfigUser(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"userEmail":"bob@foo.com"}`)
	t.Setenv("QUONFIG_DEV_CONTEXT", "")

	customer := NewContextSet().WithNamedContextValues("quonfig-user", map[string]interface{}{"plan": "pro"})
	c := mergeRuleClient(t, mergeRuleWorkspace(t), WithQuonfigUserContext(true), WithGlobalContext(customer))

	get := func(k string) (bool, bool) { return c.FeatureIsOn(k, nil) }
	assertFlags(t, "dev{quonfig-user.email} + customer global{quonfig-user.plan}", get, map[string]bool{
		"m.qu-plan":  true,
		"m.qu-email": false,
	})
}

// Dev context survives a customer GlobalContext that does not name quonfig-user.
func TestContextMergeRule_DevContextSurvivesUnrelatedGlobal(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"userEmail":"bob@foo.com"}`)
	t.Setenv("QUONFIG_DEV_CONTEXT", "")

	c := mergeRuleClient(t, mergeRuleWorkspace(t), WithQuonfigUserContext(true), WithGlobalContext(mergeRuleGlobal()))

	get := func(k string) (bool, bool) { return c.FeatureIsOn(k, nil) }
	assertFlags(t, "dev{quonfig-user.email} + customer global{user,team}", get, map[string]bool{
		"m.qu-email":   true,
		"m.user-email": true,
		"m.team-key":   true,
	})
}
