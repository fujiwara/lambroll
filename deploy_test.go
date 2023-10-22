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
	excludes := []string{}
	ex, err := lambroll.ExpandExcludeFile(file)
	if err != nil {
		t.Error("failed to expand", err)
	}
	excludes = append(excludes, ex...)
	if len(excludes) != len(expectExcludes) {
		t.Errorf("unexpeted expanded excludes %#v", excludes)
	}
	for i, line := range expectExcludes {
		if line != excludes[i] {
			t.Errorf("unexpected expanded excludes[%d] expected:%s, got:%s", i, line, excludes[i])
		}
	}
}
