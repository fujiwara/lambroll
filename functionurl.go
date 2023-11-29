package lambroll

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/samber/lo"
)

var (
	SidPattern = regexp.MustCompile("^lambroll-[0-9a-f]+$")
	SidFormat  = "lambroll-%x"
)

type FunctionURL struct {
	Config      *FunctionURLConfig     `json:"Config"`
	Permissions FunctionURLPermissions `json:"Permissions"`
}

func (f *FunctionURL) Validate(functionName string) error {
	if f.Config == nil {
		return errors.New("function url 'Config' attribute is required")
	}
	f.Config.FunctionName = aws.String(functionName)
	// fill default values
	switch f.Config.AuthType {
	case types.FunctionUrlAuthTypeNone:
		if len(f.Permissions) == 0 {
			f.Permissions = append(f.Permissions, &FunctionURLPermission{
				AddPermissionInput: lambda.AddPermissionInput{
					Principal: aws.String("*"),
				},
			})
		}
	case types.FunctionUrlAuthTypeAwsIam:
		if len(f.Permissions) == 0 {
			return fmt.Errorf("function url 'Permissions' attribute is required when 'AuthType' is '%s'", types.FunctionUrlAuthTypeAwsIam)
		}
	default:
		return fmt.Errorf("unknown function url 'AuthType': %s", f.Config.AuthType)
	}
	return nil
}

type FunctionURLConfig = lambda.CreateFunctionUrlConfigInput

type FunctionURLPermissions []*FunctionURLPermission

func (ps FunctionURLPermissions) Sids() []string {
	sids := make([]string, 0, len(ps))
	for _, p := range ps {
		sids = append(sids, p.Sid())
	}
	sort.Strings(sids)
	return sids
}

func (ps FunctionURLPermissions) Find(sid string) *FunctionURLPermission {
	for _, p := range ps {
		if p.Sid() == sid {
			return p
		}
	}
	return nil
}

type FunctionURLPermission struct {
	lambda.AddPermissionInput

	sid  string
	once sync.Once
}

func (p *FunctionURLPermission) Sid() string {
	p.once.Do(func() {
		b, _ := json.Marshal(p)
		h := sha1.Sum(b)
		p.sid = fmt.Sprintf(SidFormat, h)
	})
	return p.sid
}

type PolicyOutput struct {
	Id        string            `json:"Id"`
	Version   string            `json:"Version"`
	Statement []PolicyStatement `json:"Statement"`
}

type PolicyStatement struct {
	Sid       string `json:"Sid"`
	Effect    string `json:"Effect"`
	Principal any    `json:"Principal"`
	Action    string `json:"Action"`
	Resource  any    `json:"Resource"`
	Condition any    `json:"Condition"`
}

func (ps *PolicyStatement) PrincipalAccountID() *string {
	if ps.Principal == nil {
		return nil
	}
	switch v := ps.Principal.(type) {
	case string:
		return aws.String(v)
	case map[string]interface{}:
		if v["AWS"] == nil {
			return nil
		}
		switch vv := v["AWS"].(type) {
		case string:
			if a, err := arn.Parse(vv); err == nil {
				return aws.String(a.AccountID)
			}
			return aws.String(vv)
		}
	}
	return nil
}

/*
	{
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
	    }
*/
func (ps *PolicyStatement) PrincipalOrgID() *string {
	principal := ps.PrincipalAccountID()
	if principal == nil || *principal != "*" {
		return nil
	}
	m, ok := ps.Condition.(map[string]interface{})
	if !ok {
		return nil
	}
	if m["StringEquals"] == nil {
		return nil
	}
	mm, ok := m["StringEquals"].(map[string]interface{})
	if !ok {
		return nil
	}
	if mm["lambda:FunctionUrlAuthType"] == nil {
		return nil
	}
	if v, ok := mm["lambda:FunctionUrlAuthType"].(string); ok && v != "AWS_IAM" {
		return nil
	}
	if mm["aws:PrincipalOrgID"] == nil {
		return nil
	}
	if v, ok := mm["aws:PrincipalOrgID"].(string); ok {
		return aws.String(v)
	}
	return nil
}

