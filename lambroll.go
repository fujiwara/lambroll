package lambroll

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/kayac/go-config"
	"github.com/kayac/go-config/tfstate"
	"github.com/pkg/errors"
)

// Function represents configuration of Lambda function
type Function = lambda.CreateFunctionInput

// Tags represents tags of function
type Tags = map[string]*string

func (app *App) functionArn(name string) string {
	return fmt.Sprintf(
		"arn:aws:lambda:%s:%s:function:%s",
		*app.sess.Config.Region,
		app.AWSAccountID(),
		name,
	)
}

var (
	// IgnoreFilename defines file name includes ingore patterns at creating zip archive.
	IgnoreFilename = ".lambdaignore"

	// FunctionFilename defines file name for function definition.
	FunctionFilename = "function.json"

	// FunctionZipFilename defines file name for zip archive downloaded at init.
	FunctionZipFilename = "function.zip"

	// DefaultExcludes is a preset excludes file list
	DefaultExcludes = []string{
		IgnoreFilename,
		FunctionFilename,
		FunctionZipFilename,
		".git/*",
		".terraform/*",
		"terraform.tfstate",
	}

	// CurrentAliasName is alias name for current deployed function
	CurrentAliasName = "current"
)

// App represents lambroll application
type App struct {
	sess      *session.Session
	lambda    *lambda.Lambda
	accountID string
	profile   string
	loader    *config.Loader
}

// New creates an application
func New(opt *Option) (*App, error) {
	awsCfg := &aws.Config{}
	if opt.Region != nil {
		awsCfg.Region = aws.String(*opt.Region)
	}
	sessOpt := session.Options{Config: *awsCfg}
	var profile string
	if opt.Profile != nil {
		sessOpt.Profile = *opt.Profile
		profile = *opt.Profile
	}
	sess := session.Must(session.NewSessionWithOptions(sessOpt))

	loader := config.New()
	if opt.TFState != nil {
		funcs, err := tfstate.Load(*opt.TFState)
		if err != nil {
			return nil, err
		}
		loader.Funcs(funcs)
	}

	return &App{
		sess:    sess,
		lambda:  lambda.New(sess),
		profile: profile,
		loader:  loader,
	}, nil
}

// AWSAccountID returns AWS account ID in current session
func (app *App) AWSAccountID() string {
	if app.accountID != "" {
		return app.accountID
	}
	svc := sts.New(app.sess)
	r, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		log.Println("[warn] failed to get caller identity", err)
		return ""
	}
	app.accountID = *r.Account
	return app.accountID
}

func (app *App) loadFunction(path string) (*Function, error) {
	var fn Function
	err := app.loader.LoadWithEnvJSON(&fn, path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load %s", path)
	}
	return &fn, nil
}

func newFuctionFrom(c *lambda.FunctionConfiguration, tags Tags) *Function {
	fn := &Function{
		Description:  c.Description,
		FunctionName: c.FunctionName,
		Handler:      c.Handler,
		MemorySize:   c.MemorySize,
		Role:         c.Role,
		Runtime:      c.Runtime,
		Timeout:      c.Timeout,
	}
	if e := c.Environment; e != nil {
		fn.Environment = &lambda.Environment{
			Variables: e.Variables,
		}
	}
	for _, layer := range c.Layers {
		fn.Layers = append(fn.Layers, layer.Arn)
	}
	if t := c.TracingConfig; t != nil {
		fn.TracingConfig = &lambda.TracingConfig{
			Mode: t.Mode,
		}
	}
	if v := c.VpcConfig; v != nil && *v.VpcId != "" {
		fn.VpcConfig = &lambda.VpcConfig{
			SubnetIds:        v.SubnetIds,
			SecurityGroupIds: v.SecurityGroupIds,
		}
	}
	fn.Tags = tags

	return fn
}
