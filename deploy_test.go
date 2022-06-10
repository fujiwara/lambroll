package lambroll_test

import (
	"testing"

	"github.com/fujiwara/lambroll"
)

var expectExcludes = []string{
	"*.bin",
	"*.zip",
}

func TestDeployOptionExpand(t *testing.T) {
	file := "test/src/.lambdaignore"
	opt := lambroll.DeployOption{
		ExcludeFile: &file,
	}
	excludes, err := lambroll.ExpandExcludeFile(*opt.ExcludeFile)
	if err != nil {
		t.Error("failed to expand", err)
	}
	opt.Excludes = append(opt.Excludes, excludes...)
	if len(opt.Excludes) != len(expectExcludes) {
		t.Errorf("unexpeted expanded excludes %#v", opt.Excludes)
	}
	for i, line := range expectExcludes {
		if line != opt.Excludes[i] {
			t.Errorf("unexpected expanded excludes[%d] expected:%s, got:%s", i, line, opt.Excludes[i])
		}
	}
}
