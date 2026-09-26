package quonfig

import (
	"sync"
	"testing"
)

// TestGetBoolValue_ConcurrentWeightedRolloutWithoutHashKey is the public-API
// reproduction of qfg-9dxb.1 (audit C1): a percentage rollout evaluated
// concurrently for contexts that lack the hash property must not race or
// panic the process.
func TestGetBoolValue_ConcurrentWeightedRolloutWithoutHashKey(t *testing.T) {
	client := detailsClient(t, []ConfigResponse{
		{
			ID:        "cfg-rollout",
			Key:       "rollout",
			Type:      ConfigTypeFeatureFlag,
			ValueType: ValueTypeBool,
			Default: RuleSet{Rules: []Rule{
				{
					Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
					Value: Value{
						Type: ValueTypeWeightedValues,
						Value: &WeightedValuesData{
							HashByPropertyName: "user.key",
							WeightedValues: []WeightedValue{
								{Weight: 50, Value: Value{Type: ValueTypeBool, Value: true}},
								{Weight: 50, Value: Value{Type: ValueTypeBool, Value: false}},
							},
						},
					},
				},
			}},
		},
	}, nil)

	const goroutines = 16
	const iterations = 5000
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if _, ok, err := client.GetBoolValue("rollout", nil); err != nil || !ok {
					t.Errorf("GetBoolValue: ok=%v err=%v", ok, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
