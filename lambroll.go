package lambroll

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/google/go-jsonnet"
	"github.com/hashicorp/go-envparse"
	"github.com/kayac/go-config"
	"github.com/pkg/errors"
	"github.com/shogo82148/go-retry"
)

const versionLatest = "$LATEST"

var retryPolicy = retry.Policy{
	MinDelay: time.Second,
	MaxDelay: 5 * time.Second,
	MaxCount: 10,
}

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

	extStr  map[string]string
	extCode map[string]string
}

// New creates an application
func New(opt *Option) (*App, error) {
	for _, envfile := range *opt.Envfile {
		if err := exportEnvFile(envfile); err != nil {
			return nil, err
		}
	}

	awsCfg := &aws.Config{}
	if opt.Region != nil && *opt.Region != "" {
		awsCfg.Region = aws.String(*opt.Region)
	}
	if opt.Endpoint != nil && *opt.Endpoint != "" {
		customResolverFunc := func(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
			switch service {
			case endpoints.S3ServiceID, endpoints.LambdaServiceID, endpoints.StsServiceID:
				return endpoints.ResolvedEndpoint{
					URL: *opt.Endpoint,
				}, nil
			default:
				return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
			}
		}
		awsCfg.EndpointResolver = endpoints.ResolverFunc(customResolverFunc)
	}
	sessOpt := session.Options{
		Config:            *awsCfg,
		SharedConfigState: session.SharedConfigEnable,
	}
	var profile string
	if opt.Profile != nil && *opt.Profile != "" {
		sessOpt.Profile = *opt.Profile
		profile = *opt.Profile
	}
	sess := session.Must(session.NewSessionWithOptions(sessOpt))

	loader := config.New()
	if opt.TFState != nil && *opt.TFState != "" {
		funcs, err := tfstate.FuncMap(*opt.TFState)
		if err != nil {
			return nil, err
		}
		loader.Funcs(funcs)
	}

	app := &App{
		sess:    sess,
		lambda:  lambda.New(sess),
		profile: profile,
		loader:  loader,
	}
	if opt.ExtStr != nil {
		app.extStr = *opt.ExtStr
	}
	if opt.ExtCode != nil {
		app.extCode = *opt.ExtCode
	}

	return app, nil
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
	var (
		src []byte
		err error
	)
	switch filepath.Ext(path) {
	case ".jsonnet":
		vm := jsonnet.MakeVM()
		for k, v := range app.extStr {
			vm.ExtVar(k, v)
		}
		for k, v := range app.extCode {
			vm.ExtCode(k, v)
		}
		jsonStr, err := vm.EvaluateFile(path)
		if err != nil {
			return nil, err
		}
		src, err = app.loader.ReadWithEnvBytes([]byte(jsonStr))
		if err != nil {
			return nil, err
		}
	default:
		src, err = app.loader.ReadWithEnv(path)
		if err != nil {
			return nil, err
		}
	}
	var fn Function
	if err := unmarshalJSON(src, &fn, path); err != nil {
		return nil, errors.Wrapf(err, "failed to load %s", path)
	}
	return &fn, nil
}

func fillDefaultValues(fn *Function) {
	if len(fn.Architectures) == 0 {
		fn.Architectures = []*string{aws.String("x86_64")}
	}
	if fn.Description == nil {
		fn.Description = aws.String("")
	}
	if fn.MemorySize == nil {
		fn.MemorySize = aws.Int64(128)
	}
	if fn.TracingConfig == nil {
		fn.TracingConfig = &lambda.TracingConfig{
			Mode: aws.String("PassThrough"),
		}
	}
	if fn.Timeout == nil {
		fn.Timeout = aws.Int64(3)
	}
}

func newFuctionFrom(c *lambda.FunctionConfiguration, tags Tags) *Function {
	fn := &Function{
		Architectures:     c.Architectures,
		Description:       c.Description,
		FunctionName:      c.FunctionName,
		Handler:           c.Handler,
		MemorySize:        c.MemorySize,
		Role:              c.Role,
		Runtime:           c.Runtime,
		Timeout:           c.Timeout,
		DeadLetterConfig:  c.DeadLetterConfig,
		FileSystemConfigs: c.FileSystemConfigs,
		KMSKeyArn:         c.KMSKeyArn,
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

func exportEnvFile(file string) error {
	if file == "" {
		return nil
	}

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	envs, err := envparse.Parse(f)
	if err != nil {
		return err
	}
	for key, value := range envs {
		os.Setenv(key, value)
	}
	return nil
}
