package lambroll

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"
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

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	configv2 "github.com/aws/aws-sdk-go-v2/config"
	lambdav2 "github.com/aws/aws-sdk-go-v2/service/lambda"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	stsv2 "github.com/aws/aws-sdk-go-v2/service/sts"

	lambdav2types "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

const versionLatest = "$LATEST"
const packageTypeImage = "Image"

var retryPolicy = retry.Policy{
	MinDelay: time.Second,
	MaxDelay: 10 * time.Second,
	MaxCount: 30,
}

// Function represents configuration of Lambda function
type Function = lambda.CreateFunctionInput

type FunctionV2 = lambdav2.CreateFunctionInput

// Tags represents tags of function
type Tags = map[string]*string

type TagsV2 = map[string]string

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
	FunctionFilenames = []string{
		"function.json",
		"function.jsonnet",
	}

	// FunctionZipFilename defines file name for zip archive downloaded at init.
	FunctionZipFilename = "function.zip"

	// DefaultExcludes is a preset excludes file list
	DefaultExcludes = []string{
		IgnoreFilename,
		FunctionFilenames[0],
		FunctionFilenames[1],
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

	awsv2Config awsv2.Config
	lambdav2    *lambdav2.Client

	extStr  map[string]string
	extCode map[string]string
}

func newAwsV1Session(opt *Option) (*session.Session, error) {
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
	if opt.Profile != nil && *opt.Profile != "" {
		sessOpt.Profile = *opt.Profile
	}
	sess := session.Must(session.NewSessionWithOptions(sessOpt))
	return sess, nil
}

func newAwsV2Config(ctx context.Context, opt *Option) (awsv2.Config, error) {
	var region string
	if opt.Region != nil && *opt.Region != "" {
		region = awsv2.ToString(opt.Region)
	}
	optFuncs := []func(*configv2.LoadOptions) error{
		configv2.WithRegion(region),
	}
	if opt.Endpoint != nil && *opt.Endpoint != "" {
		customResolver := awsv2.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (awsv2.Endpoint, error) {
			if service == lambdav2.ServiceID || service == stsv2.ServiceID || service == s3v2.ServiceID {
				return awsv2.Endpoint{
					PartitionID:   "aws",
					URL:           *opt.Endpoint,
					SigningRegion: region,
				}, nil
			}
			// returning EndpointNotFoundError will allow the service to fallback to it's default resolution
			return awsv2.Endpoint{}, &awsv2.EndpointNotFoundError{}
		})
		optFuncs = append(optFuncs, configv2.WithEndpointResolverWithOptions(customResolver))
	}
	if opt.Profile != nil && *opt.Profile != "" {
		optFuncs = append(optFuncs, configv2.WithSharedConfigProfile("test-account"))
	}
	return configv2.LoadDefaultConfig(ctx, optFuncs...)
}

