package lambroll

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

func TestLoadFunction(t *testing.T) {
	os.Setenv("FUNCTION_NAME", "test")
	envfiles := []string{"test/env"}
	path := "test/terraform.tfstate"
	app, err := New(context.Background(), &Option{
		TFState: &path,
		PrefixedTFState: map[string]string{
			"prefix1_": "test/terraform_1.tfstate",
			"prefix2_": "test/terraform_2.tfstate",
		},
		Envfile: envfiles,
		ExtStr: map[string]string{
			"Description": "hello function",
		},
		ExtCode: map[string]string{
			"MemorySize": "64 * 2", // == 128
		},
	})
	if err != nil {
		t.Error(err)
	}
	for _, f := range []string{"test/function.json", "test/function.jsonnet"} {
		fn, err := app.loadFunction(f)
		if err != nil {
			t.Error(err)
		}
		if *fn.Role != "arn:aws:iam::123456789012:role/test_lambda_role" {
			t.Errorf("unexpected role got %s", *fn.Role)
		}
		if *fn.FunctionName != "test" {
			t.Errorf("unexpected function name got %s", *fn.FunctionName)
		}
		if *fn.FileSystemConfigs[0].Arn != "arn:aws:elasticfilesystem:ap-northeast-1:123456789012:access-point/fsap-04fc0858274e7dd9a" {
			t.Errorf("unexpected fileSystemConfigs %v", *&fn.FileSystemConfigs)
		}
		if fn.Environment.Variables["JSON"] != `{"foo":"bar"}` {
			t.Errorf("unexpected environment %v", fn.Environment.Variables)
		}
		if fn.Environment.Variables["PREFIXED_TFSTATE_1"] != "arn:aws:iam::123456789012:role/test_lambda_role_1" {
			t.Errorf("unexpected environment %v", fn.Environment.Variables)
		}
		if fn.Environment.Variables["PREFIXED_TFSTATE_2"] != "arn:aws:iam::123456789012:role/test_lambda_role_2" {
			t.Errorf("unexpected environment %v", fn.Environment.Variables)
		}
		if fn.VpcConfig.SecurityGroupIds[0] != "sg-01a9b01eab0a3c154" {
			t.Errorf("unexpected SecurityGroupIds %v", fn.VpcConfig.SecurityGroupIds)
		}
		arch := fn.Architectures
		if len(arch) != 1 || arch[0] != "x86_64" {
			t.Errorf("unexpected Architectures %v", fn.Architectures)
		}
		if *fn.LoggingConfig.LogGroup != "/aws/lambda/test/json" {
			t.Errorf("unexpected LoggingConfig %v", fn.LoggingConfig)
		}
		if *fn.EphemeralStorage.Size != 1024 {
			t.Errorf("unexpected EphemeralStorage %v", fn.EphemeralStorage)
		}
		t.Log(fn)
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
	fn := newFunctionFrom(conf, nil, tags)
	if *fn.FunctionName != "hello" {
		t.Errorf("unexpected function name got %s", *fn.FunctionName)
	}
	if *fn.MemorySize != 128 {
		t.Errorf("unexpected memory size got %d", *fn.MemorySize)
	}
	if fn.Runtime != types.RuntimeNodejs18x {
		t.Errorf("unexpected runtime got %s", fn.Runtime)
	}
	if *fn.Timeout != 3 {
		t.Errorf("unexpected timeout got %d", *fn.Timeout)
	}
	if *fn.Handler != "index.handler" {
		t.Errorf("unexpected handler got %s", *fn.Handler)
	}
	if *fn.Role != "arn:aws:iam::0123456789012:role/YOUR_LAMBDA_ROLE_NAME" {
		t.Errorf("unexpected role got %s", *fn.Role)
	}
	if fn.Tags["foo"] != "bar" {
		t.Errorf("unexpected tags got %v", fn.Tags)
	}
	if fn.SnapStart != nil {
		t.Errorf("unexpected snap start got %v", fn.SnapStart)
	}
}
