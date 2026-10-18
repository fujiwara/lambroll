package lambroll_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/fujiwara/lambroll"
)

func TestPlanReservedConcurrency(t *testing.T) {
	cases := []struct {
		name    string
		local   *int32
		current *int32
		expect  any
		wantErr bool
	}{
		{"not managed", nil, aws.Int32(10), lambroll.ConcurrencyActionNone, false},
		{"not managed and not set", nil, nil, lambroll.ConcurrencyActionNone, false},
		{"set new value", aws.Int32(10), nil, lambroll.ConcurrencyActionPut, false},
		{"change value", aws.Int32(10), aws.Int32(5), lambroll.ConcurrencyActionPut, false},
		{"unchanged", aws.Int32(10), aws.Int32(10), lambroll.ConcurrencyActionNone, false},
		{"set zero", aws.Int32(0), nil, lambroll.ConcurrencyActionPut, false},
		{"zero unchanged", aws.Int32(0), aws.Int32(0), lambroll.ConcurrencyActionNone, false},
		{"remove", aws.Int32(-1), aws.Int32(10), lambroll.ConcurrencyActionDelete, false},
		{"remove zero", aws.Int32(-1), aws.Int32(0), lambroll.ConcurrencyActionDelete, false},
		{"remove not set", aws.Int32(-1), nil, lambroll.ConcurrencyActionNone, false},
		{"invalid", aws.Int32(-2), nil, lambroll.ConcurrencyActionNone, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			action, err := lambroll.PlanReservedConcurrency(c.local, c.current)
			if c.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if action != c.expect {
				t.Errorf("unexpected action %v, expected %v", action, c.expect)
			}
		})
	}
}

func TestNormalizeReservedConcurrencyForDiff(t *testing.T) {
	cases := []struct {
		name   string
		local  *int32
		remote *int32
		expect *int32
	}{
		{"not managed and not set", nil, nil, nil},
		{"managed", aws.Int32(5), aws.Int32(10), aws.Int32(10)},
		{"managed and not set", aws.Int32(5), nil, nil},
		{"remove and not set", aws.Int32(-1), nil, aws.Int32(-1)},
		{"remove and set", aws.Int32(-1), aws.Int32(10), aws.Int32(10)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			local := &lambroll.Function{ReservedConcurrentExecutions: c.local}
			remote := &lambroll.Function{ReservedConcurrentExecutions: c.remote}
			lambroll.NormalizeReservedConcurrencyForDiff(local, remote)
			got := remote.ReservedConcurrentExecutions
			if (got == nil) != (c.expect == nil) || (c.expect != nil && *got != *c.expect) {
				t.Errorf("unexpected remote ReservedConcurrentExecutions %v, expected %v", got, c.expect)
			}
		})
	}
	// must not panic when the remote function does not exist
	lambroll.NormalizeReservedConcurrencyForDiff(&lambroll.Function{ReservedConcurrentExecutions: aws.Int32(-1)}, nil)
}

func TestLoadFunctionReservedConcurrentExecutions(t *testing.T) {
	app, err := lambroll.New(context.Background(), &lambroll.Option{})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		src    string
		expect *int32
	}{
		{"not defined", `{"FunctionName":"test"}`, nil},
		{"zero", `{"FunctionName":"test","ReservedConcurrentExecutions":0}`, aws.Int32(0)},
		{"remove", `{"FunctionName":"test","ReservedConcurrentExecutions":-1}`, aws.Int32(-1)},
		{"value", `{"FunctionName":"test","ReservedConcurrentExecutions":10}`, aws.Int32(10)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "function.json")
			if err := os.WriteFile(path, []byte(c.src), 0644); err != nil {
				t.Fatal(err)
			}
			fn, err := app.LoadFunction(path)
			if err != nil {
				t.Fatal(err)
			}
			if aws.ToString(fn.FunctionName) != "test" {
				t.Errorf("unexpected FunctionName %v", fn.FunctionName)
			}
			if (fn.ReservedConcurrentExecutions == nil) != (c.expect == nil) ||
				(c.expect != nil && *fn.ReservedConcurrentExecutions != *c.expect) {
				t.Errorf("unexpected ReservedConcurrentExecutions %v, expected %v", fn.ReservedConcurrentExecutions, c.expect)
			}

			// marshalJSON omits empty values, but must keep 0 and -1
			b, err := lambroll.MarshalJSON(fn)
			if err != nil {
				t.Fatal(err)
			}
			has := strings.Contains(string(b), `"ReservedConcurrentExecutions"`)
			if has != (c.expect != nil) {
				t.Errorf("unexpected marshaled JSON %s", string(b))
			}
		})
	}
}