func (app *App) loadFunctionUrl(path string, functionName string) (*FunctionURL, error) {
	f, err := loadDefinitionFile[FunctionURL](app, path, DefaultFunctionURLFilenames)
	if err != nil {
		return nil, err
	}
	if err := f.Validate(functionName); err != nil {
		return nil, err
	}
	return f, nil
}

func (app *App) deployFunctionURL(ctx context.Context, fc *FunctionURL) error {
	log.Println("[info] deploying function url...")

	if err := app.deployFunctionURLConfig(ctx, fc); err != nil {
		return fmt.Errorf("failed to deploy function url config: %w", err)
	}

	if err := app.deployFunctionURLPermissions(ctx, fc); err != nil {
		return fmt.Errorf("failed to deploy function url permissions: %w", err)
	}

	log.Println("[info] deployed function url")
	return nil
}

func (app *App) deployFunctionURLConfig(ctx context.Context, fc *FunctionURL) error {
	create := false
	fqFunctionName := fullQualifiedFunctionName(*fc.Config.FunctionName, fc.Config.Qualifier)
	functinoUrlConfig, err := app.lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: fc.Config.FunctionName,
		Qualifier:    fc.Config.Qualifier,
	})
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			log.Printf("[info] function url config for %s not found. creating", fqFunctionName)
			create = true
		} else {
			return fmt.Errorf("failed to get function url config: %w", err)
		}
	}

	if create {
		res, err := app.lambda.CreateFunctionUrlConfig(ctx, fc.Config)
		if err != nil {
			return fmt.Errorf("failed to create function url config: %w", err)
		}
		log.Printf("[info] created function url config for %s", fqFunctionName)
		log.Printf("[info] Function URL: %s", *res.FunctionUrl)
	} else {
		log.Printf("[info] updating function url config for %s", fqFunctionName)
		if functinoUrlConfig.Cors != nil && fc.Config.Cors == nil {
			// reset cors config
			fc.Config.Cors = &types.Cors{}
		}
		res, err := app.lambda.UpdateFunctionUrlConfig(ctx, &lambda.UpdateFunctionUrlConfigInput{
			FunctionName: fc.Config.FunctionName,
			Qualifier:    fc.Config.Qualifier,
			AuthType:     fc.Config.AuthType,
			Cors:         fc.Config.Cors,
			InvokeMode:   fc.Config.InvokeMode,
		})
		if err != nil {
			return fmt.Errorf("failed to update function url config: %w", err)
		}
		log.Printf("[info] updated function url config for %s", fqFunctionName)
		log.Printf("[info] Function URL: %s", *res.FunctionUrl)
	}
	return nil
}

func (app *App) deployFunctionURLPermissions(ctx context.Context, fc *FunctionURL) error {
	fqFunctionName := fullQualifiedFunctionName(*fc.Config.FunctionName, fc.Config.Qualifier)
	existsSids := []string{}
	{
		res, err := app.lambda.GetPolicy(ctx, &lambda.GetPolicyInput{
			FunctionName: fc.Config.FunctionName,
			Qualifier:    fc.Config.Qualifier,
		})
		if err != nil {
			var nfe *types.ResourceNotFoundException
			if errors.As(err, &nfe) {
				// do nothing
			} else {
				return fmt.Errorf("failed to get policy: %w", err)
			}
		}
		if res != nil {
			log.Printf("[debug] policy for %s: %s", fqFunctionName, *res.Policy)
			var policy PolicyOutput
			if err := json.Unmarshal([]byte(*res.Policy), &policy); err != nil {
				return fmt.Errorf("failed to unmarshal policy: %w", err)
			}
			for _, s := range policy.Statement {
				if s.Action != "lambda:InvokeFunctionUrl" || s.Effect != "Allow" {
					// not a lambda function url policy
					continue
				}
				existsSids = append(existsSids, s.Sid)
			}
			sort.Strings(existsSids)
		}
	}

	removeSids, addSids := lo.Difference(existsSids, fc.Permissions.Sids())
	if len(removeSids) == 0 && len(addSids) == 0 {
		log.Println("[info] no changes in permissions")
		return nil
	}

	log.Printf("[info] adding %d permissions", len(addSids))
	for _, sid := range addSids {
		p := fc.Permissions.Find(sid)
		if p == nil {
			// should not happen
			panic(fmt.Sprintf("permission not found: %s", sid))
		}
		in := &lambda.AddPermissionInput{
			Action:              aws.String("lambda:InvokeFunctionUrl"),
			FunctionName:        fc.Config.FunctionName,
			Qualifier:           fc.Config.Qualifier,
			FunctionUrlAuthType: fc.Config.AuthType,
			StatementId:         aws.String(sid),
			Principal:           p.Principal,
			PrincipalOrgID:      p.PrincipalOrgID,
		}
		v, _ := json.Marshal(p)
		log.Printf("[debug] adding permission: %s", string(v))
		if _, err := app.lambda.AddPermission(ctx, in); err != nil {
			return fmt.Errorf("failed to add permission: %w", err)
		}
		log.Printf("[info] added permission for %s", fqFunctionName)
	}

	for _, sid := range removeSids {
		log.Printf("[info] removing permission Sid %s...", sid)
		if _, err := app.lambda.RemovePermission(ctx, &lambda.RemovePermissionInput{
			FunctionName: fc.Config.FunctionName,
			Qualifier:    fc.Config.Qualifier,
			StatementId:  aws.String(sid),
		}); err != nil {
			return fmt.Errorf("failed to remove permission: %w", err)
		}
		log.Printf("[info] removed permission Sid %s", sid)
	}

	return nil
}

