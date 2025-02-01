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
	"strings"
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

func (fc *FunctionURL) AddPermissionInput(p *FunctionURLPermission) *lambda.AddPermissionInput {
	return &lambda.AddPermissionInput{
		Action:              aws.String("lambda:InvokeFunctionUrl"),
		FunctionName:        fc.Config.FunctionName,
		Qualifier:           fc.Config.Qualifier,
		FunctionUrlAuthType: fc.Config.AuthType,
		StatementId:         aws.String(p.Sid()),
		Principal:           p.Principal,
		PrincipalOrgID:      p.PrincipalOrgID,
		SourceArn:           p.SourceArn,
		SourceAccount:       p.SourceAccount,
	}
}

func (fc *FunctionURL) RemovePermissionInput(sid string) *lambda.RemovePermissionInput {
	return &lambda.RemovePermissionInput{
		FunctionName: fc.Config.FunctionName,
		Qualifier:    fc.Config.Qualifier,
		StatementId:  aws.String(sid),
	}
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
	if p.sid != "" {
		return p.sid
	} else if p.StatementId != nil {
		return *p.StatementId
	}
	p.once.Do(func() {
		b, _ := json.Marshal(p)
		h := sha1.Sum(b)
		p.sid = fmt.Sprintf(SidFormat, h)
		p.StatementId = aws.String(p.sid)
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

func (ps *PolicyStatement) PrincipalString() *string {
	if ps.Principal == nil {
		return nil
	}
	switch v := ps.Principal.(type) {
	case string:
		return aws.String(v)
	case map[string]interface{}:
		if v["AWS"] != nil {
			switch vv := v["AWS"].(type) {
			case string:
				if a, err := arn.Parse(vv); err == nil {
					return aws.String(a.AccountID)
				}
				return aws.String(vv)
			}
		} else if v["Service"] != nil {
			switch vv := v["Service"].(type) {
			case string:
				return aws.String(vv)
			}
		}
	}
	return nil
}

func (ps *PolicyStatement) PrincipalOrgID() *string {
	principal := ps.PrincipalString()
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

func (ps *PolicyStatement) SourceArn() *string {
	if ps.Condition == nil {
		return nil
	}
	m, ok := ps.Condition.(map[string]interface{})
	if !ok {
		return nil
	}
	if m["ArnLike"] == nil {
		return nil
	}
	mm, ok := m["ArnLike"].(map[string]interface{})
	if !ok {
		return nil
	}
	var sourceArn any
	for k, v := range mm {
		if strings.ToLower(k) == "aws:sourcearn" {
			sourceArn = v
			break
		}
	}
	if sourceArn == nil {
		return nil
	}
	if v, ok := sourceArn.(string); ok {
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

func (app *App) deployFunctionURL(ctx context.Context, fc *FunctionURL, opt *DeployOption) error {
	log.Printf("[info] deploying function url... %s", opt.label())

	if err := app.deployFunctionURLConfig(ctx, fc, opt); err != nil {
		return fmt.Errorf("failed to deploy function url config: %w", err)
	}

	if err := app.deployFunctionURLPermissions(ctx, fc, opt); err != nil {
		return fmt.Errorf("failed to deploy function url permissions: %w", err)
	}

	log.Println("[info] deployed function url", opt.label())
	return nil
}

func (app *App) deployFunctionURLConfig(ctx context.Context, fc *FunctionURL, opt *DeployOption) error {
	create := false
	fqFunctionName := fullQualifiedFunctionName(*fc.Config.FunctionName, fc.Config.Qualifier)
	functionUrlConfig, err := app.lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: fc.Config.FunctionName,
		Qualifier:    fc.Config.Qualifier,
	})
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			log.Printf("[info] function url config for %s not found. creating %s", fqFunctionName, opt.label())
			create = true
		} else {
			return fmt.Errorf("failed to get function url config: %w", err)
		}
	}

	if opt.DryRun {
		log.Println("[info] dry-run mode. skipping function url config deployment")
		return nil
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
		if functionUrlConfig.Cors != nil && fc.Config.Cors == nil {
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

func (app *App) deployFunctionURLPermissions(ctx context.Context, fc *FunctionURL, opt *DeployOption) error {
	adds, removes, err := app.calcFunctionURLPermissionsDiff(ctx, fc)
	if err != nil {
		return err
	}
	if len(adds) == 0 && len(removes) == 0 {
		log.Println("[info] no changes in permissions.")
		return nil
	}

	log.Printf("[info] adding %d permissions %s", len(adds), opt.label())
	if !opt.DryRun {
		for _, p := range adds {
			if _, err := app.lambda.AddPermission(ctx, fc.AddPermissionInput(p)); err != nil {
				return fmt.Errorf("failed to add permission: %w", err)
			}
			log.Printf("[info] added permission Sid: %s", p.Sid())
		}
	}

	log.Printf("[info] removing %d permissions %s", len(removes), opt.label())
	if !opt.DryRun {
		for _, p := range removes {
			if _, err := app.lambda.RemovePermission(ctx, fc.RemovePermissionInput(*p.StatementId)); err != nil {
				return fmt.Errorf("failed to remove permission: %w", err)
			}
			log.Printf("[info] removed permission Sid: %s", *p.StatementId)
		}
	}
	return nil
}

func (app *App) calcFunctionURLPermissionsDiff(ctx context.Context, fc *FunctionURL) (FunctionURLPermissions, FunctionURLPermissions, error) {
	existsPermissions, err := app.getFunctionURLPermissions(ctx, *fc.Config.FunctionName, fc.Config.Qualifier)
	if err != nil {
		return nil, nil, err
	}
	existsSids := lo.Map(existsPermissions, func(p *FunctionURLPermission, _ int) string {
		return p.Sid()
	})

	removeSids, addSids := lo.Difference(existsSids, fc.Permissions.Sids())
	if len(removeSids) == 0 && len(addSids) == 0 {
		return nil, nil, nil
	}

	var adds FunctionURLPermissions
	for _, sid := range addSids {
		p := fc.Permissions.Find(sid)
		if p == nil {
			// should not happen
			panic(fmt.Sprintf("permission not found for adding: %s", sid))
		}
		adds = append(adds, p)
	}

	var removes FunctionURLPermissions
	for _, sid := range removeSids {
		p := existsPermissions.Find(sid)
		if p == nil {
			// should not happen
			panic(fmt.Sprintf("permission not found for removal: %s", sid))
		}
		removes = append(removes, p)
	}

	return adds, removes, nil
}

func (app *App) getFunctionURLPermissions(ctx context.Context, functionName string, qualifier *string) (FunctionURLPermissions, error) {
	fqFunctionName := fullQualifiedFunctionName(functionName, qualifier)
	res, err := app.lambda.GetPolicy(ctx, &lambda.GetPolicyInput{
		FunctionName: &functionName,
		Qualifier:    qualifier,
	})
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			// do nothing
		} else {
			return nil, fmt.Errorf("failed to get policy: %w", err)
		}
	}
	ps := make(FunctionURLPermissions, 0)
	if res != nil {
		log.Printf("[debug] policy for %s: %s", fqFunctionName, *res.Policy)
		var policy PolicyOutput
		if err := json.Unmarshal([]byte(*res.Policy), &policy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal policy: %w", err)
		}
		for _, s := range policy.Statement {
			if s.Action != "lambda:InvokeFunctionUrl" || s.Effect != "Allow" {
				// not a lambda function url policy
				continue
			}
			st, _ := json.Marshal(s)
			log.Println("[debug] exists sid", s.Sid, string(st))
			ps = append(ps, &FunctionURLPermission{
				sid: s.Sid,
				AddPermissionInput: lambda.AddPermissionInput{
					StatementId:    aws.String(s.Sid),
					Principal:      s.PrincipalString(),
					PrincipalOrgID: s.PrincipalOrgID(),
					SourceArn:      s.SourceArn(),
				},
			})
		}
	}
	return ps, nil
}

func (app *App) initFunctionURL(ctx context.Context, fn *Function, exists bool, opt *InitOption) error {
	fc, err := app.lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: fn.FunctionName,
		Qualifier:    opt.Qualifier,
	})
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			if exists {
				log.Printf("[warn] function url config for %s not found", *fn.FunctionName)
				return nil
			} else {
				log.Printf("[info] initializing function url config for %s", *fn.FunctionName)
				// default settings will be used
				fc = &lambda.GetFunctionUrlConfigOutput{
					AuthType: types.FunctionUrlAuthTypeNone,
				}
			}
		} else {
			return fmt.Errorf("failed to get function url config: %w", err)
		}
	}

	fu := &FunctionURL{
		Config: &lambda.CreateFunctionUrlConfigInput{
			Cors:       fc.Cors,
			AuthType:   fc.AuthType,
			InvokeMode: fc.InvokeMode,
			Qualifier:  opt.Qualifier,
		},
	}

	ps, err := app.getFunctionURLPermissions(ctx, *fn.FunctionName, opt.Qualifier)
	if err != nil {
		return err
	}
	fu.Permissions = ps

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
	if err := app.saveFile(ctx, name, b, os.FileMode(0644), opt.ForceOverwrite); err != nil {
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
