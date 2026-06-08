package lambroll_test

import (
	"strings"
	"testing"

	"github.com/fujiwara/lambroll"
	"github.com/google/go-cmp/cmp"
)

func TestResolveMaskSelector(t *testing.T) {
	tests := []struct {
		name         string
		arg          string
		wantSelector string
		wantWarn     bool
	}{
		{
			name:         "dot-prefixed selector used verbatim",
			arg:          ".Environment.Variables",
			wantSelector: ".Environment.Variables",
			wantWarn:     false,
		},
		{
			name:         "dot-prefixed per-value selector used verbatim",
			arg:          ".Environment.Variables[]",
			wantSelector: ".Environment.Variables[]",
			wantWarn:     false,
		},
		{
			name:         "dotless name expanded to env-var selector",
			arg:          "DB_PASSWORD",
			wantSelector: `.Environment.Variables["DB_PASSWORD"]`,
			wantWarn:     false,
		},
		{
			name:         "env-var name matching is case-sensitive (lower)",
			arg:          "db_password",
			wantSelector: `.Environment.Variables["db_password"]`,
			wantWarn:     false,
		},
		{
			name:         "dotless containing dot warns but still treated as name",
			arg:          "Environment.Variables",
			wantSelector: `.Environment.Variables["Environment.Variables"]`,
			wantWarn:     true,
		},
		{
			name:         "dotless containing bracket warns",
			arg:          "Variables[FOO]",
			wantSelector: `.Environment.Variables["Variables[FOO]"]`,
			wantWarn:     true,
		},
		{
			name:         "dotless containing pipe warns",
			arg:          "A|B",
			wantSelector: `.Environment.Variables["A|B"]`,
			wantWarn:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSelector, gotWarn := lambroll.ResolveMaskSelector(tt.arg)
			if gotSelector != tt.wantSelector {
				t.Errorf("selector: got %q, want %q", gotSelector, tt.wantSelector)
			}
			if gotWarn != tt.wantWarn {
				t.Errorf("warn: got %v, want %v", gotWarn, tt.wantWarn)
			}
		})
	}
}

func TestResolveMaskSelectorCaseSensitive(t *testing.T) {
	upper, _ := lambroll.ResolveMaskSelector("DB_PASSWORD")
	lower, _ := lambroll.ResolveMaskSelector("db_password")
	if upper == lower {
		t.Errorf("expected case-sensitive distinct selectors, both %q", upper)
	}
}

func TestBuildEffectiveMaskSet(t *testing.T) {
	tests := []struct {
		name   string
		cli    []string
		option []string
		want   []string
	}{
		{
			name:   "empty produces empty",
			cli:    nil,
			option: nil,
			want:   []string{},
		},
		{
			name:   "option only",
			cli:    nil,
			option: []string{"DB_PASSWORD", ".Environment.Variables"},
			want: []string{
				`.Environment.Variables["DB_PASSWORD"]`,
				".Environment.Variables",
			},
		},
		{
			name:   "cli added on top of option",
			cli:    []string{"API_KEY"},
			option: []string{"DB_PASSWORD"},
			want: []string{
				`.Environment.Variables["DB_PASSWORD"]`,
				`.Environment.Variables["API_KEY"]`,
			},
		},
		{
			name:   "duplicate across sources deduped, option order wins",
			cli:    []string{"DB_PASSWORD", "API_KEY"},
			option: []string{"DB_PASSWORD"},
			want: []string{
				`.Environment.Variables["DB_PASSWORD"]`,
				`.Environment.Variables["API_KEY"]`,
			},
		},
		{
			name:   "dot and name resolving to same selector dedupe",
			cli:    []string{`.Environment.Variables["X"]`},
			option: []string{"X"},
			want: []string{
				`.Environment.Variables["X"]`,
			},
		},
		{
			name:   "duplicates within one source deduped",
			cli:    []string{"A", "A", "B"},
			option: nil,
			want: []string{
				`.Environment.Variables["A"]`,
				`.Environment.Variables["B"]`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lambroll.BuildEffectiveMaskSet(tt.cli, tt.option)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("effective mask set (-want +got):\n%s", diff)
			}

			// fail-safe: size never below deduped option count.
			dedupedOption := map[string]struct{}{}
			for _, o := range tt.option {
				sel, _ := lambroll.ResolveMaskSelector(o)
				dedupedOption[sel] = struct{}{}
			}
			if len(got) < len(dedupedOption) {
				t.Errorf("effective set size %d < deduped option count %d", len(got), len(dedupedOption))
			}
		})
	}
}

