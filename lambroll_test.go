package lambroll_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/fujiwara/lambroll"
	"github.com/google/go-cmp/cmp"
)

var testCasesFillDefaultValues = []struct {
	name   string
	in     *lambroll.Function
	expect *lambroll.Function
}{
	{
		name: "normal",
		in:   &lambroll.Function{FunctionName: aws.String("test")},
		expect: &lambroll.Function{
			FunctionName:  aws.String("test"),
			Description:   aws.String(""),
			Architectures: []types.Architecture{types.ArchitectureX8664},
			EphemeralStorage: &types.EphemeralStorage{
				Size: aws.Int32(512),
			},
			Layers: []string{},
			LoggingConfig: &types.LoggingConfig{
				LogFormat: types.LogFormatText,
				LogGroup:  aws.String("/aws/lambda/test"),
			},
			MemorySize: aws.Int32(128),
			SnapStart: &types.SnapStart{
				ApplyOn: types.SnapStartApplyOnNone,
			},
			Timeout: aws.Int32(3),
			TracingConfig: &types.TracingConfig{
				Mode: types.TracingModePassThrough,
			},
		},
	},
	{
		name: "logging config JSON",
		in: &lambroll.Function{
			FunctionName: aws.String("test"),
			LoggingConfig: &types.LoggingConfig{
				LogFormat: types.LogFormatJson,
			},
		},
		expect: &lambroll.Function{
			FunctionName:  aws.String("test"),
			Description:   aws.String(""),
			Architectures: []types.Architecture{types.ArchitectureX8664},
			EphemeralStorage: &types.EphemeralStorage{
				Size: aws.Int32(512),
			},
			Layers: []string{},
			LoggingConfig: &types.LoggingConfig{
				ApplicationLogLevel: "INFO",
				LogFormat:           types.LogFormatJson,
				LogGroup:            aws.String("/aws/lambda/test"),
				SystemLogLevel:      "INFO",
			},
			MemorySize: aws.Int32(128),
			SnapStart: &types.SnapStart{
				ApplyOn: types.SnapStartApplyOnNone,
			},
			Timeout: aws.Int32(3),
			TracingConfig: &types.TracingConfig{
				Mode: types.TracingModePassThrough,
			},
		},
	},
	{
		name: "file system configs (S3 Files gets DirectS3Read AUTO, EFS untouched)",
		in: &lambroll.Function{
			FunctionName: aws.String("test"),
			FileSystemConfigs: []types.FileSystemConfig{
				{
					Arn:            aws.String("arn:aws:elasticfilesystem:ap-northeast-1:123456789012:access-point/fsap-04fc0858274e7dd9a"),
					LocalMountPath: aws.String("/mnt/efs"),
				},
				{
					Arn:            aws.String("arn:aws:s3files:ap-northeast-1:123456789012:file-system/fs-0a975615cfccfa09f/access-point/fsap-05b7f172fa3e59ee8"),
					LocalMountPath: aws.String("/mnt/data"),
				},
				{
					Arn:            aws.String("arn:aws:s3files:ap-northeast-1:123456789012:file-system/fs-0a975615cfccfa09f/access-point/fsap-0aaaaaaaaaaaaaaaa"),
					LocalMountPath: aws.String("/mnt/data2"),
					S3FilesConfig:  &types.S3FilesConfig{},
				},
				{
					Arn:            aws.String("arn:aws:s3files:ap-northeast-1:123456789012:file-system/fs-0a975615cfccfa09f/access-point/fsap-0bbbbbbbbbbbbbbbb"),
					LocalMountPath: aws.String("/mnt/data3"),
					S3FilesConfig:  &types.S3FilesConfig{DirectS3Read: types.DirectS3ReadEnabled},
				},
			},
		},
		expect: &lambroll.Function{
			FunctionName:  aws.String("test"),
			Description:   aws.String(""),
			Architectures: []types.Architecture{types.ArchitectureX8664},
			EphemeralStorage: &types.EphemeralStorage{
				Size: aws.Int32(512),
			},
			FileSystemConfigs: []types.FileSystemConfig{
				{
					Arn:            aws.String("arn:aws:elasticfilesystem:ap-northeast-1:123456789012:access-point/fsap-04fc0858274e7dd9a"),
					LocalMountPath: aws.String("/mnt/efs"),
				},
				{
					Arn:            aws.String("arn:aws:s3files:ap-northeast-1:123456789012:file-system/fs-0a975615cfccfa09f/access-point/fsap-05b7f172fa3e59ee8"),
					LocalMountPath: aws.String("/mnt/data"),
					S3FilesConfig:  &types.S3FilesConfig{DirectS3Read: types.DirectS3ReadAuto},
				},
				{
					Arn:            aws.String("arn:aws:s3files:ap-northeast-1:123456789012:file-system/fs-0a975615cfccfa09f/access-point/fsap-0aaaaaaaaaaaaaaaa"),
					LocalMountPath: aws.String("/mnt/data2"),
					S3FilesConfig:  &types.S3FilesConfig{DirectS3Read: types.DirectS3ReadAuto},
				},
				{
					Arn:            aws.String("arn:aws:s3files:ap-northeast-1:123456789012:file-system/fs-0a975615cfccfa09f/access-point/fsap-0bbbbbbbbbbbbbbbb"),
					LocalMountPath: aws.String("/mnt/data3"),
					S3FilesConfig:  &types.S3FilesConfig{DirectS3Read: types.DirectS3ReadEnabled},
				},
			},
			Layers: []string{},
			LoggingConfig: &types.LoggingConfig{
				LogFormat: types.LogFormatText,
				LogGroup:  aws.String("/aws/lambda/test"),
			},
			MemorySize: aws.Int32(128),
			SnapStart: &types.SnapStart{
				ApplyOn: types.SnapStartApplyOnNone,
			},
			Timeout: aws.Int32(3),
			TracingConfig: &types.TracingConfig{
				Mode: types.TracingModePassThrough,
			},
		},
	},
}

func TestFillDefaultValues(t *testing.T) {
	for _, tt := range testCasesFillDefaultValues {
		t.Run(tt.name, func(t *testing.T) {
			lambroll.FillDefaultValues(tt.in)
			in := lambroll.ToJSONString(tt.in)
			expect := lambroll.ToJSONString(tt.expect)
			if diff := cmp.Diff(in, expect); diff != "" {
				t.Errorf("differs: (-got +want)\n%s", diff)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
