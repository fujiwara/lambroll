package lambroll

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/aereal/jsondiff"
)

func mustJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("invalid json %q: %v", s, err)
	}
	return v
}

func dumpJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestResolveMaskSelector(t *testing.T) {
	cases := []struct {
		arg      string
		selector string
		warn     bool
	}{
		{"DB_PASSWORD", `.Environment.Variables["DB_PASSWORD"]`, false},
		{".Environment.Variables[]", ".Environment.Variables[]", false},
		{".Foo.Bar", ".Foo.Bar", false},
		{"looks.like.selector", `.Environment.Variables["looks.like.selector"]`, true},
		{"has[bracket]", `.Environment.Variables["has[bracket]"]`, true},
		// a name containing " must be JSON-escaped so it cannot break the subscript
		{`a"b`, `.Environment.Variables["a\"b"]`, false},
	}
	for _, c := range cases {
		sel, warn := resolveMaskSelector(c.arg)
		if sel != c.selector || warn != c.warn {
			t.Errorf("resolveMaskSelector(%q) = (%q, %v), want (%q, %v)", c.arg, sel, warn, c.selector, c.warn)
		}
	}
}

func TestResolveMaskSelectorsDedup(t *testing.T) {
	got := resolveMaskSelectors([]string{
		"DB_PASSWORD",
		".Environment.Variables[]",
		"DB_PASSWORD", // dup of the first (same resolved selector)
		".Environment.Variables[]",
	})
	want := []string{`.Environment.Variables["DB_PASSWORD"]`, ".Environment.Variables[]"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("resolveMaskSelectors dedup = %v, want %v", got, want)
	}
}

func TestMaskTokenRegistry(t *testing.T) {
	r := newMaskTokenRegistry()
	a1 := r.tokenFor(`"secret"`)
	a2 := r.tokenFor(`"secret"`) // same value -> same token
	b := r.tokenFor(`"other"`)   // different value -> different token
	if a1 != a2 {
		t.Errorf("equal values got different tokens: %q vs %q", a1, a2)
	}
	if a1 == b {
		t.Errorf("distinct values share a token: %q", a1)
	}
	if a1 != "***MASKED#1***" || b != "***MASKED#2***" {
		t.Errorf("unexpected tokens: %q %q", a1, b)
	}
}

func TestCanonicalizeKeyOrderIndependent(t *testing.T) {
	x := mustJSON(t, `{"b":1,"a":2}`)
	y := mustJSON(t, `{"a":2,"b":1}`)
	cx, _ := canonicalize(x)
	cy, _ := canonicalize(y)
	if cx != cy {
		t.Errorf("structurally-equal values canonicalized differently: %q vs %q", cx, cy)
	}
}

func TestApplyMaskEnvVar(t *testing.T) {
	reg := newMaskTokenRegistry()
	root := mustJSON(t, `{"Environment":{"Variables":{"A":"1","B":"2"}}}`)
	got, err := applyMask(root, []string{`.Environment.Variables["A"]`}, reg)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"Environment":{"Variables":{"A":"***MASKED#1***","B":"2"}}}`
	if dumpJSON(t, got) != want {
		t.Errorf("applyMask = %s, want %s", dumpJSON(t, got), want)
	}
}

// TestApplyMaskEnvVarNameWithQuote verifies a name containing " is escaped so
// the generated selector still targets exactly that key (and does not break).
func TestApplyMaskEnvVarNameWithQuote(t *testing.T) {
	reg := newMaskTokenRegistry()
	root := mustJSON(t, `{"Environment":{"Variables":{"a\"b":"secret","c":"keep"}}}`)
	sel, _ := resolveMaskSelector(`a"b`)
	got, err := applyMask(root, []string{sel}, reg)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"Environment":{"Variables":{"a\"b":"***MASKED#1***","c":"keep"}}}`
	if dumpJSON(t, got) != want {
		t.Errorf("applyMask = %s, want %s", dumpJSON(t, got), want)
	}
}

// TestApplyMaskDrift verifies the drift-visibility contract: equal values across
// two renders share a token (no diff), distinct values get distinct tokens.
func TestApplyMaskDrift(t *testing.T) {
	reg := newMaskTokenRegistry()
	sel := []string{".Environment.Variables[]"}

	remote, err := applyMask(mustJSON(t, `{"Environment":{"Variables":{"SAME":"x","CHANGED":"old"}}}`), sel, reg)
	if err != nil {
		t.Fatal(err)
	}
	local, err := applyMask(mustJSON(t, `{"Environment":{"Variables":{"SAME":"x","CHANGED":"new"}}}`), sel, reg)
	if err != nil {
		t.Fatal(err)
	}

	rv := remote.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)
	lv := local.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)

	if rv["SAME"] != lv["SAME"] {
		t.Errorf("equal value got different tokens: %v vs %v", rv["SAME"], lv["SAME"])
	}
	if rv["CHANGED"] == lv["CHANGED"] {
		t.Errorf("changed value shares a token: %v", rv["CHANGED"])
	}
	for _, v := range []any{rv["SAME"], rv["CHANGED"], lv["SAME"], lv["CHANGED"]} {
		if s, _ := v.(string); s == "x" || s == "old" || s == "new" {
			t.Errorf("plaintext value leaked: %v", v)
		}
	}
}

// TestApplyMaskAbsentLiteralKey guards the core correctness property: a selector
// pointing at an absent path must NOT create that path (which would fabricate a
// field and show as a spurious diff).
func TestApplyMaskAbsentLiteralKey(t *testing.T) {
	reg := newMaskTokenRegistry()
	root := mustJSON(t, `{"FunctionName":"x"}`)
	got, err := applyMask(root, []string{`.Environment.Variables["A"]`}, reg)
	if err != nil {
		t.Fatal(err)
	}
	if dumpJSON(t, got) != `{"FunctionName":"x"}` {
		t.Errorf("absent key was created: %s", dumpJSON(t, got))
	}
}

// TestApplyMaskAbsentIterable guards isNullIterationError: iterating an absent
// node is a no-op, not an error and not a created field.
func TestApplyMaskAbsentIterable(t *testing.T) {
	reg := newMaskTokenRegistry()
	root := mustJSON(t, `{"FunctionName":"x"}`)
	got, err := applyMask(root, []string{".Environment.Variables[]"}, reg)
	if err != nil {
		t.Fatalf("absent iterable should be a no-op, got error: %v", err)
	}
	if dumpJSON(t, got) != `{"FunctionName":"x"}` {
		t.Errorf("absent iterable mutated the document: %s", dumpJSON(t, got))
	}
}

// TestApplyMaskNullValueSkipped verifies a present null value is left untouched
// (null is not sensitive and select(. != null) skips it).
func TestApplyMaskNullValueSkipped(t *testing.T) {
	reg := newMaskTokenRegistry()
	root := mustJSON(t, `{"Environment":{"Variables":{"A":null,"B":"2"}}}`)
	got, err := applyMask(root, []string{".Environment.Variables[]"}, reg)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"Environment":{"Variables":{"A":null,"B":"***MASKED#1***"}}}`
	if dumpJSON(t, got) != want {
		t.Errorf("applyMask = %s, want %s", dumpJSON(t, got), want)
	}
}

