package lambroll_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/aereal/jsondiff"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/fatih/color"
	"github.com/fujiwara/lambroll"
	"github.com/google/go-cmp/cmp"
)

func TestSortFunctionForDiff(t *testing.T) {
	fn := &lambroll.Function{
		VpcConfig: &types.VpcConfig{
			SubnetIds: []string{
				"subnet-08dc9a51660120991",
				"subnet-023e96b860485e2ad",
				"subnet-045cd24ab8e92a20d",
			},
			SecurityGroupIds: []string{
				"sg-99999999",
				"sg-11111111",
			},
		},
	}
	lambroll.SortFunctionForDiff(fn)

	wantSubnets := []string{
		"subnet-023e96b860485e2ad",
		"subnet-045cd24ab8e92a20d",
		"subnet-08dc9a51660120991",
	}
	if diff := cmp.Diff(wantSubnets, fn.VpcConfig.SubnetIds); diff != "" {
		t.Errorf("subnet ids not sorted (-want +got):\n%s", diff)
	}

	wantSGs := []string{"sg-11111111", "sg-99999999"}
	if diff := cmp.Diff(wantSGs, fn.VpcConfig.SecurityGroupIds); diff != "" {
		t.Errorf("security group ids not sorted (-want +got):\n%s", diff)
	}
}

func TestSortFunctionForDiffNil(t *testing.T) {
	// must not panic on nil function or nil VpcConfig
	lambroll.SortFunctionForDiff(nil)
	lambroll.SortFunctionForDiff(&lambroll.Function{})
}

func TestRunExternalDiff(t *testing.T) {
	// "cat" concatenates the remote and local files passed as the last two args,
	// so the writer must contain both contents in order.
	var buf bytes.Buffer
	err := lambroll.RunExternalDiff(
		context.Background(),
		"cat",
		"function.json",
		"remote-content\n",
		"local-content\n",
		&buf,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "remote-content\nlocal-content\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestRunExternalDiffCommandError(t *testing.T) {
	var buf bytes.Buffer
	err := lambroll.RunExternalDiff(
		context.Background(),
		"this-command-should-not-exist-xyz",
		"function.json",
		"a", "b",
		&buf,
	)
	if err == nil {
		t.Error("expected an error for a non-existent command, got nil")
	}
}

func TestRenderForExternalDiff(t *testing.T) {
	t.Run("string passes through", func(t *testing.T) {
		got, err := lambroll.RenderForExternalDiff("CodeSha256: abc", "")
		if err != nil {
			t.Fatal(err)
		}
		if got != "CodeSha256: abc" {
			t.Errorf("unexpected: %q", got)
		}
	})

	t.Run("map marshaled as indented json", func(t *testing.T) {
		got, err := lambroll.RenderForExternalDiff(map[string]any{"a": "b"}, "")
		if err != nil {
			t.Fatal(err)
		}
		want := "{\n  \"a\": \"b\"\n}\n"
		if got != want {
			t.Errorf("unexpected: %q want %q", got, want)
		}
	})

	t.Run("ignore query deletes fields", func(t *testing.T) {
		in := map[string]any{"Keep": "yes", "Drop": "no"}
		got, err := lambroll.RenderForExternalDiff(in, ".Drop")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, "Drop") {
			t.Errorf("ignored field should be removed: %q", got)
		}
		if !strings.Contains(got, "Keep") {
			t.Errorf("kept field should remain: %q", got)
		}
	})
}

func newFuncConfig(vars map[string]any) map[string]any {
	return map[string]any{
		"FunctionName": "test-func",
		"Environment": map[string]any{
			"Variables": vars,
		},
	}
}

func runEmitDiff(t *testing.T, from, to map[string]any, ignore string, selectors []string) (string, bool) {
	t.Helper()
	app := &lambroll.App{}
	opt := &lambroll.DiffOption{}
	var buf bytes.Buffer
	opt.SetWriter(&buf)
	reg := lambroll.NewMaskTokenRegistry()
	d, err := app.EmitDiff(
		context.Background(), opt, "function.json",
		&jsondiff.Input{Name: "remote", X: from},
		&jsondiff.Input{Name: "local", X: to},
		ignore, selectors, reg,
	)
	if err != nil {
		t.Fatalf("emitDiff error: %v", err)
	}
	return buf.String(), d
}

