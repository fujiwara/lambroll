package lambroll_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
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
