package lambroll

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

func (p *FunctionURLPermission) String() string {
	b, _ := marshalJSON(p)
	return string(b)
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
	ret := make([]*lambda.AddPermissionInput, 0, len(perms))
	for _, perm := range perms {
		// use actual StatementId if exists
		if p.actualSids != nil {
			if sid, ok := p.actualSids[aws.ToString(perm.Action)]; ok {
				perm.StatementId = aws.String(sid)
				ret = append(ret, perm)
			} else {
				// not exists on remote, do not add
				slog.Debug("not found actual StatementId for action", "action", aws.ToString(perm.Action))
			}
			continue
		}
		// generate StatementId based on permission content
		b, _ := marshalJSON(perm)
		sid := fmt.Sprintf(SidFormat, sha1.Sum(b))
		perm.StatementId = aws.String(sid)
		ret = append(ret, perm)
	}
	return ret
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
	if ps.Condition == nil {
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
	if mm["aws:SourceAccount"] == nil {
		return nil
	}
	if v, ok := mm["aws:SourceAccount"].(string); ok {
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
	slog.Info("deploying function url...", "label", opt.label())

	if err := app.deployFunctionURLConfig(ctx, fc, opt); err != nil {
		return fmt.Errorf("failed to deploy function url config: %w", err)
	}

	if err := app.deployFunctionURLPermissions(ctx, fc, opt); err != nil {
		return fmt.Errorf("failed to deploy function url permissions: %w", err)
	}

	slog.Info("deployed function url", "label", opt.label())
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
			slog.Info("function url config not found. creating", "function", fqFunctionName, "label", opt.label())
			create = true
		} else {
			return fmt.Errorf("failed to get function url config: %w", err)
		}
	}

	if opt.DryRun {
		slog.Info("dry-run mode. skipping function url config deployment")
		return nil
	}

	if create {
		res, err := app.lambda.CreateFunctionUrlConfig(ctx, fc.Config)
		if err != nil {
			return fmt.Errorf("failed to create function url config: %w", err)
		}
		slog.Info("created function url config", "function", fqFunctionName)
		slog.Info("Function URL", "url", *res.FunctionUrl)
	} else {
		slog.Info("updating function url config", "function", fqFunctionName)
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
		slog.Info("updated function url config", "function", fqFunctionName)
		slog.Info("Function URL", "url", *res.FunctionUrl)
	}
	return nil
}

func (app *App) deployFunctionURLPermissions(ctx context.Context, fc *FunctionURL, opt *DeployOption) error {
	adds, removes, err := app.calcFunctionURLPermissionsDiff(ctx, fc)
	if err != nil {
		return err
	}
	if len(adds) == 0 && len(removes) == 0 {
		slog.Info("no changes in permissions")
		return nil
	}

	slog.Info("adding permissions", "count", len(adds), "label", opt.label())
	if !opt.DryRun {
		for _, perm := range adds {
			if _, err := app.lambda.AddPermission(ctx, perm); err != nil {
				return fmt.Errorf("failed to add permission: %w", err)
			}
			slog.Info("added permission", "action", aws.ToString(perm.Action), "sid", aws.ToString(perm.StatementId))
		}
	}

	slog.Info("removing permissions", "count", len(removes), "label", opt.label())
	if !opt.DryRun {
		for _, perm := range removes {
			if _, err := app.lambda.RemovePermission(ctx, &lambda.RemovePermissionInput{
				FunctionName: perm.FunctionName,
				Qualifier:    perm.Qualifier,
				StatementId:  perm.StatementId,
			}); err != nil {
				var nfe *types.ResourceNotFoundException
				if errors.As(err, &nfe) {
					slog.Warn("permission Sid not found. skipped removing", "sid", *perm.StatementId)
					continue
				}
				return fmt.Errorf("failed to remove permission: %w", err)
			}
			slog.Info("removed permission", "sid", *perm.StatementId)
		}
	}
	return nil
}

func (app *App) calcFunctionURLPermissionsDiff(ctx context.Context, fc *FunctionURL) ([]*lambda.AddPermissionInput, []*lambda.AddPermissionInput, error) {
	// remote permissions to be compared
	remotePermissions, err := app.getFunctionURLPermissions(ctx, *fc.Config.FunctionName, fc.Config.Qualifier)
	if err != nil {
		return nil, nil, err
	}
	remote := make(map[string]*lambda.AddPermissionInput)
	for _, p := range remotePermissions {
		inputs := p.AddPermissionInputs(fc)
		for _, in := range inputs {
			slog.Debug("remote permission", "sid", aws.ToString(in.StatementId), "action", aws.ToString(in.Action))
			sid := aws.ToString(in.StatementId)
			remote[sid] = in
		}
	}

	// local permissions to be applied
	local := make(map[string]*lambda.AddPermissionInput)
	for _, p := range fc.Permissions {
		inputs := p.AddPermissionInputs(fc)
		for _, in := range inputs {
			slog.Debug("local permission", "sid", aws.ToString(in.StatementId), "action", aws.ToString(in.Action))
			sid := aws.ToString(in.StatementId)
			local[sid] = in
		}
	}

	// calculate difference
	removeSids, addSids := lo.Difference(lo.Keys(remote), lo.Keys(local))
	if len(removeSids) == 0 && len(addSids) == 0 {
		slog.Debug("no changes in permissions")
		return nil, nil, nil
	}
	slog.Debug("SIDs to be added", "sids", addSids)
	slog.Debug("SIDs to be removed", "sids", removeSids)

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
	slog.Debug("policy", "function", fqFunctionName, "policy", *res.Policy)
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
				slog.Warn("function url config not found", "function", *fn.FunctionName)
				return nil
			} else {
				slog.Info("initializing function url config", "function", *fn.FunctionName)
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
	slog.Info("creating file", "name", name)
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
