package lambroll

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
	"github.com/fujiwara/logutils"
)

type Option struct {
	Function string `help:"Function file path" env:"LAMBROLL_FUNCTION" json:"function,omitempty"`
	LogLevel string `help:"log level (trace, debug, info, warn, error)" default:"info" enum:",trace,debug,info,warn,error" env:"LAMBROLL_LOGLEVEL" json:"log_level"`
	Color    bool   `help:"enable colored output" default:"true" env:"LAMBROLL_COLOR" negatable:"" json:"color,omitempty"`

	Region          *string           `help:"AWS region" env:"AWS_REGION" json:"region,omitempty"`
	Profile         *string           `help:"AWS credential profile name" env:"AWS_PROFILE" json:"profile,omitempty"`
	TFState         *string           `name:"tfstate" help:"URL to terraform.tfstate" env:"LAMBROLL_TFSTATE" json:"tfstate,omitempty"`
	PrefixedTFState map[string]string `name:"prefixed-tfstate" help:"key value pair of the prefix for template function name and URL to terraform.tfstate" env:"LAMBROLL_PREFIXED_TFSTATE" json:"prefixed_tfstate,omitempty"`
	Endpoint        *string           `help:"AWS API Lambda Endpoint" env:"AWS_LAMBDA_ENDPOINT" json:"endpoint,omitempty"`
	Envfile         []string          `help:"environment files" env:"LAMBROLL_ENVFILE" json:"envfile,omitempty"`
	ExtStr          map[string]string `help:"external string values for Jsonnet" env:"LAMBROLL_EXTSTR" json:"extstr,omitempty"`
	ExtCode         map[string]string `help:"external code values for Jsonnet" env:"LAMBROLL_EXTCODE" json:"extcode,omitempty"`
}

type CLIOptions struct {
	Option

	Deploy   *DeployOption   `cmd:"deploy" help:"deploy or create function"`
	Init     *InitOption     `cmd:"init" help:"init function.json"`
	List     *ListOption     `cmd:"list" help:"list functions"`
	Rollback *RollbackOption `cmd:"rollback" help:"rollback function"`
	Invoke   *InvokeOption   `cmd:"invoke" help:"invoke function"`
	Archive  *ArchiveOption  `cmd:"archive" help:"archive function"`
	Logs     *LogsOption     `cmd:"logs" help:"show logs of function"`
	Diff     *DiffOption     `cmd:"diff" help:"show diff of function"`
	Render   *RenderOption   `cmd:"render" help:"render function.json"`
	Status   *StatusOption   `cmd:"status" help:"show status of function"`
	Delete   *DeleteOption   `cmd:"delete" help:"delete function"`
	Versions *VersionsOption `cmd:"versions" help:"show versions of function"`

	Version struct{} `cmd:"version" help:"show version"`
}

type CLIParseFunc func([]string) (string, *CLIOptions, func(), error)

func ParseCLI(args []string) (string, *CLIOptions, func(), error) {
	// compatible with v1
	if len(args) == 0 || len(args) > 0 && args[0] == "help" {
		args = []string{"--help"}
	}

	kongOpts := []kong.Option{kong.Vars{"version": Version}}

	// parse just the envfile opts to load envfile
	var opts CLIOptions
	p, err := kong.New(&opts, kongOpts...)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to new kong: %w", err)
	}
	if _, err := p.Parse(args); err != nil {
		return "", nil, nil, fmt.Errorf("failed to parse args: %w", err)
	}
	for _, envfile := range opts.Envfile {
		if err := exportEnvFile(envfile); err != nil {
			return "", nil, nil, fmt.Errorf("failed to load envfile: %w", err)
		}
	}
	mergedEnvfiles := make([]string, 0)

	// load default options from lambroll.json or .jsonnet
	defaultOpt, err := loadDefinitionFile[Option](nil, "", DefaultOptionFilenames)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// ignore not found error
		} else {
			return "", nil, nil, fmt.Errorf("failed to load default options: %w", err)
		}
	} else {
		defaultOptBytes, err := json.Marshal(defaultOpt)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to marshal default options: %w", err)
		}
		resolver, err := kong.JSON(bytes.NewReader(defaultOptBytes))
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to parse default options: %w", err)
		}
		mergedEnvfiles = append(mergedEnvfiles, defaultOpt.Envfile...)
		kongOpts = append(kongOpts, kong.Resolvers(resolver))
	}

	parser, err := kong.New(&opts, kongOpts...)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to new kong: %w", err)
	}
	c, err := parser.Parse(args)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to parse args: %w", err)
	}
	opts.Envfile = append(mergedEnvfiles, opts.Envfile...)
	sub := strings.Fields(c.Command())[0]
	return sub, &opts, func() { c.PrintUsage(true) }, nil
}

func CLI(ctx context.Context, parse CLIParseFunc) (int, error) {
	sub, opts, usage, err := parse(os.Args[1:])
	if err != nil {
		return 1, err
	}

	color.NoColor = !opts.Color
	if opts.LogLevel == "" {
		opts.LogLevel = DefaultLogLevel
	}
	filter := &logutils.LevelFilter{
		Levels: []logutils.LogLevel{"trace", "debug", "info", "warn", "error"},
		ModifierFuncs: []logutils.ModifierFunc{
			logutils.Color(color.FgHiWhite), // trace
			logutils.Color(color.FgHiBlack), // debug
			nil,                             // info
			logutils.Color(color.FgYellow),  // warn
			logutils.Color(color.FgRed),     // error
		},
		MinLevel: logutils.LogLevel(opts.LogLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)

	if err := dispatchCLI(ctx, sub, usage, opts); err != nil {
		return 1, err
	}
	return 0, nil
}

func dispatchCLI(ctx context.Context, sub string, usage func(), opts *CLIOptions) error {
	switch sub {
	case "version", "":
		fmt.Println("lambroll", Version)
		return nil
	}

	app, err := New(ctx, &opts.Option)
	if err != nil {
		return err
	}
	if opts.Function != "" {
		log.Printf("[info] lambroll %s with %s", Version, opts.Function)
	} else {
		log.Printf("[info] lambroll %s", Version)
	}
	switch sub {
	case "init":
		return app.Init(ctx, opts.Init)
	case "list":
		return app.List(ctx, opts.List)
	case "deploy":
		return app.Deploy(ctx, opts.Deploy)
	case "invoke":
		return app.Invoke(ctx, opts.Invoke)
	case "logs":
		return app.Logs(ctx, opts.Logs)
	case "versions":
		return app.Versions(ctx, opts.Versions)
	case "archive":
		return app.Archive(ctx, opts.Archive)
	case "rollback":
		return app.Rollback(ctx, opts.Rollback)
	case "render":
		return app.Render(ctx, opts.Render)
	case "diff":
		return app.Diff(ctx, opts.Diff)
	case "delete":
		return app.Delete(ctx, opts.Delete)
	case "status":
		return app.Status(ctx, opts.Status)
	default:
		usage()
	}
	return nil
}
