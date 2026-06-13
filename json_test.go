package lambroll

import (
	"encoding/json"
	"reflect"
	"testing"
)

func mustUnmarshal(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("invalid json %q: %v", s, err)
	}
	return v
}

// TestOmitEmptyValuesKeepsEnvVars verifies that empty string values inside
// Environment.Variables are preserved (an intentionally-empty environment
// variable is deployed to AWS as-is), while empty values elsewhere are dropped.
func TestOmitEmptyValuesKeepsEnvVars(t *testing.T) {
	in := mustUnmarshal(t, `{
		"FunctionName": "f",
		"Description": "",
		"Environment": {"Variables": {"FILLED": "x", "EMPTY": ""}}
	}`)
	got := omitEmptyValues(in)
	want := mustUnmarshal(t, `{
		"FunctionName": "f",
		"Environment": {"Variables": {"FILLED": "x", "EMPTY": ""}}
	}`)
	if !reflect.DeepEqual(got, want) {
		b, _ := json.Marshal(got)
		t.Errorf("omitEmptyValues = %s, want EMPTY env var kept and empty Description dropped", b)
	}
}

// TestOmitEmptyValuesAllEmptyEnvVars verifies that a Variables map containing
// only empty strings is kept (not collapsed away), so the env var stays visible.
func TestOmitEmptyValuesAllEmptyEnvVars(t *testing.T) {
	in := mustUnmarshal(t, `{"Environment": {"Variables": {"EMPTY": ""}}}`)
	got := omitEmptyValues(in)
	want := mustUnmarshal(t, `{"Environment": {"Variables": {"EMPTY": ""}}}`)
	if !reflect.DeepEqual(got, want) {
		b, _ := json.Marshal(got)
		t.Errorf("omitEmptyValues = %s, want the empty env var preserved", b)
	}
}

// TestOmitEmptyValuesEmptyStringOutsideEnvVars verifies the scoping: an empty
// string under a key named "Variables" that is NOT under Environment is still
// dropped, so the env var exception does not leak to unrelated fields.
func TestOmitEmptyValuesEmptyStringOutsideEnvVars(t *testing.T) {
	in := mustUnmarshal(t, `{"Other": {"Variables": {"K": ""}}}`)
	got := omitEmptyValues(in)
	if got != nil {
		b, _ := json.Marshal(got)
		t.Errorf("omitEmptyValues = %s, want empty string outside Environment.Variables dropped (nil)", b)
	}
}

// TestOmitEmptyValuesDropsOtherEmptiesInEnvVars verifies that within
// Environment.Variables only string values are kept; a null value (not a valid
// env var value, but defensive) is still dropped.
func TestOmitEmptyValuesDropsNullInEnvVars(t *testing.T) {
	in := mustUnmarshal(t, `{"Environment": {"Variables": {"NULL": null, "EMPTY": ""}}}`)
	got := omitEmptyValues(in)
	want := mustUnmarshal(t, `{"Environment": {"Variables": {"EMPTY": ""}}}`)
	if !reflect.DeepEqual(got, want) {
		b, _ := json.Marshal(got)
		t.Errorf("omitEmptyValues = %s, want null dropped and empty string kept", b)
	}
}
