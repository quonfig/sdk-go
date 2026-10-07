package quonfig

import (
	"crypto/md5"
	"crypto/rand"
	"encoding"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/quonfig/sdk-go/internal/telemetry"
)

// reportableValuePrefix is the redaction prefix for confidential values in
// telemetry payloads. The full form is "*****<first-5-hex-chars-of-md5(raw)>".
const reportableValuePrefix = "*****"

// reportableValueFor returns the redacted form of a value for telemetry
// reporting, or nil if the value is not confidential and does not require
// decryption. The hash is computed over the raw stored string value (the
// ciphertext for decryptWith values, the plaintext for plain confidential
// values) -- not over the decrypted plaintext.
func reportableValueFor(val *Value) *string {
	if val == nil {
		return nil
	}
	if !val.Confidential && val.DecryptWith == "" {
		return nil
	}
	raw := val.StringValue()
	sum := md5.Sum([]byte(raw))
	h := hex.EncodeToString(sum[:])
	if len(h) < 5 {
		return nil
	}
	out := reportableValuePrefix + h[:5]
	return &out
}

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
		Logger:                     opts.Logger,
		Timeout:                    opts.TelemetryTimeout,
		ConnectTimeout:             opts.TelemetryConnectTimeout,
		MaxRetainedBatches:         opts.TelemetryMaxRetainedBatches,
		MaxRetainedBytes:           opts.TelemetryMaxRetainedBytes,
		MaxRetainedAge:             opts.TelemetryMaxRetainedAge,
		MaxEvaluationSummaries:     opts.TelemetryMaxEvaluationSummaries,
		MaxContextShapeFields:      opts.TelemetryMaxContextShapeFields,
		MaxExampleContexts:         opts.TelemetryMaxExampleContexts,
		MaxExampleContextsSeen:     opts.TelemetryMaxExampleContextsSeen,
		Clock:                      opts.testTelemetryClock,
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
		ReportableValue:    reportableValueFor(result.Value),
		Reason:             int(result.Reason),
	})
}

// RecordContext converts a ContextSet to telemetry.ContextData and records it.
func (t *telemetrySubmitter) RecordContext(ctx *ContextSet) {
	if ctx == nil {
		return
	}

	// Convert only as much as the context telemetry mode uses. With example
	// contexts off, nothing renders a context value: v1.5.0 never ran a
	// value's MarshalText/MarshalJSON in those modes, and this runs on every
	// evaluation.
	switch {
	case t.submitter.ExampleContextsEnabled():
		t.submitter.RecordContext(contextSetToTelemetryData(ctx))
	case t.submitter.ContextShapesEnabled():
		t.submitter.RecordContext(contextSetToTelemetryShapes(ctx))
	}
}

// RecordHedgeFired records one config-fetch cycle whose hedge fired the
// secondary leg (failover observability).
func (t *telemetrySubmitter) RecordHedgeFired() {
	t.submitter.RecordHedgeFired()
}

// RecordGuardRejected records one install dropped by the reject-older ordering
// guard because the payload was strictly older than the held generation
// (failover observability; an equal-generation re-delivery is not counted, see
// Client.isStrictlyOlderThanHeld and qfg-rr5b).
func (t *telemetrySubmitter) RecordGuardRejected() {
	t.submitter.RecordGuardRejected()
}

// RecordResolvedFrom records one successful HTTP install by the leg that served
// it (sourceIndex 0 = primary, > 0 = secondary; negative ignored).
func (t *telemetrySubmitter) RecordResolvedFrom(sourceIndex int) {
	t.submitter.RecordResolvedFrom(sourceIndex)
}

// generateInstanceHash creates a random UUID v4 string without external dependencies.
func generateInstanceHash() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// contextSetToTelemetryData converts a ContextSet to the telemetry package's
// ContextData. The result must not share memory with the caller's context:
// the submitter aggregates it on another goroutine and json.Marshals it at
// flush time, so a caller write to a shared nested map after the call returned
// would crash the process ("concurrent map iteration and map write"). Scalars
// pass through, map[string]interface{}, []interface{} and []string are
// deep-copied, arrays and slices of scalars are copied, and
// encoding.TextMarshaler values (uuid.UUID) are rendered the way json.Marshal
// would render them. Everything else is left out of telemetry: every other map
// type (map[string]string included), slices and arrays of non-scalars
// ([]map[string]interface{}, [][]int, []uuid.UUID), structs, pointers,
// channels and funcs.
func contextSetToTelemetryData(ctx *ContextSet) telemetry.ContextData {
	contexts := make(map[string]map[string]interface{}, len(ctx.data))
	for name, nc := range ctx.data {
		props := make(map[string]interface{}, len(nc.Data))
		for k, v := range nc.Data {
			if c, ok := telemetryContextValue(v); ok {
				props[k] = c
			}
		}
		contexts[name] = props
	}
	return telemetry.ContextData{Contexts: contexts}
}

