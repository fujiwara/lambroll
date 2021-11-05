package lambroll

import (
	"os"
	"testing"
)

func TestLoadFunction(t *testing.T) {
	os.Setenv("FUNCTION_NAME", "test")
	envfiles := []string{"test/env"}
	path := "test/terraform.tfstate"
	app, err := New(&Option{
		TFState: &path,
		Envfile: &envfiles,
		ExtStr: &map[string]string{
			"Description": "hello function",
		},
		ExtCode: &map[string]string{
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
		if *fn.Environment.Variables["JSON"] != `{"foo":"bar"}` {
			t.Errorf("unexpected environment %v", fn.Environment.Variables)
		}
		if *fn.VpcConfig.SecurityGroupIds[0] != "sg-01a9b01eab0a3c154" {
			t.Errorf("unexpected SecurityGroupIds %v", fn.VpcConfig.SecurityGroupIds)
		}
		arch := fn.Architectures
		if len(arch) != 2 || *arch[0] != "x86_64" || *arch[1] != "arm64" {
			t.Errorf("unexpected Architectures %v", fn.Architectures)
		}
		t.Log(fn.String())
	}
}
