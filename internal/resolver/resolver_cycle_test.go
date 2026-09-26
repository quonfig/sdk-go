package resolver

import (
	"errors"
	"runtime/debug"
	"testing"

	quonfig "github.com/quonfig/sdk-go"
	"github.com/quonfig/sdk-go/internal/eval"
)

type mapStore map[string]*eval.FullConfig

func (m mapStore) GetConfig(key string) (*eval.FullConfig, bool) {
	c, ok := m[key]
	return c, ok
}

func confidential(key, decryptWith string) *eval.FullConfig {
	return &eval.FullConfig{
		Key:       key,
		ValueType: quonfig.ValueTypeString,
		Default: quonfig.RuleSet{Rules: []quonfig.Rule{{Value: quonfig.Value{
			Type: quonfig.ValueTypeString, Value: "AA--00112233445566778899AABB--BB",
			Confidential: true, DecryptWith: decryptWith,
		}}}},
	}
}

// Mirrors the runtime resolver guard (qfg-9dxb.4): a decryptWith cycle must
// return ErrUnableToDecrypt rather than overflow the stack.
func TestResolve_DecryptWithCycleReturnsError(t *testing.T) {
	defer debug.SetMaxStack(debug.SetMaxStack(64 << 20))

	cases := map[string]mapStore{
		"self":    {"secret": confidential("secret", "k"), "k": confidential("k", "k")},
		"A->B->A": {"secret": confidential("secret", "a"), "a": confidential("a", "b"), "b": confidential("b", "a")},
	}
	for name, store := range cases {
		t.Run(name, func(t *testing.T) {
			ev := eval.NewEvaluator(store)
			r := New(store, ev, nil)
			cfg := store["secret"]
			_, err := r.Resolve(&cfg.Default.Rules[0].Value, cfg, "", nil)
			if !errors.Is(err, ErrUnableToDecrypt) {
				t.Fatalf("expected ErrUnableToDecrypt, got %v", err)
			}
		})
	}
}
