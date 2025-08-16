package lambroll_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/fujiwara/lambroll"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var ignore = cmpopts.IgnoreUnexported(
	types.EphemeralStorage{},
	types.Environment{},
	types.FileSystemConfig{},
	types.LoggingConfig{},
	types.TracingConfig{},
	types.VpcConfig{},
	lambda.CreateFunctionOutput{},
)

func TestLoadFunction(t *testing.T) {
	t.Setenv("FUNCTION_NAME", "test")
	app, err := lambroll.New(context.Background(), &lambroll.Option{
		TFState: aws.String("test/terraform.tfstate"),
		PrefixedTFState: map[string]string{
			"prefix1_": "test/terraform_1.tfstate",
			"prefix2_": "test/terraform_2.tfstate",
		},
		Envfile: []string{"test/env"},
		ExtStr: map[string]string{
			"description":    "hello function",
			"architecture":   "x86_64",
		},
		ExtCode: map[string]string{
			"memory_size":    "64 * 2", // == 128
			"storage_size":   "1024",
			"timeout":        "5",
		},
	})
	if err != nil {
		t.Error(err)
	}
	app.CallerIdentity().Resolver = func(_ context.Context) (*sts.GetCallerIdentityOutput, error) {
		return &sts.GetCallerIdentityOutput{
			Account: aws.String("123456789012"),
			Arn:     aws.String("arn:aws:iam::123456789012:user/test-user"),
			UserId:  aws.String("AIXXXXXXXXXXXXXXXXXX"),
		}, nil
	}
	expected := lambroll.Function{
		Architectures: []types.Architecture{types.ArchitectureX8664},
		Description:   aws.String("hello function"),
		Environment: &types.Environment{
			Variables: map[string]string{
				"PREFIXED_TFSTATE_1": "arn:aws:iam::123456789012:role/test_lambda_role_1",
				"PREFIXED_TFSTATE_2": "arn:aws:iam::123456789012:role/test_lambda_role_2",
				"JSON":               `{"foo":"bar"}`,
			},
		},
		EphemeralStorage: &types.EphemeralStorage{
			Size: aws.Int32(1024),
		},
		FileSystemConfigs: []types.FileSystemConfig{
			{
				Arn:            aws.String("arn:aws:elasticfilesystem:ap-northeast-1:123456789012:access-point/fsap-04fc0858274e7dd9a"),
				LocalMountPath: aws.String("/mnt/lambda"),
			},
		},
		FunctionName: aws.String("test"),
		Handler:      aws.String("index.js"),
		LoggingConfig: &types.LoggingConfig{
			ApplicationLogLevel: "DEBUG",
			LogGroup:            aws.String("/aws/lambda/test/json"),
			SystemLogLevel:      "INFO",
			LogFormat:           types.LogFormatJson,
		},
		MemorySize: aws.Int32(128),
		Runtime:    types.RuntimeNodejs16x,
		Role:       aws.String("arn:aws:iam::123456789012:role/test_lambda_role"),
		Timeout:    aws.Int32(5),
		TracingConfig: &types.TracingConfig{
			Mode: types.TracingModePassThrough,
		},
		VpcConfig: &types.VpcConfig{
			SubnetIds: []string{
				"subnet-08dc9a51660120991",
				"subnet-023e96b860485e2ad",
				"subnet-045cd24ab8e92a20d",
			},
			SecurityGroupIds: []string{
				"sg-01a9b01eab0a3c154",
			},
		},
	}

	for _, f := range []string{"test/function.json", "test/function.jsonnet"} {
		fn, err := app.LoadFunction(f)
		if err != nil {
			t.Error(err)
		}
		expectedJSON, _ := lambroll.MarshalJSON(expected)
		fnJSON, _ := lambroll.MarshalJSON(fn)
		if diff := cmp.Diff(string(expectedJSON), string(fnJSON), ignore); diff != "" {
			t.Errorf("%s: unexpected function got %s", f, diff)
		}
	}
}

func TestNewFunction(t *testing.T) {
	conf := &types.FunctionConfiguration{
		FunctionName: aws.String("hello"),
		MemorySize:   aws.Int32(128),
		Runtime:      types.RuntimeNodejs18x,
		Timeout:      aws.Int32(3),
		Handler:      aws.String("index.handler"),
		Role:         aws.String("arn:aws:iam::0123456789012:role/YOUR_LAMBDA_ROLE_NAME"),
	}
	tags := map[string]string{
		"foo": "bar",
	}
	fn := lambroll.NewFunctionFrom(conf, nil, tags)

	expected := lambroll.Function{
		FunctionName: aws.String("hello"),
		MemorySize:   aws.Int32(128),
		Runtime:      types.RuntimeNodejs18x,
		Timeout:      aws.Int32(3),
		Handler:      aws.String("index.handler"),
		Role:         aws.String("arn:aws:iam::0123456789012:role/YOUR_LAMBDA_ROLE_NAME"),
		Tags:         tags,
	}

	fnJSON, _ := lambroll.MarshalJSON(fn)
	expectedJSON, _ := lambroll.MarshalJSON(expected)
	if diff := cmp.Diff(string(expectedJSON), string(fnJSON), ignore); diff != "" {
		t.Errorf("unexpected function got %s", diff)
	}
}
