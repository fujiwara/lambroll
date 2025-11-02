package lambroll_test

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/fujiwara/lambroll"
	"github.com/go-test/deep"
)

var permissionsTestCases = []struct {
	subject                string
	statementJSON          []byte
	expectedPrincipal      *string
	expectedPrincipalOrgID *string
	expectedSourceArn      *string
}{
	{
		subject: "AuthType NONE",
		statementJSON: []byte(`{
			"Action": "lambda:InvokeFunctionUrl",
			"Condition": {
				"StringEquals": {
					"lambda:FunctionUrlAuthType": "NONE"
				}
			},
			"Effect": "Allow",
			"Principal": "*",
			"Resource": "arn:aws:lambda:ap-northeast-1:123456789012:function:hello",
			"Sid": "lambroll-8f4ec83e623a309d9ca15db9276da30b2129be9c"
		}`),
		expectedPrincipal:      aws.String("*"),
		expectedPrincipalOrgID: nil,
	},
	{
		subject: "AuthType AWS_IAM with Principal OrgID",
		statementJSON: []byte(`{
			"Sid": "lambroll-622ed5c2bb0714ef0af1929fcea568e4ba0c4dbe",
			"Effect": "Allow",
			"Principal": "*",
			"Action": "lambda:InvokeFunctionUrl",
			"Resource": "arn:aws:lambda:ap-northeast-1:1234567890:function:hello",
			"Condition": {
				"StringEquals": {
					"lambda:FunctionUrlAuthType": "AWS_IAM",
					"aws:PrincipalOrgID": "o-xxxxxxxxxx"
				}
			}
		}`),
		expectedPrincipal:      aws.String("*"),
		expectedPrincipalOrgID: aws.String("o-xxxxxxxxxx"),
	},
	{
		subject: "AuthType AWS_IAM with Principal",
		statementJSON: []byte(`{
			"Action": "lambda:InvokeFunctionUrl",
			"Condition": {
				"StringEquals": {
					"lambda:FunctionUrlAuthType": "AWS_IAM"
				}
			},
			"Effect": "Allow",
			"Principal": {
				"AWS": "arn:aws:iam::123456789012:root"
			},
			"Resource": "arn:aws:lambda:ap-northeast-1:123456789012:function:hello",
			"Sid": "lambroll-3b135eca4b14335775cda9f947966093a57d270f"
		}`),
		expectedPrincipal:      aws.String("123456789012"),
		expectedPrincipalOrgID: nil,
	},
	{
		subject: "AuthType AWS_IAM with Principal CF OAC",
		statementJSON: []byte(`{
			"Action": "lambda:InvokeFunctionUrl",
			"Condition": {
				"ArnLike": {
					"aws:SourceArn": "arn:aws:cloudfront::123456789012:distribution/ABCDEFG12345678"
				}
			},
			"Effect": "Allow",
			"Principal": {
				"Service": "cloudfront.amazonaws.com"
			},
			"Resource": "arn:aws:lambda:ap-northeast-1:123456789012:function:hello",
			"Sid": "lambroll-3b135eca4b14335775cda9f947966093a57d270f"
		}`),
		expectedPrincipal:      aws.String("cloudfront.amazonaws.com"),
		expectedPrincipalOrgID: nil,
		expectedSourceArn:      aws.String("arn:aws:cloudfront::123456789012:distribution/ABCDEFG12345678"),
	},
}

func TestParseStatement(t *testing.T) {
	for _, c := range permissionsTestCases {
		st := &lambroll.PolicyStatement{}
		if err := json.Unmarshal(c.statementJSON, st); err != nil {
			t.Errorf("%s failed to unmarshal json: %s", c.subject, err)
		}
		if diff := deep.Equal(c.expectedPrincipal, st.PrincipalString()); diff != nil {
			t.Errorf("%s PrincipalString diff %s", c.subject, diff)
		}
		if diff := deep.Equal(c.expectedPrincipalOrgID, st.PrincipalOrgID()); diff != nil {
			t.Errorf("%s PrincipalOrgID diff %s", c.subject, diff)
		}
		if diff := deep.Equal(c.expectedSourceArn, st.SourceArn()); diff != nil {
			t.Errorf("%s SourceArn diff %s", c.subject, diff)
		}
	}
}

