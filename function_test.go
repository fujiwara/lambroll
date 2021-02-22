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
	})
	if err != nil {
		t.Error(err)
	}
	fn, err := app.loadFunction("test/function.json")
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
	t.Log(fn.String())
}
