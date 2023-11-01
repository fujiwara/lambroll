package lambroll

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"slices"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/samber/lo"
)

var (
	SidPattern = regexp.MustCompile("^lambroll-[0-9a-f]+$")
)

type FunctionURL struct {
	Config      *FunctionURLConfig     `json:"Config"`
	Permissions FunctionURLPermissions `json:"Permissions"`
}

type FunctionURLConfig = lambda.CreateFunctionUrlConfigInput

type FunctionURLPermissions []*FunctionURLPermission

func (ps FunctionURLPermissions) Sids() []string {
	sids := lo.Map(ps, func(p *FunctionURLPermission, _ int) string {
		return p.Sid()
	})
	slices.Sort(sids)
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

	once sync.Once
	sid  string
}

func (p *FunctionURLPermission) Sid() string {
	p.once.Do(func() {
		b, _ := json.Marshal(p)
		h := sha1.Sum(b)
		p.sid = fmt.Sprintf("lambroll-%x", h)
	})
	return p.sid
}

func (p *FunctionURLPermission) String() string {
	b, _ := json.Marshal(p)
	return string(b)
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
	Resource  string `json:"Resource"`
	Condition any    `json:"Condition"`
}

func (app *App) loadFunctionUrl(path string, functionName string) (*FunctionURL, error) {
	f, err := loadDefinitionFile[FunctionURL](app, path, DefaultFunctionURLFilenames)
	if err != nil {
		return nil, err
	}
	if f.Config == nil {
		return nil, errors.New("function url 'Config' attribute is required")
	}
	// fill default values
	if f.Config.AuthType == types.FunctionUrlAuthTypeNone && len(f.Permissions) == 0 {
		f.Permissions = append(f.Permissions, &FunctionURLPermission{
			AddPermissionInput: lambda.AddPermissionInput{
				Principal: aws.String("*"),
			},
		})
	}
	f.Config.FunctionName = &functionName
	for _, p := range f.Permissions {
		p.FunctionName = &functionName
	}
	return f, nil
}

func (app *App) deployFunctionURL(ctx context.Context, functionName, path string) error {
	fc, err := app.loadFunctionUrl(path, functionName)
	if err != nil {
		return fmt.Errorf("failed to load function url config: %w", err)
	}
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
	if _, err := app.lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: fc.Config.FunctionName,
		Qualifier:    fc.Config.Qualifier,
	}); err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			log.Printf("[info] function url config for %s not found. creating", *fc.Config.FunctionName)
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
		log.Printf("[info] created function url config for %s", *fc.Config.FunctionName)
		log.Printf("[info] Function URL: %s", *res.FunctionUrl)
	} else {
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
		log.Printf("[info] updated function url config for %s", *fc.Config.FunctionName)
		log.Printf("[info] Function URL: %s", *res.FunctionUrl)
	}
	return nil
}

func (app *App) deployFunctionURLPermissions(ctx context.Context, fc *FunctionURL) error {
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
			log.Printf("[debug] policy for %s: %s", *fc.Config.FunctionName, *res.Policy)
			var policy PolicyOutput
			if err := json.Unmarshal([]byte(*res.Policy), &policy); err != nil {
				return fmt.Errorf("failed to unmarshal policy: %w", err)
			}
			for _, s := range policy.Statement {
				if !SidPattern.MatchString(s.Sid) || s.Action != "lambda:InvokeFunctionUrl" || s.Effect != "Allow" {
					// not a lambroll policy
					continue
				}
				existsSids = append(existsSids, s.Sid)
			}
			slices.Sort(existsSids)
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
		log.Printf("[info] added permission for %s", *fc.Config.FunctionName)
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