func TestFunctionURLPermission_Equals(t *testing.T) {
	tests := []struct {
		name string
		p1   *lambroll.FunctionURLPermission
		p2   *lambroll.FunctionURLPermission
		want bool
	}{
		{
			name: "identical permissions",
			p1: &lambroll.FunctionURLPermission{
				Principal:      aws.String("*"),
				PrincipalOrgID: aws.String("o-xxxxxxxxxx"),
				SourceArn:      aws.String("arn:aws:cloudfront::123456789012:distribution/ABCDEFG12345678"),
				SourceAccount:  aws.String("123456789012"),
			},
			p2: &lambroll.FunctionURLPermission{
				Principal:      aws.String("*"),
				PrincipalOrgID: aws.String("o-xxxxxxxxxx"),
				SourceArn:      aws.String("arn:aws:cloudfront::123456789012:distribution/ABCDEFG12345678"),
				SourceAccount:  aws.String("123456789012"),
			},
			want: true,
		},
		{
			name: "different principal",
			p1: &lambroll.FunctionURLPermission{
				Principal: aws.String("*"),
			},
			p2: &lambroll.FunctionURLPermission{
				Principal: aws.String("123456789012"),
			},
			want: false,
		},
		{
			name: "different principal org ID",
			p1: &lambroll.FunctionURLPermission{
				Principal:      aws.String("*"),
				PrincipalOrgID: aws.String("o-aaaaaaaaaa"),
			},
			p2: &lambroll.FunctionURLPermission{
				Principal:      aws.String("*"),
				PrincipalOrgID: aws.String("o-bbbbbbbbbb"),
			},
			want: false,
		},
		{
			name: "different source arn",
			p1: &lambroll.FunctionURLPermission{
				Principal: aws.String("*"),
				SourceArn: aws.String("arn:aws:cloudfront::123456789012:distribution/AAA"),
			},
			p2: &lambroll.FunctionURLPermission{
				Principal: aws.String("*"),
				SourceArn: aws.String("arn:aws:cloudfront::123456789012:distribution/BBB"),
			},
			want: false,
		},
		{
			name: "nil vs empty string principal",
			p1: &lambroll.FunctionURLPermission{
				Principal: nil,
			},
			p2: &lambroll.FunctionURLPermission{
				Principal: aws.String(""),
			},
			want: true,
		},
		{
			name: "both nil principals",
			p1: &lambroll.FunctionURLPermission{
				Principal: nil,
			},
			p2: &lambroll.FunctionURLPermission{
				Principal: nil,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p1.Equals(tt.p2)
			if got != tt.want {
				t.Errorf("Equals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionURLPermission_AddPermissionInputs(t *testing.T) {
	fc := &lambroll.FunctionURL{
		Config: &lambroll.FunctionURLConfig{
			FunctionName: aws.String("test-function"),
			Qualifier:    aws.String("1"),
			AuthType:     "AWS_IAM",
		},
	}

	tests := []struct {
		name       string
		permission *lambroll.FunctionURLPermission
		validate   func(t *testing.T, perms []*lambda.AddPermissionInput)
	}{
		{
			name: "basic permission without actualSids",
			permission: &lambroll.FunctionURLPermission{
				Principal:      aws.String("*"),
				PrincipalOrgID: aws.String("o-xxxxxxxxxx"),
			},
			validate: func(t *testing.T, perms []*lambda.AddPermissionInput) {
				// Should return 2 permissions
				if len(perms) != 2 {
					t.Errorf("expected 2 permissions, got %d", len(perms))
					return
				}

				// First permission: InvokeFunctionUrl
				p1 := perms[0]
				if aws.ToString(p1.Action) != "lambda:InvokeFunctionUrl" {
					t.Errorf("first permission action = %v, want lambda:InvokeFunctionUrl", aws.ToString(p1.Action))
				}
				if aws.ToString(p1.FunctionName) != "test-function" {
					t.Errorf("first permission FunctionName = %v, want test-function", aws.ToString(p1.FunctionName))
				}
				if aws.ToString(p1.Qualifier) != "1" {
					t.Errorf("first permission Qualifier = %v, want 1", aws.ToString(p1.Qualifier))
				}
				if aws.ToString(p1.Principal) != "*" {
					t.Errorf("first permission Principal = %v, want *", aws.ToString(p1.Principal))
				}
				if aws.ToString(p1.PrincipalOrgID) != "o-xxxxxxxxxx" {
					t.Errorf("first permission PrincipalOrgID = %v, want o-xxxxxxxxxx", aws.ToString(p1.PrincipalOrgID))
				}
				if p1.FunctionUrlAuthType != "AWS_IAM" {
					t.Errorf("first permission FunctionUrlAuthType = %v, want AWS_IAM", p1.FunctionUrlAuthType)
				}
				if p1.StatementId == nil {
					t.Error("first permission StatementId is nil")
				}

				// Second permission: InvokeFunction
				p2 := perms[1]
				if aws.ToString(p2.Action) != "lambda:InvokeFunction" {
					t.Errorf("second permission action = %v, want lambda:InvokeFunction", aws.ToString(p2.Action))
				}
				if aws.ToString(p2.FunctionName) != "test-function" {
					t.Errorf("second permission FunctionName = %v, want test-function", aws.ToString(p2.FunctionName))
				}
				if aws.ToString(p2.Qualifier) != "1" {
					t.Errorf("second permission Qualifier = %v, want 1", aws.ToString(p2.Qualifier))
				}
				if aws.ToString(p2.Principal) != "*" {
					t.Errorf("second permission Principal = %v, want *", aws.ToString(p2.Principal))
				}
				if !aws.ToBool(p2.InvokedViaFunctionUrl) {
					t.Error("second permission InvokedViaFunctionUrl should be true")
				}
				if p2.StatementId == nil {
					t.Error("second permission StatementId is nil")
				}

				// StatementIds should be different
				if aws.ToString(p1.StatementId) == aws.ToString(p2.StatementId) {
					t.Errorf("StatementIds should be different, both are %v", aws.ToString(p1.StatementId))
				}
			},
		},
		{
			name: "permission with actualSids",
			permission: &lambroll.FunctionURLPermission{
				Principal: aws.String("*"),
				// actualSids would be set internally
			},
			validate: func(t *testing.T, perms []*lambda.AddPermissionInput) {
				if len(perms) != 2 {
					t.Errorf("expected 2 permissions, got %d", len(perms))
					return
				}

				// Both should have auto-generated StatementIds
				if perms[0].StatementId == nil || perms[1].StatementId == nil {
					t.Error("StatementIds should not be nil")
				}
			},
		},
		{
			name: "permission with SourceArn and SourceAccount",
			permission: &lambroll.FunctionURLPermission{
				Principal:     aws.String("cloudfront.amazonaws.com"),
				SourceArn:     aws.String("arn:aws:cloudfront::123456789012:distribution/ABCDEFG"),
				SourceAccount: aws.String("123456789012"),
			},
			validate: func(t *testing.T, perms []*lambda.AddPermissionInput) {
				if len(perms) != 2 {
					t.Errorf("expected 2 permissions, got %d", len(perms))
					return
				}

				for i, p := range perms {
					if aws.ToString(p.SourceArn) != "arn:aws:cloudfront::123456789012:distribution/ABCDEFG" {
						t.Errorf("permission[%d] SourceArn = %v, want arn:aws:cloudfront::123456789012:distribution/ABCDEFG",
							i, aws.ToString(p.SourceArn))
					}
					if aws.ToString(p.SourceAccount) != "123456789012" {
						t.Errorf("permission[%d] SourceAccount = %v, want 123456789012",
							i, aws.ToString(p.SourceAccount))
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perms := tt.permission.AddPermissionInputs(fc)
			tt.validate(t, perms)
		})
	}
}

func TestFunctionURLPermission_AddPermissionInputs_WithActualSids(t *testing.T) {
	fc := &lambroll.FunctionURL{
		Config: &lambroll.FunctionURLConfig{
			FunctionName: aws.String("test-function"),
			Qualifier:    aws.String("1"),
			AuthType:     "AWS_IAM",
		},
	}

	// Create permission (actualSids would be set internally when loading from remote)
	// Since actualSids is a private field, we test that StatementIds are deterministic
	// by generating permissions multiple times and verifying the SIDs match
	permission := &lambroll.FunctionURLPermission{
		Principal: aws.String("*"),
	}

	perms1 := permission.AddPermissionInputs(fc)
	perms2 := permission.AddPermissionInputs(fc)

	if len(perms1) != 2 || len(perms2) != 2 {
		t.Fatalf("expected 2 permissions in each call, got %d and %d", len(perms1), len(perms2))
	}

	// StatementIds should be deterministic (same permission content = same SID)
	for i := 0; i < 2; i++ {
		if aws.ToString(perms1[i].StatementId) != aws.ToString(perms2[i].StatementId) {
			t.Errorf("StatementId[%d] not deterministic: %v != %v",
				i, aws.ToString(perms1[i].StatementId), aws.ToString(perms2[i].StatementId))
		}
	}

	// Verify StatementIds follow the expected format (lambroll-<sha1>)
	for i, p := range perms1 {
		sid := aws.ToString(p.StatementId)
		if len(sid) != 49 { // "lambroll-" (9) + 40 hex chars
			t.Errorf("permission[%d] StatementId length = %d, want 49 (lambroll- prefix + 40 char hash)",
				i, len(sid))
		}
		if sid[:9] != "lambroll-" {
			t.Errorf("permission[%d] StatementId should start with 'lambroll-', got %s", i, sid[:9])
		}
	}
}