func (app *App) initFunctionURL(ctx context.Context, fn *Function, opt *InitOption) error {
	fc, err := app.lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: fn.FunctionName,
		Qualifier:    opt.Qualifier,
	})
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			log.Printf("[warn] function url config for %s not found", *fn.FunctionName)
			return nil
		} else {
			return fmt.Errorf("failed to get function url config: %w", err)
		}
	}
	fqFunctionName := fullQualifiedFunctionName(*fn.FunctionName, opt.Qualifier)
	fu := &FunctionURL{
		Config: &lambda.CreateFunctionUrlConfigInput{
			Cors:       fc.Cors,
			AuthType:   fc.AuthType,
			InvokeMode: fc.InvokeMode,
			Qualifier:  opt.Qualifier,
		},
	}

	{
		res, err := app.lambda.GetPolicy(ctx, &lambda.GetPolicyInput{
			FunctionName: fn.FunctionName,
			Qualifier:    opt.Qualifier,
		})
		if err != nil {
			var nfe *types.ResourceNotFoundException
			if errors.As(err, &nfe) {
				// do nothing
			} else {
				return fmt.Errorf("failed to get policy: %w", err)
			}
		}
		if res != nil {
			log.Printf("[debug] policy for %s: %s", fqFunctionName, *res.Policy)
			var policy PolicyOutput
			if err := json.Unmarshal([]byte(*res.Policy), &policy); err != nil {
				return fmt.Errorf("failed to unmarshal policy: %w", err)
			}
			for _, s := range policy.Statement {
				if s.Action != "lambda:InvokeFunctionUrl" || s.Effect != "Allow" {
					// not a lambda function url policy
					continue
				}
				fu.Permissions = append(fu.Permissions, &FunctionURLPermission{
					AddPermissionInput: lambda.AddPermissionInput{
						Principal:      s.PrincipalAccountID(),
						PrincipalOrgID: s.PrincipalOrgID(),
					},
				})
			}
		}
	}

	var name string
	if opt.Jsonnet {
		name = DefaultFunctionURLFilenames[1]
	} else {
		name = DefaultFunctionURLFilenames[0]
	}
	log.Printf("[info] creating %s", name)
	b, _ := marshalJSON(fu)
	if opt.Jsonnet {
		b, err = jsonToJsonnet(b, name)
		if err != nil {
			return err
		}
	}
	if err := app.saveFile(name, b, os.FileMode(0644)); err != nil {
		return err
	}

	return nil
}

func fillDefaultValuesFunctionUrlConfig(fc *FunctionURLConfig) {
	if fc.AuthType == "" {
		fc.AuthType = types.FunctionUrlAuthTypeNone
	}
	if fc.InvokeMode == "" {
		fc.InvokeMode = types.InvokeModeBuffered
	}
}
