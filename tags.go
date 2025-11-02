package lambroll

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/samber/lo"
)

func (app *App) updateTags(ctx context.Context, fn *Function, opt *DeployOption) error {
	logger := opt.logger()

	if fn.Tags == nil {
		logger.Debug("Tags not defined in function.json skip updating tags")
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
	logger.Debug("tags found", "count", len(tags.Tags))

	setTags, removeTagKeys := mergeTags(tags.Tags, fn.Tags)
	// ignore AWS managed tags because they are not allowed to be modified
	setTags = lo.OmitBy(setTags, func(tag string, _ string) bool {
		return isAWSManagedTag(tag)
	})
	removeTagKeys = lo.Reject(removeTagKeys, func(tag string, _ int) bool {
		return isAWSManagedTag(tag)
	})

	if len(setTags) == 0 && len(removeTagKeys) == 0 {
		logger.Debug("no need to update tags (unchanged)")
		return nil
	}

	if n := len(setTags); n > 0 {
		logger.Info("setting tags", "count", n)
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
		logger.Info("removing tags", "count", n)
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
				slog.Debug("update tag", "key", key, "value", newValue)
				sets[key] = newValue
			}
		} else {
			slog.Debug("remove tag", "key", key)
			removes = append(removes, key)
		}
	}
	for key, newValue := range newTags {
		if _, ok := oldTags[key]; !ok {
			slog.Debug("add tag", "key", key, "value", newValue)
			sets[key] = newValue
		}
	}
	return
}

func isAWSManagedTag(tag string) bool {
	if strings.HasPrefix(strings.ToLower(tag), "aws:") {
		slog.Info("ignoring AWS managed tag", "tag", tag)
		return true
	}
	return false
}
