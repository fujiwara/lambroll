package lambroll

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/samber/lo"
)

func (app *App) updateTags(ctx context.Context, fn *Function, opt *DeployOption) error {
	if fn.Tags == nil {
		log.Println("[debug] Tags not defined in function.json skip updating tags")
		return nil
	}
	arn := app.functionArn(ctx, *fn.FunctionName)
	tags, err := app.lambda.ListTags(ctx, &lambda.ListTagsInput{
		Resource: aws.String(arn),
	})
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			tags, err = &lambda.ListTagsOutput{}, nil
		} else {
			return fmt.Errorf("failed to list tags of %s: %w", arn, err)
		}
	}
	log.Printf("[debug] %d tags found", len(tags.Tags))

	setTags, removeTagKeys := mergeTags(tags.Tags, fn.Tags)
	// ignore AWS managed tags because they are not allowed to be modified
	setTags = lo.OmitBy(setTags, func(tag string, _ string) bool {
		return isAWSManagedTags(tag)
	})
	removeTagKeys = lo.Reject(removeTagKeys, func(tag string, _ int) bool {
		return isAWSManagedTags(tag)
	})

	if len(setTags) == 0 && len(removeTagKeys) == 0 {
		log.Println("[debug] no need to update tags (unchanged)")
		return nil
	}

	if n := len(setTags); n > 0 {
		log.Printf("[info] setting %d tags %s", n, opt.label())
		if !opt.DryRun {
			_, err = app.lambda.TagResource(ctx, &lambda.TagResourceInput{
				Resource: aws.String(arn),
				Tags:     setTags,
			})
			if err != nil {
				return fmt.Errorf("failed to tag resource: %w", err)
			}
		}
	}

	if n := len(removeTagKeys); n > 0 {
		log.Printf("[info] removing %d tags %s", n, opt.label())
		if !opt.DryRun {
			_, err = app.lambda.UntagResource(ctx, &lambda.UntagResourceInput{
				Resource: aws.String(arn),
				TagKeys:  removeTagKeys,
			})
			if err != nil {
				return fmt.Errorf("failed to untag resource: %w", err)
			}
		}
	}

	return nil
}

// mergeTags merges old/new tags
func mergeTags(oldTags, newTags Tags) (sets Tags, removes []string) {
	sets = make(Tags)
	removes = make([]string, 0)
	for key, oldValue := range oldTags {
		if newValue, ok := newTags[key]; ok {
			if newValue != oldValue {
				log.Printf("[debug] update tag %s=%s", key, newValue)
				sets[key] = newValue
			}
		} else {
			log.Printf("[debug] remove tag %s", key)
			removes = append(removes, key)
		}
	}
	for key, newValue := range newTags {
		if _, ok := oldTags[key]; !ok {
			log.Printf("[debug] add tag %s=%s", key, newValue)
			sets[key] = newValue
		}
	}
	return
}

func isAWSManagedTags(tag string) bool {
	if strings.HasPrefix(strings.ToLower(tag), "aws:") {
		log.Printf("[info] ignoring AWS managed tag %s", tag)
		return true
	}
	return false
}