func TestMaskTokenRegistry(t *testing.T) {
	reg := lambroll.NewMaskTokenRegistry()
	if got := reg.NextCounter(); got != 1 {
		t.Fatalf("initial counter = %d, want 1", got)
	}

	a := reg.TokenFor(`"alpha"`)
	if a != "***MASKED#1***" {
		t.Errorf("first token = %q, want ***MASKED#1***", a)
	}
	b := reg.TokenFor(`"beta"`)
	if b != "***MASKED#2***" {
		t.Errorf("second token = %q, want ***MASKED#2***", b)
	}
	// collapse-equals: same canonical form reuses the token, counter unchanged.
	aAgain := reg.TokenFor(`"alpha"`)
	if aAgain != a {
		t.Errorf("collapse-equals failed: got %q, want %q", aAgain, a)
	}
	if got := reg.NextCounter(); got != 3 {
		t.Errorf("counter after 2 distinct + 1 repeat = %d, want 3", got)
	}
	c := reg.TokenFor(`"gamma"`)
	if c != "***MASKED#3***" {
		t.Errorf("third distinct token = %q, want ***MASKED#3***", c)
	}
	if a == b || a == c || b == c {
		t.Errorf("distinct values produced non-distinct tokens: %q %q %q", a, b, c)
	}
}

func TestCanonicalizeCollapsesEqualObjects(t *testing.T) {
	o1 := map[string]any{"A": "1", "B": "2"}
	o2 := map[string]any{"B": "2", "A": "1"}
	c1, err := lambroll.Canonicalize(o1)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := lambroll.Canonicalize(o2)
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 {
		t.Errorf("equal objects canonicalize differently:\n%q\n%q", c1, c2)
	}

	o3 := map[string]any{"A": "1", "B": "3"}
	c3, err := lambroll.Canonicalize(o3)
	if err != nil {
		t.Fatal(err)
	}
	if c1 == c3 {
		t.Errorf("different objects canonicalize identically: %q", c1)
	}
}

