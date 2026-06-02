package lambroll_test

import (
	"testing"

	"github.com/fujiwara/lambroll"
	"github.com/go-test/deep"
)

type tags = lambroll.Tags
type keys = []string
type tagsTestCase struct {
	oldTags    tags
	newTags    tags
	setTags    tags
	removeKeys keys
}

var mergeTagsCase = []tagsTestCase{
	{
		oldTags:    tags{"Foo": "FOO", "Bar": "BAR", "Tee": "TEE"},
		newTags:    tags{"Foo": "FOO", "Baz": "BAZ", "Tee": "TEEEE"},
		setTags:    tags{"Baz": "BAZ", "Tee": "TEEEE"},
		removeKeys: keys{"Bar"},
	},
	{
		oldTags:    tags{},
		newTags:    tags{"Foo": "FOO", "Bar": "BAR"},
		setTags:    tags{"Foo": "FOO", "Bar": "BAR"},
		removeKeys: keys{},
	},
	{
		oldTags:    tags{"Foo": "Foo", "Bar": "Bar"},
		newTags:    tags{"Bar": "Bar"},
		setTags:    tags{},
		removeKeys: keys{"Foo"},
	},
	{
		oldTags:    tags{"A": "A"},
		newTags:    tags{"A": "B"},
		setTags:    tags{"A": "B"},
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

var testTags = []struct {
	name         string
	isAWSManaged bool
}{
	{
		name:         "aws:cloudformation:stack-name",
		isAWSManaged: true,
	},
	{
		name:         "AWS:cloudformation:logical-id",
		isAWSManaged: true,
	},
	{
		name:         "foo:bar",
		isAWSManaged: false,
	},
}

func TestIsAWSManagedTags(t *testing.T) {
	for _, c := range testTags {
		if lambroll.IsAWSManagedTag(c.name) != c.isAWSManaged {
			t.Errorf("unexpected result for tag name %s", c.name)
		}
	}
}
