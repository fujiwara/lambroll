package lambroll_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/fujiwara/lambroll"
	"github.com/google/go-cmp/cmp"
)

func TestSortFunctionForDiff(t *testing.T) {
	fn := &lambroll.Function{
		VpcConfig: &types.VpcConfig{
			SubnetIds: []string{
				"subnet-08dc9a51660120991",
				"subnet-023e96b860485e2ad",
				"subnet-045cd24ab8e92a20d",
			},
			SecurityGroupIds: []string{
				"sg-99999999",
				"sg-11111111",
			},
		},
	}
	lambroll.SortFunctionForDiff(fn)

	wantSubnets := []string{
		"subnet-023e96b860485e2ad",
		"subnet-045cd24ab8e92a20d",
		"subnet-08dc9a51660120991",
	}
	if diff := cmp.Diff(wantSubnets, fn.VpcConfig.SubnetIds); diff != "" {
		t.Errorf("subnet ids not sorted (-want +got):\n%s", diff)
	}

	wantSGs := []string{"sg-11111111", "sg-99999999"}
	if diff := cmp.Diff(wantSGs, fn.VpcConfig.SecurityGroupIds); diff != "" {
		t.Errorf("security group ids not sorted (-want +got):\n%s", diff)
	}
}

func TestSortFunctionForDiffNil(t *testing.T) {
	// must not panic on nil function or nil VpcConfig
	lambroll.SortFunctionForDiff(nil)
	lambroll.SortFunctionForDiff(&lambroll.Function{})
}
