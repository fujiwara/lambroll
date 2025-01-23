package lambroll_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/fujiwara/lambroll"
	"github.com/google/go-cmp/cmp"
)

var testCasesFillDefaultValues = []struct {
	name   string
	in     *lambroll.Function
	expect *lambroll.Function
}{
	{
		name: "normal",
		in:   &lambroll.Function{FunctionName: aws.String("test")},
		expect: &lambroll.Function{
			FunctionName:  aws.String("test"),
			Description:   aws.String(""),
			Architectures: []types.Architecture{types.ArchitectureX8664},
			EphemeralStorage: &types.EphemeralStorage{
				Size: aws.Int32(512),
			},
			Layers: []string{},
			LoggingConfig: &types.LoggingConfig{
				LogFormat: types.LogFormatText,
				LogGroup:  aws.String("/aws/lambda/test"),
			},
			MemorySize: aws.Int32(128),
			SnapStart: &types.SnapStart{
				ApplyOn: types.SnapStartApplyOnNone,
			},
			Timeout: aws.Int32(3),
			TracingConfig: &types.TracingConfig{
				Mode: types.TracingModePassThrough,
			},
		},
	},
	{
		name: "logging config JSON",
		in: &lambroll.Function{
			FunctionName: aws.String("test"),
			LoggingConfig: &types.LoggingConfig{
				LogFormat: types.LogFormatJson,
			},
		},
		expect: &lambroll.Function{
			FunctionName:  aws.String("test"),
			Description:   aws.String(""),
			Architectures: []types.Architecture{types.ArchitectureX8664},
			EphemeralStorage: &types.EphemeralStorage{
				Size: aws.Int32(512),
			},
			Layers: []string{},
			LoggingConfig: &types.LoggingConfig{
				ApplicationLogLevel: "INFO",
				LogFormat:           types.LogFormatJson,
				LogGroup:            aws.String("/aws/lambda/test"),
				SystemLogLevel:      "INFO",
			},
			MemorySize: aws.Int32(128),
			SnapStart: &types.SnapStart{
				ApplyOn: types.SnapStartApplyOnNone,
			},
			Timeout: aws.Int32(3),
			TracingConfig: &types.TracingConfig{
				Mode: types.TracingModePassThrough,
			},
		},
	},
}

func TestFillDefaultValues(t *testing.T) {
	for _, tt := range testCasesFillDefaultValues {
		t.Run(tt.name, func(t *testing.T) {
			lambroll.FillDefaultValues(tt.in)
			in := lambroll.ToJSONString(tt.in)
			expect := lambroll.ToJSONString(tt.expect)
			if diff := cmp.Diff(in, expect); diff != "" {
				t.Errorf("differs: (-got +want)\n%s", diff)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
