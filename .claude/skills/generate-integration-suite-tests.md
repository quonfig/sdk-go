# generate-integration-suite-tests

Generate static Go integration tests from the shared cross-SDK test suite.

## Context

Quonfig has a shared integration test suite in `integration-test-data/` that defines:
- **Test data**: JSON config/flag/segment files in `integration-test-data/data/integration-tests/`
- **Test definitions**: YAML files in `integration-test-data/tests/eval/` specifying inputs, contexts, and expected outputs

Every SDK must pass the same tests to guarantee behavioral consistency across 10+ languages. This skill generates **static, idiomatic Go test functions** from the YAML definitions — no YAML parsing at runtime.

## What to generate

For each YAML file in `integration-test-data/tests/eval/`, generate a corresponding Go test file in `sdk-go/internal/fixtures/`. For example, `get.yaml` → `get_generated_test.go`.

Each test case in the YAML becomes its own Go test function with explicit assertions. The test names come from the YAML test case names (no numbering needed — names are the cross-SDK identifier).

### Generated test structure

```go
package fixtures

import "testing"

// Code generated from integration-test-data/tests/eval/get.yaml. DO NOT EDIT.

func TestGet_ReturnsFoundValueForKey(t *testing.T) {
    cfg, ok := configStore.GetConfig("my-test-key")
    if !ok {
        t.Fatal("config not found: my-test-key")
    }
    ctx := buildContextFromMaps(nil, nil, nil)
    match := evaluator.EvaluateConfig(cfg, "Production", ctx)
    resolved := mustResolve(t, match, cfg, ctx)
    assertStringValue(t, resolved, "my-test-value")
}
```

### Shared test helpers

The generated tests should use shared helpers defined in a non-generated file (`test_helpers.go` or similar). These helpers handle:
- `buildContextFromMaps(global, block, local map[string]map[string]interface{})` — builds a merged context
- `mustResolve(t, match, cfg, ctx)` — resolves the matched value, calls t.Fatal on error
- `assertStringValue(t, value, expected)`, `assertIntValue(...)`, `assertBoolValue(...)`, `assertFloatValue(...)`, `assertStringListValue(...)`, `assertJSONValue(...)`, `assertDurationMillis(...)` — type-specific assertions
- `assertRaises(t, err, expectedError)` — for get_or_raise error cases

### Test data loading

Keep the existing `TestMain` in a non-generated file that:
1. Loads all JSON configs from `integration-test-data/data/integration-tests/` using `loader.go`
2. Creates the shared `configStore`, `evaluator`, and `testResolver`
3. Sets environment variables (PREFAB_INTEGRATION_TEST_ENCRYPTION_KEY, IS_A_NUMBER, NOT_A_NUMBER)

The generated tests use these package-level variables — no data loading in the generated code.

## Step-by-step

1. **Read all YAML files** in `integration-test-data/tests/eval/`.

2. **Read the existing test infrastructure** in `sdk-go/internal/fixtures/` to understand what helpers exist and what the package structure looks like.

3. **For each YAML file**, generate a `<name>_generated_test.go` file containing one test function per test case. Use the YAML test case `name` field to derive the Go function name (sanitized to valid Go identifier).

4. **Each generated test function should**:
   - Look up the config by key (from `input.key` or `input.flag`)
   - Build context from the three-level hierarchy (global/block/local) if specified
   - Call the evaluator
   - Resolve the value
   - Assert the expected result based on function type and expected fields

5. **Handle special cases in the generated code**:
   - `get_or_raise` with `expected.status: raise` → expect an error from resolve
   - `initialization_timeout` → generate `t.Skip("requires network timing")`
   - Duration → assert milliseconds
   - JSON → deep compare
   - Missing config with default → assert default value
   - `client_overrides.on_no_default: 2` → handle nil/not-found gracefully
   - Configs with `prefab-api-key.*` criteria → generate skip logic if test doesn't provide that context

6. **Create/update shared helpers** if they don't exist yet.

7. **Run tests**: `cd sdk-go && go test ./internal/fixtures/ -v -count=1`

8. **Add `// Code generated ... DO NOT EDIT.` header** to all generated files so they're clearly machine-generated.

## YAML format reference

```yaml
function: get|enabled|get_or_raise|get_feature_flag|get_weighted_values
tests:
  - name: optional group name
    cases:
      - name: test case description    # THIS IS THE CROSS-SDK IDENTIFIER
        client: config_client|feature_flag_client|client
        function: get|enabled|get_or_raise
        type: STRING|INT|DOUBLE|BOOLEAN|STRING_LIST|JSON|DURATION
        input:
          key: "config-key"
          flag: "flag-key"
          default: <value>
        contexts:
          global: { contextType: { prop: value } }
          block:  { contextType: { prop: value } }
          local:  { contextType: { prop: value } }
        expected:
          value: <expected>
          millis: <number>
          status: raise
          error: <error_type>
          message: <string>
        client_overrides:
          on_no_default: 2
```

## Paths (relative to repo root)

- YAML test definitions: `integration-test-data/tests/eval/*.yaml`
- Test data (loaded at runtime): `integration-test-data/data/integration-tests/`
- Generated output: `sdk-go/internal/fixtures/*_generated_test.go`
- Shared helpers: `sdk-go/internal/fixtures/test_helpers.go` (or similar, non-generated)
- Data loader: `sdk-go/internal/fixtures/loader.go` (existing, non-generated)
- Reference: `sdk-go/internal/fixtures/runner_test.go` (existing dynamic runner — can be removed once generated tests pass)
