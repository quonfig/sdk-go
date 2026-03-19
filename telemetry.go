package quonfig

import (
	"crypto/rand"
	"fmt"

	"github.com/quonfig/sdk-go/internal/telemetry"
)

// telemetrySubmitter wraps the internal telemetry.Submitter and
// provides the bridge between quonfig types and telemetry types.
type telemetrySubmitter struct {
	submitter *telemetry.Submitter
}

func newTelemetrySubmitter(opts Options) *telemetrySubmitter {
	cfg := telemetry.Config{
		APIKey:                     opts.APIKey,
		TelemetryURL:               opts.TelemetryURL,
		SyncInterval:               opts.TelemetrySyncInterval,
		CollectEvaluationSummaries: opts.CollectEvaluationSummaries,
		ContextTelemetryMode:       string(opts.ContextTelemetryMode),
		InstanceHash:               generateInstanceHash(),
	}
	if opts.HTTPClient != nil {
		cfg.HTTPClient = opts.HTTPClient
	}
	return &telemetrySubmitter{
		submitter: telemetry.NewSubmitter(cfg),
	}
}

func (t *telemetrySubmitter) Start() {
	t.submitter.Start()
}

func (t *telemetrySubmitter) Stop() {
	t.submitter.Stop()
}

// RecordEvaluation converts an EvalResult to telemetry.EvalMatch and records it.
func (t *telemetrySubmitter) RecordEvaluation(result *EvalResult) {
	if result == nil || !result.IsMatch {
		return
	}

	var selectedValue interface{}
	if result.Value != nil {
		selectedValue = result.Value.Value
	}

	t.submitter.RecordEvaluation(telemetry.EvalMatch{
		ConfigID:           result.ConfigID,
		ConfigKey:          result.ConfigKey,
		ConfigType:         string(result.ConfigType),
		RuleIndex:          result.RuleIndex,
		WeightedValueIndex: result.WeightedValueIndex,
		SelectedValue:      selectedValue,
		Reason:             int(result.Reason),
	})
}

// RecordContext converts a ContextSet to telemetry.ContextData and records it.
func (t *telemetrySubmitter) RecordContext(ctx *ContextSet) {
	if ctx == nil {
		return
	}

	ctxData := contextSetToTelemetryData(ctx)
	t.submitter.RecordContext(ctxData)
}

// generateInstanceHash creates a random UUID v4 string without external dependencies.
func generateInstanceHash() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// contextSetToTelemetryData converts a ContextSet to the telemetry package's ContextData.
func contextSetToTelemetryData(ctx *ContextSet) telemetry.ContextData {
	contexts := make(map[string]map[string]interface{}, len(ctx.data))
	for name, nc := range ctx.data {
		props := make(map[string]interface{}, len(nc.Data))
		for k, v := range nc.Data {
			props[k] = v
		}
		contexts[name] = props
	}
	return telemetry.ContextData{Contexts: contexts}
}
