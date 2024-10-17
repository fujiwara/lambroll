package lambroll

import (
	"context"
	"fmt"
	"strconv"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

type LayerArnResolver struct {
	svc *lambda.Client
}

func newLayerArnResolver(cfg aws.Config) *LayerArnResolver {
	return &LayerArnResolver{
		svc: lambda.NewFromConfig(cfg),
	}
}

func (r *LayerArnResolver) resolve(ctx context.Context, name, version string) (string, error) {
	switch version {
	case "latest", "":
		out, err := r.svc.ListLayerVersions(ctx, &lambda.ListLayerVersionsInput{
			LayerName: &name,
			MaxItems:  aws.Int32(1),
		})
		if err != nil {
			return "", fmt.Errorf("failed to list layer versions: %w", err)
		}
		if len(out.LayerVersions) == 0 {
			return "", fmt.Errorf("layer_arn: layer %s not found", name)
		}
		return *out.LayerVersions[0].LayerVersionArn, nil
	default:
		v, err := strconv.ParseInt(version, 10, 64)
		if err != nil {
			return "", fmt.Errorf("layer_arn: version must be a string of number or 'latest'")
		}
		out, err := r.svc.GetLayerVersion(ctx, &lambda.GetLayerVersionInput{
			LayerName:     &name,
			VersionNumber: &v,
		})
		if err != nil {
			return "", fmt.Errorf("failed to get layer version %s:%s %w", name, version, err)
		}
		return *out.LayerVersionArn, nil
	}
}

func (r *LayerArnResolver) JsonnetNativeFuncs(ctx context.Context) []*jsonnet.NativeFunction {
	return []*jsonnet.NativeFunction{
		{
			Name:   "layer_arn",
			Params: []ast.Identifier{"name", "version"},
			Func: func(params []any) (any, error) {
				if len(params) != 2 {
					return nil, fmt.Errorf("layer_arn: invalid number of arguments")
				}
				name, ok := params[0].(string)
				if !ok {
					return nil, fmt.Errorf("layer_arn: name must be a string")
				}
				version, ok := params[1].(string)
				if !ok {
					return nil, fmt.Errorf("layer_arn: version must be a string")
				}
				return r.resolve(ctx, name, version)
			},
		},
	}
}

func (r *LayerArnResolver) FuncMap(ctx context.Context) template.FuncMap {
	return template.FuncMap{
		"layer_arn": func(name, version string) (string, error) {
			return r.resolve(ctx, name, version)
		},
	}
}
