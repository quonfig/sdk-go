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

// A decryptWith chain that loops back on itself (a key config that is itself
// confidential and decrypted with itself, or A->B->A) must surface
// ErrUnableToDecrypt, not recurse until an unrecoverable stack overflow
// (qfg-9dxb.4). Same for ciphertext too short to hold a GCM tag.
func TestGetStringValue_MalformedDecryptionDoesNotCrash(t *testing.T) {
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
		"short ciphertext": {
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
