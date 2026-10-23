package lambroll_test

import (
	"testing"

	"github.com/fujiwara/lambroll"
)

func TestValidateReservedConcurrency(t *testing.T) {
	for _, tc := range []struct {
		value   *int32
		wantErr bool
	}{
		{value: nil},
		{value: new(int32(0))},
		{value: new(int32(10))},
		{value: new(int32(-1))},
		{value: new(int32(-2)), wantErr: true},
	} {
		err := lambroll.ValidateReservedConcurrency(tc.value)
		if (err != nil) != tc.wantErr {
			t.Errorf("value %s: unexpected error %v", lambroll.FormatReservedConcurrency(tc.value), err)
		}
	}
}

func TestPlanReservedConcurrency(t *testing.T) {
	for _, tc := range []struct {
		name             string
		desired, current *int32
		want             lambroll.ConcurrencyAction
	}{
		{name: "not specified", desired: nil, current: new(int32(10)), want: lambroll.ConcurrencyActionNone},
		{name: "set", desired: new(int32(10)), current: nil, want: lambroll.ConcurrencyActionPut},
		{name: "change", desired: new(int32(10)), current: new(int32(5)), want: lambroll.ConcurrencyActionPut},
		{name: "unchanged", desired: new(int32(10)), current: new(int32(10)), want: lambroll.ConcurrencyActionNone},
		{name: "throttle", desired: new(int32(0)), current: nil, want: lambroll.ConcurrencyActionPut},
		{name: "remove", desired: new(int32(-1)), current: new(int32(10)), want: lambroll.ConcurrencyActionDelete},
		{name: "remove already removed", desired: new(int32(-1)), current: nil, want: lambroll.ConcurrencyActionNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := lambroll.PlanReservedConcurrency(tc.desired, tc.current); got != tc.want {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
