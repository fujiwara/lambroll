package lambroll

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
)

// VersionsOption represents options for Versions()
type VersionsOption struct {
	FunctionFilePath *string
	Output           *string
	Delete           *bool
	KeepVersions     *int
}

type versionsOutput struct {
	Version      string    `json:"Version"`
	Aliases      []string  `json:"Aliases,omitempty"`
	LastModified time.Time `json:"LastModified"`
	Runtime      string    `json:"Runtime"`
}

type versionsOutputs []versionsOutput

func (vo versionsOutputs) JSON() string {
	b, _ := json.Marshal(vo)
	var out bytes.Buffer
	json.Indent(&out, b, "", "  ")
	return out.String()
}

func (vo versionsOutputs) TSV() string {
	buf := new(strings.Builder)
	for _, v := range vo {
		buf.WriteString(v.TSV())
	}
	return buf.String()
}

func (vo versionsOutputs) Table() string {
	buf := new(strings.Builder)
	w := tablewriter.NewWriter(buf)
	w.SetHeader([]string{"Version", "Last Modified", "Aliases", "Runtime"})
	for _, v := range vo {
		w.Append([]string{
			v.Version,
			v.LastModified.Local().Format(time.RFC3339),
			strings.Join(v.Aliases, ","),
			v.Runtime,
		})
	}
	w.Render()
	return buf.String()
}

func (v versionsOutput) TSV() string {
	return fmt.Sprintf("%s\t%s\t%s\t%s\n",
		v.Version,
		v.LastModified.Local().Format(time.RFC3339),
		strings.Join(v.Aliases, ","),
		v.Runtime,
	)
}

// Versions manages the versions of a Lambda function
func (app *App) Versions(opt VersionsOption) error {
	newFunc, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}
	name := *newFunc.FunctionName
	if *opt.Delete {
		return app.deleteVersions(name, *opt.KeepVersions)
	}

	aliases := make(map[string][]string)
	var nextAliasMarker *string
	for {
		res, err := app.lambda.ListAliases(&lambda.ListAliasesInput{
			FunctionName: &name,
			Marker:       nextAliasMarker,
		})
		if err != nil {
			return errors.Wrap(err, "failed to list aliases")
		}
		for _, alias := range res.Aliases {
			aliases[*alias.FunctionVersion] = append(aliases[*alias.FunctionVersion], *alias.Name)
			if alias.RoutingConfig == nil || alias.RoutingConfig.AdditionalVersionWeights == nil {
				continue
			}
			for v := range alias.RoutingConfig.AdditionalVersionWeights {
				aliases[v] = append(aliases[v], *alias.Name)
			}
		}
		if nextAliasMarker = res.NextMarker; nextAliasMarker == nil {
			break
		}
	}

	var versions []*lambda.FunctionConfiguration
	var nextMarker *string
	for {
		res, err := app.lambda.ListVersionsByFunction(&lambda.ListVersionsByFunctionInput{
			FunctionName: &name,
			Marker:       nextMarker,
		})
		if err != nil {
			return errors.Wrap(err, "failed to list versions")
		}
		versions = append(versions, res.Versions...)
		if nextMarker = res.NextMarker; nextMarker == nil {
			break
		}
	}

	vo := make(versionsOutputs, 0, len(versions))
	for _, v := range versions {
		if aws.StringValue(v.Version) == versionLatest {
			continue
		}
		lm, err := time.Parse("2006-01-02T15:04:05.999-0700", *v.LastModified)
		if err != nil {
			return errors.Wrap(err, "failed to parse last modified")
		}
		vo = append(vo, versionsOutput{
			Version:      *v.Version,
			Aliases:      aliases[*v.Version],
			LastModified: lm,
			Runtime:      *v.Runtime,
		})
	}

	switch *opt.Output {
	case "json":
		fmt.Println(vo.JSON())
	case "tsv":
		fmt.Print(vo.TSV())
	case "table":
		fmt.Print(vo.Table())
	}
	return nil
}