func TestEmitDiffMaskBuiltin(t *testing.T) {
	const oldSecret = "old-secret-value"
	const newSecret = "new-secret-value"

	t.Run("changed value renders as differing tokens, drift visible", func(t *testing.T) {
		from := newFuncConfig(map[string]any{"DB_PASSWORD": oldSecret})
		to := newFuncConfig(map[string]any{"DB_PASSWORD": newSecret})
		out, hasDiff := runEmitDiff(t, from, to, "", []string{`.Environment.Variables["DB_PASSWORD"]`})
		if !hasDiff {
			t.Fatal("expected a difference for changed value")
		}
		if strings.Contains(out, oldSecret) || strings.Contains(out, newSecret) {
			t.Errorf("raw value leaked into built-in diff:\n%s", out)
		}
		if !strings.Contains(out, "***MASKED#") {
			t.Errorf("expected a mask token in diff output:\n%s", out)
		}
		// two distinct values -> two distinct tokens
		if !strings.Contains(out, "***MASKED#1***") || !strings.Contains(out, "***MASKED#2***") {
			t.Errorf("expected distinct tokens for drift:\n%s", out)
		}
	})

	t.Run("equal masked values produce no diff line and no difference", func(t *testing.T) {
		from := newFuncConfig(map[string]any{"DB_PASSWORD": oldSecret})
		to := newFuncConfig(map[string]any{"DB_PASSWORD": oldSecret})
		out, hasDiff := runEmitDiff(t, from, to, "", []string{`.Environment.Variables["DB_PASSWORD"]`})
		if hasDiff {
			t.Errorf("equal masked values reported a difference:\n%s", out)
		}
		if out != "" {
			t.Errorf("equal masked values produced output:\n%s", out)
		}
	})
}

func TestEmitDiffMaskExternal(t *testing.T) {
	const oldSecret = "old-secret-value"
	const newSecret = "new-secret-value"

	app := &lambroll.App{}
	opt := &lambroll.DiffOption{External: "cat"}
	var buf bytes.Buffer
	opt.SetWriter(&buf)
	reg := lambroll.NewMaskTokenRegistry()
	from := newFuncConfig(map[string]any{"DB_PASSWORD": oldSecret})
	to := newFuncConfig(map[string]any{"DB_PASSWORD": newSecret})

	d, err := app.EmitDiff(
		context.Background(), opt, "function.json",
		&jsondiff.Input{Name: "remote", X: from},
		&jsondiff.Input{Name: "local", X: to},
		"", []string{`.Environment.Variables["DB_PASSWORD"]`}, reg,
	)
	if err != nil {
		t.Fatalf("emitDiff error: %v", err)
	}
	if !d {
		t.Fatal("expected a difference")
	}
	out := buf.String()
	if strings.Contains(out, oldSecret) || strings.Contains(out, newSecret) {
		t.Errorf("raw value leaked into external diff input:\n%s", out)
	}
	if !strings.Contains(out, "***MASKED#") {
		t.Errorf("expected a mask token in external diff input:\n%s", out)
	}
}

func TestEmitDiffMaskWholeObject(t *testing.T) {
	from := newFuncConfig(map[string]any{"DB_PASSWORD": "p1", "API_KEY": "k1"})
	to := newFuncConfig(map[string]any{"DB_PASSWORD": "p2", "API_KEY": "k2"})
	out, hasDiff := runEmitDiff(t, from, to, "", []string{".Environment.Variables"})
	if !hasDiff {
		t.Fatal("expected a difference")
	}
	for _, raw := range []string{"p1", "p2", "k1", "k2", "DB_PASSWORD", "API_KEY"} {
		if strings.Contains(out, raw) {
			t.Errorf("whole-object mask leaked %q:\n%s", raw, out)
		}
	}
	if !strings.Contains(out, "***MASKED#") {
		t.Errorf("expected a mask token:\n%s", out)
	}
}

