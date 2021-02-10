package appspec_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/fujiwara/lambroll/appspec"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/go-config"
)

var expected = &appspec.AppSpec{
	Version: aws.String("0.0"),
	Resources: []map[string]*appspec.Resource{
		{
			"myLambdaFunction": {
				Type: aws.String("AWS::Lambda::Function"),
				Properties: &appspec.Properties{
					Name:           aws.String("myLambdaFunction"),
					Alias:          aws.String("myLambdaFunctionAlias"),
					CurrentVersion: aws.String("1"),
					TargetVersion:  aws.String("2"),
				},
			},
		},
	},
	Hooks: []*appspec.Hook{
		{BeforeAllowTraffic: "LambdaFunctionToValidateBeforeTrafficShift"},
		{AfterAllowTraffic: "LambdaFunctionToValidateAfterTrafficShift"},
	},
}

func TestAppSpec(t *testing.T) {
	var s appspec.AppSpec
	err := config.LoadWithEnv(&s, "test.yml")
	if err != nil {
		t.Error(err)
	}
	t.Log(s.String())
	if diff := cmp.Diff(&s, expected); diff != "" {
		t.Error(diff)
	}
}