// contextSetToTelemetryShapes converts a ContextSet for shapes_only context
// telemetry. Each value is replaced by telemetry.ShapeValue, which has the same
// inferred field type and shares nothing with the caller, so the shapes are
// exactly the ones v1.5.0 recorded (no kind is dropped) and no value is
// copied, reflected on or rendered.
func contextSetToTelemetryShapes(ctx *ContextSet) telemetry.ContextData {
	contexts := make(map[string]map[string]interface{}, len(ctx.data))
	for name, nc := range ctx.data {
		props := make(map[string]interface{}, len(nc.Data))
		for k, v := range nc.Data {
			props[k] = telemetry.ShapeValue(v)
		}
		contexts[name] = props
	}
	return telemetry.ContextData{Contexts: contexts}
}

// telemetryContextValue returns a copy of v that is safe to hand to the
// telemetry goroutine, or ok=false when v is a kind telemetry drops. It
// deep-copies JSON-shaped values like deepCopyJSONValue, but drops an unknown
// reference kind (at any depth) instead of returning it as is, because here it
// would still be the caller's object. It also copies slices of scalars and
// renders TextMarshalers (see telemetryMarshalledValue); the user's
// MarshalJSON/MarshalText therefore runs here, on the evaluating goroutine,
// once per evaluation. Only periodic_example context telemetry calls it.
func telemetryContextValue(v interface{}) (interface{}, bool) {
	switch t := v.(type) {
	case nil, string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, time.Time:
		return v, true
	case map[string]interface{}:
		if t == nil {
			return v, true
		}
		out := make(map[string]interface{}, len(t))
		for k, e := range t {
			if c, ok := telemetryContextValue(e); ok {
				out[k] = c
			}
		}
		return out, true
	case []interface{}:
		if t == nil {
			return v, true
		}
		out := make([]interface{}, 0, len(t))
		for _, e := range t {
			if c, ok := telemetryContextValue(e); ok {
				out = append(out, c)
			}
		}
		return out, true
	case []string:
		return deepCopyJSONValue(t), true
	}
	rt := reflect.TypeOf(v)
	if _, ok := v.(encoding.TextMarshaler); ok && rt.Kind() != reflect.Pointer {
		return telemetryMarshalledValue(v)
	}
	switch {
	case isTelemetryScalarKind(rt.Kind()):
		// Named scalar types (type Plan string) are values, so they are safe.
		return v, true
	case rt.Kind() == reflect.Array && isTelemetryScalarKind(rt.Elem().Kind()):
		// The interface holds its own copy of an array.
		return v, true
	case rt.Kind() == reflect.Slice && isTelemetryScalarKind(rt.Elem().Kind()):
		// []int, []int64, []byte, type IDs []int64: a flat copy of the same
		// type shares nothing with the caller and marshals like the original.
		src := reflect.ValueOf(v)
		if src.IsNil() {
			return v, true
		}
		dst := reflect.MakeSlice(rt, src.Len(), src.Len())
		reflect.Copy(dst, src)
		return dst.Interface(), true
	}
	return nil, false
}

// telemetryMarshalledValue renders a non-pointer encoding.TextMarshaler
// (uuid.UUID, netip.Addr) now, on the recording goroutine, into data that
// shares nothing with the caller. It follows json.Marshal, which is what the
// flush would have run: MarshalJSON wins over MarshalText. A JSON string
// becomes a Go string (so a uuid key still identifies an example context);
// JSON null becomes nil; any other JSON becomes a json.RawMessage, which
// marshals verbatim. A marshal error drops the value.
func telemetryMarshalledValue(v interface{}) (interface{}, bool) {
	if _, ok := v.(json.Marshaler); !ok {
		text, err := v.(encoding.TextMarshaler).MarshalText()
		if err != nil {
			return nil, false
		}
		return string(text), true
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	if string(out) == "null" {
		// json.Unmarshal("null", &s) succeeds and leaves s == "".
		return nil, true
	}
	var s string
	if json.Unmarshal(out, &s) == nil {
		return s, true
	}
	return json.RawMessage(out), true
}

// isTelemetryScalarKind reports whether k is a bool, string or numeric kind.
func isTelemetryScalarKind(k reflect.Kind) bool {
	switch k {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}