func TestApplyMaskRoot(t *testing.T) {
	reg := newMaskTokenRegistry()
	root := mustJSON(t, `{"a":1}`)
	got, err := applyMask(root, []string{"."}, reg)
	if err != nil {
		t.Fatal(err)
	}
	if got != "***MASKED#1***" {
		t.Errorf("root selector should collapse whole doc to one token, got %v", got)
	}
}

func TestApplyMaskInvalidSelectorErrors(t *testing.T) {
	reg := newMaskTokenRegistry()
	root := mustJSON(t, `{"a":1}`)
	if _, err := applyMask(root, []string{".a["}, reg); err == nil {
		t.Error("expected error for invalid selector")
	}
}

func TestMaskInputNoSelectorsUntouched(t *testing.T) {
	reg := newMaskTokenRegistry()
	x := mustJSON(t, `{"a":1}`)
	got, err := maskInput(x, "", nil, reg)
	if err != nil {
		t.Fatal(err)
	}
	// same underlying value returned unchanged
	if dumpJSON(t, got) != `{"a":1}` {
		t.Errorf("maskInput with no selectors changed the value: %s", dumpJSON(t, got))
	}
}

// captureWarnSelectors swaps the default slog logger for the duration of fn and
// returns the "selector" attribute of every "mask selector matched nothing"
// warning emitted.
func captureWarnSelectors(t *testing.T, fn func()) []string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(prev)
	fn()
	var got []string
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec struct {
			Msg      string `json:"msg"`
			Selector string `json:"selector"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("invalid log line %q: %v", line, err)
		}
		if rec.Msg == "mask selector matched nothing" {
			got = append(got, rec.Selector)
		}
	}
	return got
}

// TestMaskWarnUnmatchedOneSide verifies the both-sides accounting: a selector
// that matches on only one diff side (e.g. a field empty/omitted on the other)
// is not warned about, because masking did take effect somewhere.
func TestMaskWarnUnmatchedOneSide(t *testing.T) {
	reg := newMaskTokenRegistry()
	sel := []string{".Description"}
	// remote has a non-empty Description, local omits it (empty -> omitted).
	if _, err := applyMask(mustJSON(t, `{"Description":"hello"}`), sel, reg); err != nil {
		t.Fatal(err)
	}
	if _, err := applyMask(mustJSON(t, `{"FunctionName":"x"}`), sel, reg); err != nil {
		t.Fatal(err)
	}
	got := captureWarnSelectors(t, func() { reg.warnUnmatched(sel) })
	if len(got) != 0 {
		t.Errorf("did not expect a warning when the selector matched one side, got %v", got)
	}
}

// TestMaskWarnUnmatchedNeitherSide verifies that a selector matching on neither
// side (a likely typo, or a field empty/omitted on both sides) is warned about.
func TestMaskWarnUnmatchedNeitherSide(t *testing.T) {
	reg := newMaskTokenRegistry()
	sel := []string{".Nope"}
	if _, err := applyMask(mustJSON(t, `{"Description":"hello"}`), sel, reg); err != nil {
		t.Fatal(err)
	}
	if _, err := applyMask(mustJSON(t, `{"FunctionName":"x"}`), sel, reg); err != nil {
		t.Fatal(err)
	}
	got := captureWarnSelectors(t, func() { reg.warnUnmatched(sel) })
	if len(got) != 1 || got[0] != ".Nope" {
		t.Errorf("expected one warning for .Nope, got %v", got)
	}
}

// TestEmitDiffMaskedRendersTokens verifies the full render path: a changed
// secret shows as a diff with tokens, and the plaintext never appears.
func TestEmitDiffMaskedRendersTokens(t *testing.T) {
	var buf bytes.Buffer
	opt := &DiffOption{w: &buf}
	app := &App{}
	from := &jsondiff.Input{Name: "remote", X: mustJSON(t, `{"Environment":{"Variables":{"DB_PASSWORD":"old-secret"}}}`)}
	to := &jsondiff.Input{Name: "local", X: mustJSON(t, `{"Environment":{"Variables":{"DB_PASSWORD":"new-secret"}}}`)}

	hasDiff, err := app.emitDiff(context.Background(), opt, "function.json", from, to, "", resolveMaskSelectors([]string{"DB_PASSWORD"}))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiff {
		t.Fatal("expected a diff for a changed secret")
	}
	out := buf.String()
	if strings.Contains(out, "old-secret") || strings.Contains(out, "new-secret") {
		t.Errorf("plaintext secret leaked into diff output:\n%s", out)
	}
	if !strings.Contains(out, maskTokenPrefix) {
		t.Errorf("expected mask token in output:\n%s", out)
	}
}

// TestEmitDiffMaskedUnchangedNoDiff verifies an unchanged secret produces no
// diff (equal values share a token on both sides).
func TestEmitDiffMaskedUnchangedNoDiff(t *testing.T) {
	var buf bytes.Buffer
	opt := &DiffOption{w: &buf}
	app := &App{}
	doc := `{"Environment":{"Variables":{"DB_PASSWORD":"same-secret","OTHER":"v"}}}`
	from := &jsondiff.Input{Name: "remote", X: mustJSON(t, doc)}
	to := &jsondiff.Input{Name: "local", X: mustJSON(t, doc)}

	hasDiff, err := app.emitDiff(context.Background(), opt, "function.json", from, to, "", resolveMaskSelectors([]string{"DB_PASSWORD"}))
	if err != nil {
		t.Fatal(err)
	}
	if hasDiff {
		t.Errorf("expected no diff for an unchanged secret, got:\n%s", buf.String())
	}
}

// TestRenderDefinitionMask verifies render-style single-document masking:
// matched values become tokens, others are preserved, and plaintext never
// appears.
func TestRenderDefinitionMask(t *testing.T) {
	def := mustJSON(t, `{"FunctionName":"f","Environment":{"Variables":{"DB_PASSWORD":"secret","PUBLIC":"ok"}}}`)
	b, err := renderDefinition(def, resolveMaskSelectors([]string{"DB_PASSWORD"}))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "secret") {
		t.Errorf("plaintext leaked into render output:\n%s", s)
	}
	if !strings.Contains(s, maskTokenPrefix) {
		t.Errorf("expected mask token in render output:\n%s", s)
	}
	if !strings.Contains(s, `"PUBLIC": "ok"`) {
		t.Errorf("non-masked value not preserved:\n%s", s)
	}
}

// TestRenderDefinitionNoMaskIdentical verifies that without selectors the output
// is byte-identical to the unmasked marshaling.
func TestRenderDefinitionNoMaskIdentical(t *testing.T) {
	def := mustJSON(t, `{"FunctionName":"f","Environment":{"Variables":{"DB_PASSWORD":"secret"}}}`)
	got, err := renderDefinition(def, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := marshalJSON(def)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("renderDefinition with no mask differs from marshalJSON:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestMaskInputAppliesIgnoreThenMask(t *testing.T) {
	reg := newMaskTokenRegistry()
	x := mustJSON(t, `{"Environment":{"Variables":{"A":"1"}},"Drop":"me"}`)
	got, err := maskInput(x, ".Drop", []string{`.Environment.Variables["A"]`}, reg)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"Environment":{"Variables":{"A":"***MASKED#1***"}}}`
	if dumpJSON(t, got) != want {
		t.Errorf("maskInput = %s, want %s", dumpJSON(t, got), want)
	}
	// original must be untouched (deep copy)
	if dumpJSON(t, x) != `{"Drop":"me","Environment":{"Variables":{"A":"1"}}}` {
		t.Errorf("maskInput mutated the original: %s", dumpJSON(t, x))
	}
}
