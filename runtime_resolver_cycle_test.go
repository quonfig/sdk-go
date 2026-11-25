package quonfig

import (
	"errors"
	"runtime/debug"
	"testing"
)

func confidentialConfig(key, decryptWith, ciphertext string) *ConfigResponse {
	return &ConfigResponse{
		Key:       key,
		ValueType: ValueTypeString,
		Default: RuleSet{Rules: []Rule{{
			Value: Value{Type: ValueTypeString, Value: ciphertext, Confidential: true, DecryptWith: decryptWith},
		}}},
	}
}

// Rule R1 (qf1 spec 13.3): this SDK has no decryptWith code, so every value
// that carries decryptWith fails with ErrUnableToDecrypt on its own, whatever
// the key config it names looks like: a self-reference, an A->B->A loop
// (which once overflowed the stack, qfg-9dxb.4) or a key config holding an
// old-format key. None of them is resolved or followed.
func TestGetStringValue_DecryptWithValuesFailWithoutFollowingKeyConfigs(t *testing.T) {
	defer debug.SetMaxStack(debug.SetMaxStack(64 << 20))

	const ct = "AA--00112233445566778899AABB--BB"
	cases := map[string]map[string]*ConfigResponse{
		"key decrypts with itself": {
			"secret": confidentialConfig("secret", "k", ct),
			"k":      confidentialConfig("k", "k", ct),
		},
		"key cycle A->B->A": {
			"secret": confidentialConfig("secret", "a", ct),
			"a":      confidentialConfig("a", "b", ct),
			"b":      confidentialConfig("b", "a", ct),
		},
		"value decrypts with itself": {
			"secret": confidentialConfig("secret", "secret", ct),
		},
		"old-format key config present": {
			"secret": confidentialConfig("secret", "k", ct),
			"k": {
				Key: "k", ValueType: ValueTypeString,
				Default: RuleSet{Rules: []Rule{{Value: Value{Type: ValueTypeString,
					Value: "e657e0406fc22e17d3145966396b2130d33dcb30ac0edd62a77235cdd01fc49d"}}}},
			},
		},
	}
	for name, configs := range cases {
		t.Run(name, func(t *testing.T) {
			client, err := NewClient()
			if err != nil {
				t.Fatal(err)
			}
			env := &ConfigEnvelope{}
			for _, cfg := range configs {
				env.Configs = append(env.Configs, *cfg)
			}
			client.installEnvelope(env, -1)
			_, _, err = client.GetStringValue("secret", nil)
			if !errors.Is(err, ErrUnableToDecrypt) {
				t.Fatalf("expected ErrUnableToDecrypt, got %v", err)
			}
		})
	}
}