func TestApplyMask(t *testing.T) {
	const secret = "super-secret-value"
	const apiKey = "api-key-123"

	t.Run("per-value selector keeps keys, tokenizes each value", func(t *testing.T) {
		reg := lambroll.NewMaskTokenRegistry()
		root := map[string]any{
			"Environment": map[string]any{
				"Variables": map[string]any{
					"DB_PASSWORD": secret,
					"API_KEY":     apiKey,
				},
			},
		}
		out, err := lambroll.ApplyMask(root, []string{".Environment.Variables[]"}, reg)
		if err != nil {
			t.Fatal(err)
		}
		vars := out.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)
		if _, ok := vars["DB_PASSWORD"]; !ok {
			t.Error("DB_PASSWORD key was removed")
		}
		if _, ok := vars["API_KEY"]; !ok {
			t.Error("API_KEY key was removed")
		}
		for k, v := range vars {
			if !strings.HasPrefix(v.(string), "***MASKED#") {
				t.Errorf("value for %q not masked: %q", k, v)
			}
		}
		assertNoRaw(t, out, secret, apiKey)
	})

	t.Run("whole-object selector collapses node to one token", func(t *testing.T) {
		reg := lambroll.NewMaskTokenRegistry()
		root := map[string]any{
			"Environment": map[string]any{
				"Variables": map[string]any{
					"DB_PASSWORD": secret,
					"API_KEY":     apiKey,
				},
			},
		}
		out, err := lambroll.ApplyMask(root, []string{".Environment.Variables"}, reg)
		if err != nil {
			t.Fatal(err)
		}
		env := out.(map[string]any)["Environment"].(map[string]any)
		// the Variables key stays present, but its value becomes a single token
		// (object -> string), hiding both keys and values.
		val, ok := env["Variables"].(string)
		if !ok {
			t.Fatalf("Variables not collapsed to a string token, got %T", env["Variables"])
		}
		if !strings.HasPrefix(val, "***MASKED#") {
			t.Errorf("whole-object value not a token: %q", val)
		}
		assertNoRaw(t, out, secret, apiKey, "DB_PASSWORD", "API_KEY")
	})

	t.Run("scalar selector masks a single value", func(t *testing.T) {
		reg := lambroll.NewMaskTokenRegistry()
		root := map[string]any{
			"Environment": map[string]any{
				"Variables": map[string]any{
					"DB_PASSWORD": secret,
				},
			},
		}
		out, err := lambroll.ApplyMask(root, []string{`.Environment.Variables["DB_PASSWORD"]`}, reg)
		if err != nil {
			t.Fatal(err)
		}
		vars := out.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)
		if v := vars["DB_PASSWORD"].(string); v != "***MASKED#1***" {
			t.Errorf("scalar value = %q, want ***MASKED#1***", v)
		}
	})

	t.Run("equal local/remote values produce same token (drift invariant)", func(t *testing.T) {
		reg := lambroll.NewMaskTokenRegistry()
		local := map[string]any{
			"Environment": map[string]any{"Variables": map[string]any{"DB_PASSWORD": secret}},
		}
		remote := map[string]any{
			"Environment": map[string]any{"Variables": map[string]any{"DB_PASSWORD": secret}},
		}
		sel := []string{`.Environment.Variables["DB_PASSWORD"]`}
		lOut, _ := lambroll.ApplyMask(local, sel, reg)
		rOut, _ := lambroll.ApplyMask(remote, sel, reg)
		lv := lOut.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)["DB_PASSWORD"]
		rv := rOut.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)["DB_PASSWORD"]
		if lv != rv {
			t.Errorf("equal values masked to different tokens: %q vs %q", lv, rv)
		}
	})

	t.Run("changed local/remote values produce different tokens (drift visible)", func(t *testing.T) {
		reg := lambroll.NewMaskTokenRegistry()
		local := map[string]any{
			"Environment": map[string]any{"Variables": map[string]any{"DB_PASSWORD": "old"}},
		}
		remote := map[string]any{
			"Environment": map[string]any{"Variables": map[string]any{"DB_PASSWORD": "new"}},
		}
		sel := []string{`.Environment.Variables["DB_PASSWORD"]`}
		lOut, _ := lambroll.ApplyMask(local, sel, reg)
		rOut, _ := lambroll.ApplyMask(remote, sel, reg)
		lv := lOut.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)["DB_PASSWORD"]
		rv := rOut.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)["DB_PASSWORD"]
		if lv == rv {
			t.Errorf("changed values masked to the same token %q", lv)
		}
	})

	t.Run("match-nothing selector is a no-op and does not mutate", func(t *testing.T) {
		reg := lambroll.NewMaskTokenRegistry()
		root := map[string]any{
			"Environment": map[string]any{"Variables": map[string]any{"DB_PASSWORD": secret}},
		}
		out, err := lambroll.ApplyMask(root, []string{`.Environment.Variables["NOPE"]`}, reg)
		if err != nil {
			t.Fatal(err)
		}
		v := out.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)["DB_PASSWORD"]
		if v != secret {
			t.Errorf("match-nothing selector altered an unrelated value: %q", v)
		}
		if got := reg.NextCounter(); got != 1 {
			t.Errorf("match-nothing selector allocated a token, counter = %d", got)
		}
	})

	t.Run("empty selector set is a no-op", func(t *testing.T) {
		reg := lambroll.NewMaskTokenRegistry()
		root := map[string]any{
			"Environment": map[string]any{"Variables": map[string]any{"DB_PASSWORD": secret}},
		}
		out, err := lambroll.ApplyMask(root, nil, reg)
		if err != nil {
			t.Fatal(err)
		}
		v := out.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)["DB_PASSWORD"]
		if v != secret {
			t.Errorf("empty mask set altered a value: %q", v)
		}
	})

	t.Run("global counter spans selectors and equal values collapse cross-path", func(t *testing.T) {
		reg := lambroll.NewMaskTokenRegistry()
		root := map[string]any{
			"Environment": map[string]any{"Variables": map[string]any{
				"DB_PASSWORD": secret,
				"COPY":        secret, // same value via a different path
				"API_KEY":     apiKey,
			}},
		}
		out, err := lambroll.ApplyMask(root, []string{".Environment.Variables[]"}, reg)
		if err != nil {
			t.Fatal(err)
		}
		vars := out.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)
		if vars["DB_PASSWORD"] != vars["COPY"] {
			t.Errorf("equal values across paths got different tokens: %q vs %q", vars["DB_PASSWORD"], vars["COPY"])
		}
		if vars["DB_PASSWORD"] == vars["API_KEY"] {
			t.Errorf("distinct values collapsed: %q", vars["API_KEY"])
		}
		if got := reg.NextCounter(); got != 3 {
			t.Errorf("global counter = %d, want 3", got)
		}
	})
}

