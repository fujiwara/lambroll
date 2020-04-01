package lambroll

import (
	"os"
	"testing"
)

func TestLoadFunction(t *testing.T) {
	os.Setenv("FUNCTION_NAME", "test")
	path := "test/terraform.tfstate"
	app, err := New(&Option{TFState: &path})
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
	t.Log(fn.String())
}
