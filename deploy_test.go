package lambroll_test

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
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

func TestNewUpdateFunctionCodeInput(t *testing.T) {
	t.Run("S3 code with storage mode REFERENCE", func(t *testing.T) {
		src := `{
			"FunctionName": "test",
			"Code": {
				"S3Bucket": "my-bucket",
				"S3Key": "function.zip",
				"S3ObjectStorageMode": "REFERENCE",
				"SourceKMSKeyArn": "arn:aws:kms:ap-northeast-1:123456789012:key/dummy"
			}
		}`
		var fn lambroll.Function
		if err := json.Unmarshal([]byte(src), &fn); err != nil {
			t.Fatal("failed to unmarshal function", err)
		}
		in := lambroll.NewUpdateFunctionCodeInput(&fn)
		if aws.ToString(in.S3Bucket) != "my-bucket" {
			t.Errorf("unexpected S3Bucket %s", aws.ToString(in.S3Bucket))
		}
		if aws.ToString(in.S3Key) != "function.zip" {
			t.Errorf("unexpected S3Key %s", aws.ToString(in.S3Key))
		}
		if in.S3ObjectStorageMode != types.S3ObjectStorageModeReference {
			t.Errorf("unexpected S3ObjectStorageMode %s", in.S3ObjectStorageMode)
		}
		if aws.ToString(in.SourceKMSKeyArn) != "arn:aws:kms:ap-northeast-1:123456789012:key/dummy" {
			t.Errorf("unexpected SourceKMSKeyArn %s", aws.ToString(in.SourceKMSKeyArn))
		}
	})

	t.Run("S3 code without storage mode", func(t *testing.T) {
		fn := lambroll.Function{
			FunctionName: aws.String("test"),
			Code: &types.FunctionCode{
				S3Bucket: aws.String("my-bucket"),
				S3Key:    aws.String("function.zip"),
			},
		}
		in := lambroll.NewUpdateFunctionCodeInput(&fn)
		if in.S3ObjectStorageMode != "" {
			t.Errorf("S3ObjectStorageMode must be empty, got %s", in.S3ObjectStorageMode)
		}
	})
}
