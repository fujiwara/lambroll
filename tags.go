package lambroll

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

func (app *App) updateTags(fn *Function, opt DeployOption) error {
	if fn.Tags == nil {
		log.Println("[debug] Tags not defined in function.json skip updating tags")
		return nil
	}
	arn := app.functionArn(*fn.FunctionName)
	tags, err := app.lambda.ListTags(&lambda.ListTagsInput{
		Resource: aws.String(arn),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case lambda.ErrCodeResourceNotFoundException:
				// at create
				tags, err = &lambda.ListTagsOutput{}, nil
			default:
			}
		}
		if err != nil {
			return errors.Wrapf(err, "failed to list tags of %s", arn)
		}
	}
	log.Printf("[debug] %d tags found", len(tags.Tags))

	setTags, removeTagKeys := mergeTags(tags.Tags, fn.Tags)

	if len(setTags) == 0 && len(removeTagKeys) == 0 {
		log.Println("[debug] no need to update tags (unchnaged)")
		return nil
	}

	if n := len(setTags); n > 0 {
		log.Printf("[info] setting %d tags %s", n, opt.label())
		if !*opt.DryRun {
			_, err = app.lambda.TagResource(&lambda.TagResourceInput{
				Resource: aws.String(arn),
				Tags:     setTags,
			})
			if err != nil {
				return errors.Wrap(err, "failed to tag resource")
			}
		}
	}

	if n := len(removeTagKeys); n > 0 {
		log.Printf("[info] removing %d tags %s", n, opt.label())
		if !*opt.DryRun {
			_, err = app.lambda.UntagResource(&lambda.UntagResourceInput{
				Resource: aws.String(arn),
				TagKeys:  removeTagKeys,
			})
			if err != nil {
				return errors.Wrap(err, "failed to untag resource")
			}
		}
	}

	return nil
}

// mergeTags merges old/new tags
func mergeTags(oldTags, newTags Tags) (sets Tags, removes []*string) {
	sets = make(Tags)
	removes = make([]*string, 0)
	for key, oldValue := range oldTags {
		if newValue, ok := newTags[key]; ok {
			if *newValue != *oldValue {
				log.Printf("[debug] update tag %s=%s", key, *newValue)
				sets[key] = newValue
			}
		} else {
			log.Printf("[debug] remove tag %s", key)
			removes = append(removes, aws.String(key))
		}
	}
	for key, newValue := range newTags {
		if _, ok := oldTags[key]; !ok {
			log.Printf("[debug] add tag %s=%s", key, *newValue)
			sets[key] = newValue
		}
	}
	return
}
