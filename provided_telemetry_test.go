package quonfig

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

// ENV_VAR-provided values in evaluation-summary telemetry (qfg-goi1.10.16).
// Jeff's rule (2026-10-09): redact a provided value only when it is marked
// confidential. A confidential provided value reports a redaction over the
// stored (pre-resolution) descriptor and never the env contents; a
// non-confidential one reports its resolved value like any ordinary value.
// Mirrors sdk-node fe09070 (test/provided-telemetry-redaction.test.ts).

const providedTelemetrySecret = "env-contents-7f3a"

func providedTelemetryBodies(t *testing.T, confidential bool) []string {
	t.Helper()
	t.Setenv("QUONFIG_BACKEND_SDK_KEY", "")
	t.Setenv("QF_PROVIDED_TELEMETRY", providedTelemetrySecret)
	rec := newTelemetryPostRecorder(t)

	conf := "false"
	if confidential {
		conf = "true"
	}
	dir := writeWorkspace(t,
		`{"prod":"Production"}`,
		"configs/provided.cfg.json",
		`{
			"id":"cfg-provided",
			"key":"provided.cfg",
			"type":"config",
			"valueType":"string",
			"sendToClientSdk":false,
			"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"provided","confidential":`+conf+`,"value":{"source":"ENV_VAR","lookup":"QF_PROVIDED_TELEMETRY"}}}]},
			"environments":[]
		}`,
	)

	client, err := NewClient(
		WithSdkKey("test-backend-key"),
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithTelemetryURL(rec.server.URL),
		WithTelemetrySyncInterval(time.Minute),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	got, ok, err := client.GetStringValue("provided.cfg", nil)
	if err != nil || !ok || got != providedTelemetrySecret {
		t.Fatalf("GetStringValue = (%q, %v, %v), want (%q, true, nil)", got, ok, err, providedTelemetrySecret)
	}
	settleTelemetryQueue()
	client.Close()

	posts, _ := rec.snapshot()
	var withKey []string
	for _, b := range posts {
		if strings.Contains(b, "provided.cfg") {
			withKey = append(withKey, b)
		}
	}
	if len(withKey) == 0 {
		t.Fatalf("no telemetry POST carried provided.cfg; bodies: %v", posts)
	}
	return withKey
}

func TestProvidedTelemetry_ConfidentialIsRedacted(t *testing.T) {
	bodies := providedTelemetryBodies(t, true)
	joined := strings.Join(bodies, "\n")
	if strings.Contains(joined, providedTelemetrySecret) {
		t.Fatalf("confidential provided value leaked env contents into telemetry: %s", joined)
	}
	if !strings.Contains(joined, reportableValuePrefix) {
		t.Fatalf("confidential provided value not redacted; bodies: %s", joined)
	}
}

func TestProvidedTelemetry_NonConfidentialReportsResolvedValue(t *testing.T) {
	bodies := providedTelemetryBodies(t, false)
	joined := strings.Join(bodies, "\n")
	if !strings.Contains(joined, `"string":"`+providedTelemetrySecret+`"`) {
		t.Fatalf("non-confidential provided value: want resolved value %q as selectedValue; bodies: %s", providedTelemetrySecret, joined)
	}
	if strings.Contains(joined, "QF_PROVIDED_TELEMETRY") {
		t.Fatalf("non-confidential provided value sent the descriptor instead of the resolved value: %s", joined)
	}
}

// The redaction hash is over the stored descriptor, not the env contents.
func TestProvidedTelemetry_RedactionNotOverEnvContents(t *testing.T) {
	sum := md5.Sum([]byte(providedTelemetrySecret))
	overSecret := reportableValuePrefix + hex.EncodeToString(sum[:])[:5]
	joined := strings.Join(providedTelemetryBodies(t, true), "\n")
	if strings.Contains(joined, overSecret) {
		t.Fatalf("redaction hashed the env contents (%s); want a hash over the stored descriptor", overSecret)
	}
}