func TestEmitDiffNoMaskByteIdentical(t *testing.T) {
	// disable coloring so the byte comparison is deterministic regardless of TTY
	prev := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = prev }()

	const secret = "secret-value"
	from := newFuncConfig(map[string]any{"DB_PASSWORD": secret, "PLAIN": "old"})
	to := newFuncConfig(map[string]any{"DB_PASSWORD": secret, "PLAIN": "new"})

	withEmpty, hasDiffEmpty := runEmitDiff(t, from, to, "", nil)

	// pre-feature equivalent: jsondiff then the same coloring emitDiff applies
	pre, err := jsondiff.Diff(
		&jsondiff.Input{Name: "remote", X: from},
		&jsondiff.Input{Name: "local", X: to},
	)
	if err != nil {
		t.Fatal(err)
	}

	if !hasDiffEmpty {
		t.Fatal("expected a difference on the unmasked field")
	}
	if !strings.Contains(withEmpty, "old") || !strings.Contains(withEmpty, "new") {
		t.Errorf("empty mask set should not mask anything:\n%s", withEmpty)
	}
	if want := coloredLikeEmitDiff(pre); withEmpty != want {
		t.Errorf("empty mask set not byte-identical to pre-feature diff:\ngot:  %q\nwant: %q", withEmpty, want)
	}
}

func coloredLikeEmitDiff(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		b.WriteString(line + "\n")
	}
	return b.String()
}

func TestEmitDiffMaskIsolation(t *testing.T) {
	const secret = "isolation-secret"
	from := newFuncConfig(map[string]any{"DB_PASSWORD": secret})
	to := newFuncConfig(map[string]any{"DB_PASSWORD": "other-secret"})

	_, _ = runEmitDiff(t, from, to, "", []string{`.Environment.Variables["DB_PASSWORD"]`})

	gotFrom := from["Environment"].(map[string]any)["Variables"].(map[string]any)["DB_PASSWORD"]
	if gotFrom != secret {
		t.Errorf("original 'from' input was mutated: %q", gotFrom)
	}
	gotTo := to["Environment"].(map[string]any)["Variables"].(map[string]any)["DB_PASSWORD"]
	if gotTo != "other-secret" {
		t.Errorf("original 'to' input was mutated: %q", gotTo)
	}
}

func TestEmitDiffMaskSurfaceRestriction(t *testing.T) {
	// Simulate the CodeSha256 path: string inputs, no selectors.
	app := &lambroll.App{}
	opt := &lambroll.DiffOption{}
	var buf bytes.Buffer
	opt.SetWriter(&buf)
	reg := lambroll.NewMaskTokenRegistry()
	d, err := app.EmitDiff(
		context.Background(), opt, "CodeSha256.txt",
		&jsondiff.Input{Name: "remote", X: "CodeSha256: abc123"},
		&jsondiff.Input{Name: "--src=.", X: "CodeSha256: def456"},
		"", nil, reg,
	)
	if err != nil {
		t.Fatalf("emitDiff error: %v", err)
	}
	if !d {
		t.Fatal("expected a difference")
	}
	out := buf.String()
	if strings.Contains(out, "***MASKED#") {
		t.Errorf("CodeSha256 surface must never be masked:\n%s", out)
	}
	if !strings.Contains(out, "abc123") || !strings.Contains(out, "def456") {
		t.Errorf("CodeSha256 raw values should be visible:\n%s", out)
	}
}

func TestMaskInputIgnoreThenMask(t *testing.T) {
	const secret = "overlap-secret"
	in := newFuncConfig(map[string]any{"DB_PASSWORD": secret, "KEEP": "keep-me"})
	// ignore removes DB_PASSWORD; the mask selector then matches nothing.
	out, err := lambroll.MaskInput(
		in,
		`.Environment.Variables["DB_PASSWORD"]`,
		[]string{`.Environment.Variables["DB_PASSWORD"]`},
		lambroll.NewMaskTokenRegistry(),
	)
	if err != nil {
		t.Fatal(err)
	}
	vars := out.(map[string]any)["Environment"].(map[string]any)["Variables"].(map[string]any)
	if _, ok := vars["DB_PASSWORD"]; ok {
		t.Errorf("ignored field should be removed before masking: %v", vars)
	}
	if vars["KEEP"] != "keep-me" {
		t.Errorf("non-ignored, non-masked field altered: %v", vars["KEEP"])
	}
	// original input must remain intact (deep-copy isolation)
	if in["Environment"].(map[string]any)["Variables"].(map[string]any)["DB_PASSWORD"] != secret {
		t.Error("maskInput mutated the original input")
	}
}
