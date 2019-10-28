package lambroll

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/sts"
)

var (
	// IgnoreFilename defines file name includes ingore patterns at creating zip archive.
	IgnoreFilename = ".lambdaignore"

	// FunctionFilename defines file name for function definition.
	FunctionFilename = "function.json"

	// FunctionZipFilename defines file name for zip archive downloaded at init.
	FunctionZipFilename = "function.zip"

	// DefaultExcludes is a preset excludes file list
	DefaultExcludes = []string{IgnoreFilename, FunctionFilename, ".git/*"}
)

// App represents lambroll application
type App struct {
	sess      *session.Session
	lambda    *lambda.Lambda
	accountID string
}

// New creates an application
func New(region string) (*App, error) {
	conf := &aws.Config{}
	if region != "" {
		conf.Region = aws.String(region)
	}
	sess := session.Must(session.NewSession(conf))

	return &App{
		sess:   sess,
		lambda: lambda.New(sess),
	}, nil
}

// AWSAccountID returns AWS account ID in current session
func (app *App) AWSAccountID() string {
	if app.accountID != "" {
		return app.accountID
	}
	svc := sts.New(session.New())
	r, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		log.Println("[warn] failed to get caller identity", err)
		return ""
	}
	app.accountID = *r.Account
	return app.accountID
}