func TestApplyMaskErroringSelectorFailsLoud(t *testing.T) {
	root := func() map[string]any {
		return map[string]any{
			"Environment": map[string]any{"Variables": map[string]any{"DB_PASSWORD": "super-secret"}},
			"Layers":      []any{"arn:a", "arn:b"},
		}
	}
	for _, sel := range []string{
		".Environment.Variables | keys[]", // errors mid-iteration (must not panic)
		".Environment.Variables[0]",       // indexes a map
		".Layers[0:2]",                    // array slice (unsupported path element)
	} {
		t.Run(sel, func(t *testing.T) {
			reg := lambroll.NewMaskTokenRegistry()
			if _, err := lambroll.ApplyMask(root(), []string{sel}, reg); err == nil {
				t.Errorf("expected an error for selector %q, got nil (a silent no-op would leave the value unmasked)", sel)
			}
		})
	}
}

func TestApplyMaskRootSelectorMasksWholeDocument(t *testing.T) {
	reg := lambroll.NewMaskTokenRegistry()
	root := map[string]any{"Environment": map[string]any{"Variables": map[string]any{"DB_PASSWORD": "super-secret"}}}
	out, err := lambroll.ApplyMask(root, []string{"."}, reg)
	if err != nil {
		t.Fatalf("root selector must not abort the diff: %v", err)
	}
	tok, ok := out.(string)
	if !ok {
		t.Fatalf("root selector should collapse the document to a token, got %T", out)
	}
	if !strings.HasPrefix(tok, "***MASKED#") {
		t.Errorf("root selector did not produce a mask token: %q", tok)
	}
	if strings.Contains(tok, "super-secret") {
		t.Errorf("raw value leaked into the root token: %q", tok)
	}
}

func TestApplyMaskAbsentIterable(t *testing.T) {
	reg := lambroll.NewMaskTokenRegistry()
	root := map[string]any{"FunctionName": "f"} // no Environment block
	out, err := lambroll.ApplyMask(root, []string{".Environment.Variables[]"}, reg)
	if err != nil {
		t.Fatalf("iterating an absent node must not fail: %v", err)
	}
	if out.(map[string]any)["FunctionName"] != "f" {
		t.Errorf("absent-iterable selector altered the document: %v", out)
	}
	if got := reg.NextCounter(); got != 1 {
		t.Errorf("absent-iterable selector allocated a token, counter = %d", got)
	}
}

func TestApplyMaskNegativeIndex(t *testing.T) {
	reg := lambroll.NewMaskTokenRegistry()
	root := map[string]any{"Layers": []any{"arn:a", "arn:b", "arn:c"}}
	out, err := lambroll.ApplyMask(root, []string{".Layers[-1]"}, reg)
	if err != nil {
		t.Fatal(err)
	}
	layers := out.(map[string]any)["Layers"].([]any)
	if layers[2] == "arn:c" {
		t.Errorf("negative index did not mask the last element: %v", layers)
	}
	if layers[0] != "arn:a" || layers[1] != "arn:b" {
		t.Errorf("negative index masked the wrong elements: %v", layers)
	}
}

func assertNoRaw(t *testing.T, out any, raws ...string) {
	t.Helper()
	b, err := lambroll.MarshalJSON(out)
	if err != nil {
		t.Fatalf("marshal masked output: %v", err)
	}
	s := string(b)
	for _, raw := range raws {
		if strings.Contains(s, raw) {
			t.Errorf("raw value %q leaked into masked output:\n%s", raw, s)
		}
	}
}
