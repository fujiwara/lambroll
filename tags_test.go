package lambroll_test

import (
	"testing"

	"github.com/fujiwara/lambroll"
	"github.com/go-test/deep"
)

type tags = map[string]*string
type keys = []*string
type tagsTestCase struct {
	oldTags    tags
	newTags    tags
	setTags    tags
	removeKeys keys
}

func s(s string) *string {
	return &s
}

var mergeTagsCase = []tagsTestCase{
	tagsTestCase{
		oldTags:    tags{"Foo": s("FOO"), "Bar": s("BAR"), "Tee": s("TEE")},
		newTags:    tags{"Foo": s("FOO"), "Baz": s("BAZ"), "Tee": s("TEEEE")},
		setTags:    tags{"Baz": s("BAZ"), "Tee": s("TEEEE")},
		removeKeys: keys{s("Bar")},
	},
	tagsTestCase{
		oldTags:    tags{},
		newTags:    tags{"Foo": s("FOO"), "Bar": s("BAR")},
		setTags:    tags{"Foo": s("FOO"), "Bar": s("BAR")},
		removeKeys: keys{},
	},
	tagsTestCase{
		oldTags:    tags{"Foo": s("Foo"), "Bar": s("Bar")},
		newTags:    tags{"Bar": s("Bar")},
		setTags:    tags{},
		removeKeys: keys{s("Foo")},
	},
	tagsTestCase{
		oldTags:    tags{"A": s("A")},
		newTags:    tags{"A": s("B")},
		setTags:    tags{"A": s("B")},
		removeKeys: keys{},
	},
}

func TestMergeTags(t *testing.T) {
	for i, c := range mergeTagsCase {
		set, remove := lambroll.MergeTags(c.oldTags, c.newTags)
		if diff := deep.Equal(c.setTags, set); diff != nil {
			t.Error(i, diff)
		}
		if diff := deep.Equal(c.removeKeys, remove); diff != nil {
			t.Error(i, diff)
		}
	}
}
