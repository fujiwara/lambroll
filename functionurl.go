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
	"strings"

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
				Principal: aws.String("*"),
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

func (fc *FunctionURL) RemovePermissionInput(sid string) *lambda.RemovePermissionInput {
	return &lambda.RemovePermissionInput{
		FunctionName: fc.Config.FunctionName,
		Qualifier:    fc.Config.Qualifier,
		StatementId:  aws.String(sid),
	}
}

type FunctionURLConfig = lambda.CreateFunctionUrlConfigInput

type FunctionURLPermissions []*FunctionURLPermission

type FunctionURLPermission struct {
	Principal      *string `json:"Principal,omitempty"`
	PrincipalOrgID *string `json:"PrincipalOrgID,omitempty"`
	SourceArn      *string `json:"SourceArn,omitempty"`
	SourceAccount  *string `json:"SourceAccount,omitempty"`

	actualSids map[string]string // action -> sid(on remote)
}

func (p *FunctionURLPermission) Equals(o *FunctionURLPermission) bool {
	return aws.ToString(p.Principal) == aws.ToString(o.Principal) &&
		aws.ToString(p.PrincipalOrgID) == aws.ToString(o.PrincipalOrgID) &&
		aws.ToString(p.SourceArn) == aws.ToString(o.SourceArn) &&
		aws.ToString(p.SourceAccount) == aws.ToString(o.SourceAccount)
}

func (p *FunctionURLPermission) AddPermissionInputs(fc *FunctionURL) []*lambda.AddPermissionInput {
	perms := []*lambda.AddPermissionInput{
		{
			Action:              aws.String("lambda:InvokeFunctionUrl"),
			FunctionName:        fc.Config.FunctionName,
			Qualifier:           fc.Config.Qualifier,
			FunctionUrlAuthType: fc.Config.AuthType,
			Principal:           p.Principal,
			PrincipalOrgID:      p.PrincipalOrgID,
			SourceArn:           p.SourceArn,
			SourceAccount:       p.SourceAccount,
		},
		{
			Action:                aws.String("lambda:InvokeFunction"),
			FunctionName:          fc.Config.FunctionName,
			Qualifier:             fc.Config.Qualifier,
			Principal:             p.Principal,
			PrincipalOrgID:        p.PrincipalOrgID,
			SourceArn:             p.SourceArn,
			SourceAccount:         p.SourceAccount,
			InvokedViaFunctionUrl: aws.Bool(true),
		},
	}
	for _, perm := range perms {
		// use actual StatementId if exists
		if p.actualSids != nil {
			if sid, ok := p.actualSids[aws.ToString(perm.Action)]; ok {
				perm.StatementId = aws.String(sid)
				continue
			}
		}
		// generate StatementId based on permission content
		b, _ := marshalJSON(perm)
		sha1sum := sha1.Sum(b)
		sid := fmt.Sprintf(SidFormat, sha1sum)
		perm.StatementId = aws.String(sid)
	}
	return perms
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

func (ps *PolicyStatement) SourceAccount() *string {
	return nil // TODO
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
		for _, perm := range adds {
			if _, err := app.lambda.AddPermission(ctx, perm); err != nil {
				return fmt.Errorf("failed to add permission: %w", err)
			}
			log.Printf("[info] added permission Sid:%s Action:%s",
				aws.ToString(perm.StatementId), aws.ToString(perm.Action),
			)
		}
	}

	log.Printf("[info] removing %d permissions %s", len(removes), opt.label())
	if !opt.DryRun {
		for _, perm := range removes {
			if _, err := app.lambda.RemovePermission(ctx, &lambda.RemovePermissionInput{
				FunctionName: perm.FunctionName,
				Qualifier:    perm.Qualifier,
				StatementId:  perm.StatementId,
			}); err != nil {
				var nfe *types.ResourceNotFoundException
				if errors.As(err, &nfe) {
					log.Printf("[warn] permission Sid: %s not found. skipped removing.", *perm.StatementId)
					continue
				}
				return fmt.Errorf("failed to remove permission: %w", err)
			}
			log.Printf("[info] removed permission Sid: %s", *perm.StatementId)
		}
	}
	return nil
}

func (app *App) calcFunctionURLPermissionsDiff(ctx context.Context, fc *FunctionURL) ([]*lambda.AddPermissionInput, []*lambda.AddPermissionInput, error) {
	remotePermissions, err := app.getFunctionURLPermissions(ctx, *fc.Config.FunctionName, fc.Config.Qualifier)
	if err != nil {
		return nil, nil, err
	}
	remote := make(map[string]*lambda.AddPermissionInput)
	remoteSids := make([]string, 0)
	for _, p := range remotePermissions {
		perms := p.AddPermissionInputs(fc)
		for _, perm := range perms {
			sid := aws.ToString(perm.StatementId)
			remote[sid] = perm
			remoteSids = append(remoteSids, sid)
		}
	}

	// local permissions to be applied
	local := make(map[string]*lambda.AddPermissionInput)
	localSids := make([]string, 0)
	for _, p := range fc.Permissions {
		perms := p.AddPermissionInputs(fc)
		for _, perm := range perms {
			sid := aws.ToString(perm.StatementId)
			local[sid] = perm
			localSids = append(localSids, sid)
		}
	}

	// calculate difference
	removeSids, addSids := lo.Difference(remoteSids, localSids)
	if len(removeSids) == 0 && len(addSids) == 0 {
		return nil, nil, nil
	}

	var adds []*lambda.AddPermissionInput
	for _, sid := range addSids {
		adds = append(adds, local[sid])
	}

	var removes []*lambda.AddPermissionInput
	for _, sid := range removeSids {
		removes = append(removes, remote[sid])
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
	if res == nil {
		return ps, nil
	}
	log.Printf("[debug] policy for %s: %s", fqFunctionName, *res.Policy)
	var policy PolicyOutput
	if err := json.Unmarshal([]byte(*res.Policy), &policy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal policy: %w", err)
	}
	for _, s := range policy.Statement {
		if (s.Action == "lambda:InvokeFunctionUrl" || s.Action == "lambda:InvokeFunction") && s.Effect == "Allow" {
			// lambda function url policy
		} else {
			continue
		}
		p := &FunctionURLPermission{
			Principal:      s.PrincipalString(),
			PrincipalOrgID: s.PrincipalOrgID(),
			SourceArn:      s.SourceArn(),
			SourceAccount:  s.SourceAccount(),
			actualSids:     map[string]string{s.Action: s.Sid},
		}
		// if existing permission, merge actualSids
		var found bool
		for _, e := range ps {
			if p.Equals(e) {
				// merge actualSids
				e.actualSids[s.Action] = s.Sid
				found = true
			}
		}
		if !found {
			ps = append(ps, p)
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
