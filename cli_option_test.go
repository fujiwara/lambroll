package lambroll_test

import (
	"encoding/json"
	"testing"

	"github.com/fujiwara/lambroll"
	"github.com/google/go-cmp/cmp"
)

func TestOptionUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		want     lambroll.Option
	}{
		{
			name: "new format with ext_str and ext_code",
			jsonData: `{
				"ext_str": {"key1": "value1", "key2": "value2"},
				"ext_code": {"code1": "1+1", "code2": "2*2"}
			}`,
			want: lambroll.Option{
				ExtStr:  map[string]string{"key1": "value1", "key2": "value2"},
				ExtCode: map[string]string{"code1": "1+1", "code2": "2*2"},
			},
		},
		{
			name: "legacy format with extstr and extcode",
			jsonData: `{
				"extstr": {"key1": "value1", "key2": "value2"},
				"extcode": {"code1": "1+1", "code2": "2*2"}
			}`,
			want: lambroll.Option{
				ExtStr:  map[string]string{"key1": "value1", "key2": "value2"},
				ExtCode: map[string]string{"code1": "1+1", "code2": "2*2"},
			},
		},
		{
			name: "mixed format (new format takes precedence)",
			jsonData: `{
				"ext_str": {"key1": "new_value"},
				"extstr": {"key1": "old_value"},
				"ext_code": {"code1": "new_code"},
				"extcode": {"code1": "old_code"}
			}`,
			want: lambroll.Option{
				ExtStr:  map[string]string{"key1": "new_value"},
				ExtCode: map[string]string{"code1": "new_code"},
			},
		},
		{
			name:     "empty fields",
			jsonData: `{}`,
			want:     lambroll.Option{},
		},
		{
			name: "with other fields",
			jsonData: `{
				"region": "us-west-2",
				"profile": "default",
				"extstr": {"key": "value"},
				"log_level": "debug"
			}`,
			want: lambroll.Option{
				ExtStr: map[string]string{"key": "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got lambroll.Option
			err := json.Unmarshal([]byte(tt.jsonData), &got)
			if err != nil {
				t.Fatalf("UnmarshalJSON error = %v", err)
			}

			// Compare only the ExtStr and ExtCode fields
			if diff := cmp.Diff(tt.want.ExtStr, got.ExtStr); diff != "" {
				t.Errorf("ExtStr mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.want.ExtCode, got.ExtCode); diff != "" {
				t.Errorf("ExtCode mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestOptionUnmarshalJSONDiffMask(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		wantMask []string
		wantExt  map[string]string
	}{
		{
			name:     "diff.mask with mixed forms",
			jsonData: `{"diff": {"mask": ["DB_PASSWORD", ".Environment.Variables[\"API_KEY\"]"]}}`,
			wantMask: []string{"DB_PASSWORD", `.Environment.Variables["API_KEY"]`},
		},
		{
			name:     "absent diff key leaves mask nil",
			jsonData: `{"region": "us-west-2"}`,
			wantMask: nil,
		},
		{
			name:     "diff.mask coexists with legacy extstr",
			jsonData: `{"extstr": {"k": "v"}, "diff": {"mask": ["X"]}}`,
			wantMask: []string{"X"},
			wantExt:  map[string]string{"k": "v"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got lambroll.Option
			if err := json.Unmarshal([]byte(tt.jsonData), &got); err != nil {
				t.Fatalf("UnmarshalJSON error = %v", err)
			}
			if diff := cmp.Diff(tt.wantMask, got.Diff.Mask); diff != "" {
				t.Errorf("Diff.Mask mismatch (-want +got):\n%s", diff)
			}
			if tt.wantExt != nil {
				if diff := cmp.Diff(tt.wantExt, got.ExtStr); diff != "" {
					t.Errorf("ExtStr back-compat disturbed (-want +got):\n%s", diff)
				}
			}
		})
	}
}
