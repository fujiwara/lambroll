package lambroll

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/pkg/errors"
)

// App represents lambroll application
type App struct {
	accountID string
	lambda    *lambda.Lambda
}

// New creates an application
func New(region string) (*App, error) {
	conf := &aws.Config{}
	if region != "" {
		conf.Region = aws.String(region)
	}
	sess := session.Must(session.NewSession(conf))

	svc := sts.New(session.New())
	r, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to sts.GetCallerIdentity")
	}

	return &App{
		accountID: *r.Account,
		lambda:    lambda.New(sess),
	}, nil
}