// New creates an application
func New(ctx context.Context, opt *Option) (*App, error) {
	for _, envfile := range *opt.Envfile {
		if err := exportEnvFile(envfile); err != nil {
			return nil, err
		}
	}

	sess, err := newAwsV1Session(opt)
	if err != nil {
		return nil, err
	}
	v2cfg, err := newAwsV2Config(ctx, opt)
	if err != nil {
		return nil, err
	}

	var profile string
	if opt.Profile != nil && *opt.Profile != "" {
		profile = *opt.Profile
	}

	loader := config.New()
	if opt.TFState != nil && *opt.TFState != "" {
		funcs, err := tfstate.FuncMap(*opt.TFState)
		if err != nil {
			return nil, err
		}
		loader.Funcs(funcs)
	}
	if opt.PrefixedTFState != nil {
		prefixedFuncs := make(template.FuncMap)
		for prefix, path := range *opt.PrefixedTFState {
			if prefix == "" {
				return nil, errors.New("--prefixed-tfstate option cannot have empty key")
			}

			funcs, err := tfstate.FuncMap(path)
			if err != nil {
				return nil, err
			}

			for name, f := range funcs {
				prefixedFuncs[prefix+name] = f
			}
		}
		loader.Funcs(prefixedFuncs)
	}

	app := &App{
		sess:    sess,
		lambda:  lambda.New(sess),
		profile: profile,
		loader:  loader,

		awsv2Config: v2cfg,
		lambdav2:    lambdav2.NewFromConfig(v2cfg),
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

func (app *App) loadFunctionV2(path string) (*FunctionV2, error) {
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
	var fn FunctionV2
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
	if fn.EphemeralStorage == nil {
		fn.EphemeralStorage = &lambda.EphemeralStorage{
			Size: aws.Int64(512),
		}
	}
	if fn.Timeout == nil {
		fn.Timeout = aws.Int64(3)
	}
	if fn.SnapStart == nil {
		fn.SnapStart = &lambda.SnapStart{
			ApplyOn: aws.String("None"),
		}
	}
}

func newFunctionFromV2(c *lambdav2types.FunctionConfiguration, code *lambdav2types.FunctionCodeLocation, tags TagsV2) *FunctionV2 {
	fn := &FunctionV2{
		Architectures:     c.Architectures,
		Description:       c.Description,
		EphemeralStorage:  c.EphemeralStorage,
		FunctionName:      c.FunctionName,
		Handler:           c.Handler,
		MemorySize:        c.MemorySize,
		Role:              c.Role,
		Runtime:           c.Runtime,
		Timeout:           c.Timeout,
		DeadLetterConfig:  c.DeadLetterConfig,
		FileSystemConfigs: c.FileSystemConfigs,
		KMSKeyArn:         c.KMSKeyArn,
		SnapStart:         newSnapStartV2(c.SnapStart),
	}

	if e := c.Environment; e != nil {
		fn.Environment = &lambdav2types.Environment{
			Variables: e.Variables,
		}
	}
	for _, layer := range c.Layers {
		fn.Layers = append(fn.Layers, *layer.Arn)
	}
	if t := c.TracingConfig; t != nil {
		fn.TracingConfig = &lambdav2types.TracingConfig{
			Mode: t.Mode,
		}
	}
	if v := c.VpcConfig; v != nil && *v.VpcId != "" {
		fn.VpcConfig = &lambdav2types.VpcConfig{
			SubnetIds:        v.SubnetIds,
			SecurityGroupIds: v.SecurityGroupIds,
		}
	}

	if (code != nil && awsv2.ToString(code.RepositoryType) == "ECR") || fn.PackageType == lambdav2types.PackageTypeImage {
		log.Printf("[debug] Image URL=%s", *code.ImageUri)
		fn.PackageType = lambdav2types.PackageTypeImage
		fn.Code = &lambdav2types.FunctionCode{
			ImageUri: code.ImageUri,
		}
	}

	fn.Tags = tags

	return fn
}

func fillDefaultValuesV2(fn *FunctionV2) {
	if len(fn.Architectures) == 0 {
		fn.Architectures = lambdav2types.ArchitectureX8664.Values()
	}
	if fn.Description == nil {
		fn.Description = awsv2.String("")
	}
	if fn.MemorySize == nil {
		fn.MemorySize = awsv2.Int32(128)
	}
	if fn.TracingConfig == nil {
		fn.TracingConfig = &lambdav2types.TracingConfig{
			Mode: lambdav2types.TracingModePassThrough,
		}
	}
	if fn.EphemeralStorage == nil {
		fn.EphemeralStorage = &lambdav2types.EphemeralStorage{
			Size: awsv2.Int32(512),
		}
	}
	if fn.Timeout == nil {
		fn.Timeout = awsv2.Int32(3)
	}
	if fn.SnapStart == nil {
		fn.SnapStart = &lambdav2types.SnapStart{
			ApplyOn: lambdav2types.SnapStartApplyOnNone,
		}
	}
}

func newFunctionFrom(c *lambda.FunctionConfiguration, code *lambda.FunctionCodeLocation, tags Tags) *Function {
	fn := &Function{
		Architectures:     c.Architectures,
		Description:       c.Description,
		EphemeralStorage:  c.EphemeralStorage,
		FunctionName:      c.FunctionName,
		Handler:           c.Handler,
		MemorySize:        c.MemorySize,
		Role:              c.Role,
		Runtime:           c.Runtime,
		Timeout:           c.Timeout,
		DeadLetterConfig:  c.DeadLetterConfig,
		FileSystemConfigs: c.FileSystemConfigs,
		KMSKeyArn:         c.KMSKeyArn,
		SnapStart:         newSnapStart(c.SnapStart),
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

	if (code != nil && aws.StringValue(code.RepositoryType) == "ECR") || aws.StringValue(fn.PackageType) == packageTypeImage {
		log.Printf("[debug] Image URL=%s", *code.ImageUri)
		fn.PackageType = aws.String(packageTypeImage)
		fn.Code = &lambda.FunctionCode{
			ImageUri: code.ImageUri,
		}
	}

	fn.Tags = tags

	return fn
}

func newSnapStart(s *lambda.SnapStartResponse) *lambda.SnapStart {
	if s == nil {
		return nil
	}
	return &lambda.SnapStart{
		ApplyOn: s.ApplyOn,
	}
}

func newSnapStartV2(s *lambdav2types.SnapStartResponse) *lambdav2types.SnapStart {
	if s == nil {
		return nil
	}
	return &lambdav2types.SnapStart{
		ApplyOn: s.ApplyOn,
	}
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

var errCannotUpdateImageAndZip = errors.New("cannot update function code between Image and Zip")

func validateUpdateFunction(currentConf *lambda.FunctionConfiguration, currentCode *lambda.FunctionCodeLocation, newFn *lambda.CreateFunctionInput) error {
	newCode := newFn.Code

	// new=Image
	if newCode != nil && newCode.ImageUri != nil || aws.StringValue(newFn.PackageType) == packageTypeImage {
		// current=Zip
		if currentCode == nil || currentCode.ImageUri == nil {
			return errCannotUpdateImageAndZip
		}
	}

	// current=Image
	if currentCode != nil && currentCode.ImageUri != nil || aws.StringValue(currentConf.PackageType) == packageTypeImage {
		// new=Zip
		if newCode == nil || newCode.ImageUri == nil {
			return errCannotUpdateImageAndZip
		}
	}

	return nil
}

func validateUpdateFunctionV2(currentConf *lambdav2types.FunctionConfiguration, currentCode *lambdav2types.FunctionCodeLocation, newFn *lambdav2.CreateFunctionInput) error {
	newCode := newFn.Code

	// new=Image
	if newCode != nil && newCode.ImageUri != nil || newFn.PackageType == packageTypeImage {
		// current=Zip
		if currentCode == nil || currentCode.ImageUri == nil {
			return errCannotUpdateImageAndZip
		}
	}

	// current=Image
	if currentCode != nil && currentCode.ImageUri != nil || currentConf.PackageType == lambdav2types.PackageTypeImage {
		// new=Zip
		if newCode == nil || newCode.ImageUri == nil {
			return errCannotUpdateImageAndZip
		}
	}

	return nil
}
