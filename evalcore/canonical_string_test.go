package evalcore

import (
	"encoding/json"
	"testing"
)

// R1 (qfg-goi1.5.2, plans/2026-10-07-sdk-operator-semantics.md): string
// operators compare against a canonical string form. Every number that
// reaches the SDK through JSON (all browser/mobile contexts via api-delivery)
// is a float64, so an integral float64 must render as plain digits, never
// %v's exponent form ("1.234567e+06") or "1.0". The ITD corpus cannot carry
// whole-number floats (generators read YAML as JS numbers), so this is
// pinned here on a json.Unmarshal-decoded context.
func TestToStringCanonicalFormOnJSONDecodedContext(t *testing.T) {
	var ctx map[string]interface{}
	raw := `{"id":1234567,"big":5e9,"one":1.0,"half":1.5,"neg":-42.0,"zero":0,
		"limit":9007199254740991,"t":true,"f":false,"s":"abc",
		"list":[1234567,5e9,1.0,1.5,true,"x"]}`
	if err := json.Unmarshal([]byte(raw), &ctx); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"id":    "1234567",
		"big":   "5000000000",
		"one":   "1",
		"half":  "1.5",
		"neg":   "-42",
		"zero":  "0",
		"limit": "9007199254740991",
		"t":     "true",
		"f":     "false",
		"s":     "abc",
	}
	for k, want := range cases {
		if _, ok := ctx[k].(float64); !ok && k != "t" && k != "f" && k != "s" {
			t.Fatalf("%s: decoded as %T, want float64", k, ctx[k])
		}
		if got := ToString(ctx[k]); got != want {
			t.Errorf("ToString(%s=%#v) = %q, want %q", k, ctx[k], got, want)
		}
	}

	wantList := []string{"1234567", "5000000000", "1", "1.5", "true", "x"}
	gotList := toStringSlice(ctx["list"])
	if len(gotList) != len(wantList) {
		t.Fatalf("toStringSlice(list) = %q, want %q", gotList, wantList)
	}
	for i := range wantList {
		if gotList[i] != wantList[i] {
			t.Errorf("toStringSlice(list)[%d] = %q, want %q", i, gotList[i], wantList[i])
		}
	}
}

// End to end: is-one-of ["1234567"] matches a numeric id decoded from JSON.
func TestIsOneOfMatchesJSONDecodedIntegralFloat(t *testing.T) {
	var ctx map[string]interface{}
	if err := json.Unmarshal([]byte(`{"n":1234567,"b":5e9}`), &ctx); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key, want string
	}{{"n", "1234567"}, {"b", "5000000000"}} {
		got := toStringSlice(ctx[tc.key])
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("%s: toStringSlice = %q, want [%q]", tc.key, got, tc.want)
		}
	}
}

// Q5 (qfg-goi1.5, decided 2026-10-09/12): abbreviated versions "x" and "x.y"
// read as x.0.0 and x.y.0, matching LaunchDarkly and Statsig and the cli
// verify grammar. A "v" prefix stays invalid, and prerelease/build metadata
// still require the full x.y.z form.
func TestParseSemverAbbreviated(t *testing.T) {
	valid := map[string]string{
		"2":            "2.0.0",
		"2.0":          "2.0.0",
		"4.1":          "4.1.0",
		"0":            "0.0.0",
		"2.0.0":        "2.0.0",
		"1.0.0-beta.1": "1.0.0-beta.1",
		"1.0.0+b.5":    "1.0.0+b.5",
	}
	for in, want := range valid {
		sv, err := ParseSemver(in)
		if err != nil {
			t.Errorf("ParseSemver(%q) error: %v", in, err)
			continue
		}
		if sv.String() != want {
			t.Errorf("ParseSemver(%q) = %s, want %s", in, sv, want)
		}
	}
	for _, in := range []string{"v2.0.0", "v2", "2.", "2.0.", "02", "2.01", "2-beta", "2.0-beta", "2+b", "2.0.0.0", "", "x"} {
		if sv, err := ParseSemver(in); err == nil {
			t.Errorf("ParseSemver(%q) = %s, want error", in, sv)
		}
	}
	two := ParseSemverQuietly("2")
	full := ParseSemverQuietly("2.0.0")
	if two == nil || full == nil || two.Compare(*full) != 0 {
		t.Errorf(`"2" should compare equal to "2.0.0"`)
	}
}
