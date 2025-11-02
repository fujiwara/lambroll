package lambroll

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
	"github.com/fujiwara/sloghandler"
	"github.com/samber/lo"
)

type Option struct {
	OptionFilePath string `help:"option file path" env:"LAMBROLL_OPTION" name:"option" json:"-"`
	Function       string `help:"Function file path" env:"LAMBROLL_FUNCTION" json:"function,omitempty"`
	LogLevel       string `help:"log level (trace, debug, info, warn, error)" default:"info" enum:",trace,debug,info,warn,error" env:"LAMBROLL_LOGLEVEL" json:"log_level"`
	LogFormat      string `help:"log format (text, json)" default:"text" enum:",text,json" env:"LAMBROLL_LOGFORMAT" json:"log_format"`
	Color          bool   `help:"enable colored output" default:"true" env:"LAMBROLL_COLOR" negatable:"" json:"color,omitempty"`

	Region          *string           `help:"AWS region" env:"AWS_REGION" json:"region,omitempty"`
	Profile         *string           `help:"AWS credential profile name" env:"AWS_PROFILE" json:"profile,omitempty"`
	TFState         *string           `name:"tfstate" help:"URL to terraform.tfstate" env:"LAMBROLL_TFSTATE" json:"tfstate,omitempty"`
	PrefixedTFState map[string]string `name:"prefixed-tfstate" help:"key value pair of the prefix for template function name and URL to terraform.tfstate" env:"LAMBROLL_PREFIXED_TFSTATE" json:"prefixed_tfstate,omitempty"`
	Endpoint        *string           `help:"AWS API Lambda Endpoint" env:"AWS_LAMBDA_ENDPOINT" json:"endpoint,omitempty"`
	Envfile         []string          `help:"environment files" env:"LAMBROLL_ENVFILE" json:"envfile,omitempty"`
	ExtStr          map[string]string `help:"external string values for Jsonnet" env:"LAMBROLL_EXTSTR" json:"ext_str,omitempty"`
	ExtCode         map[string]string `help:"external code values for Jsonnet" env:"LAMBROLL_EXTCODE" json:"ext_code,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling to support both old and new field names
// TODO: Remove backward compatibility for extstr/extcode fields in v2
func (o *Option) UnmarshalJSON(data []byte) error {
	// Define a type alias to avoid infinite recursion
	type Alias Option
	aux := &struct {
		*Alias
		// Support old field names
		OldExtStr  map[string]string `json:"extstr,omitempty"`
		OldExtCode map[string]string `json:"extcode,omitempty"`
	}{
		Alias: (*Alias)(o),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// If old field names are used and new ones are empty, copy the values
	if o.ExtStr == nil && aux.OldExtStr != nil {
		o.ExtStr = aux.OldExtStr
		log.Printf("[warn] Using deprecated field name 'extstr' in option file. Please use 'ext_str' instead.")
	}
	if o.ExtCode == nil && aux.OldExtCode != nil {
		o.ExtCode = aux.OldExtCode
		log.Printf("[warn] Using deprecated field name 'extcode' in option file. Please use 'ext_code' instead.")
	}

	return nil
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

func prepareCLI(args []string) (string, []string, error) {
	var opts CLIOptions
	p, err := kong.New(&opts)
	if err != nil {
		return "", nil, fmt.Errorf("failed to new kong: %w", err)
	}
	if _, err := p.Parse(args); err != nil {
		return "", nil, fmt.Errorf("failed to parse args: %w", err)
	}
	for _, envfile := range opts.Envfile {
		if err := exportEnvFile(envfile); err != nil {
			return "", nil, fmt.Errorf("failed to load envfile: %w", err)
		}
	}
	return opts.OptionFilePath, opts.Envfile, nil
}

func ParseCLI(args []string) (string, *CLIOptions, func(), error) {
	// compatible with v1
	if len(args) == 0 || len(args) > 0 && args[0] == "help" {
		args = []string{"--help"}
	}

	// resolve envfile from args at first
	optionFilePath, argEnvfiles, err := prepareCLI(args)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to prepare env from args: %w", err)
	}
	var envfiles []string
	kongOpts := []kong.Option{kong.Vars{"version": Version}}

	// load default options
	if optionFilePath != "" {
		defaultOpt, err := loadDefinitionFile[Option](nil, optionFilePath, nil)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to load option file: %w", err)
		}
		defaultOptBytes, err := json.Marshal(defaultOpt)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to marshal default options: %w", err)
		}
		resolver, err := kong.JSON(bytes.NewReader(defaultOptBytes))
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to parse default options: %w", err)
		}
		kongOpts = append(kongOpts, kong.Resolvers(resolver))
		envfiles = defaultOpt.Envfile
		envfiles = append(envfiles, argEnvfiles...)
	} else {
		envfiles = argEnvfiles
	}

	var opts CLIOptions
	parser, err := kong.New(&opts, kongOpts...)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to new kong: %w", err)
	}
	c, err := parser.Parse(args)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to parse args: %w", err)
	}
	opts.Envfile = lo.Uniq(envfiles) // envfiles are parsed before, so it's safe to overwrite

	sub := strings.Fields(c.Command())[0]
	return sub, &opts, func() { c.PrintUsage(true) }, nil
}

func CLI(ctx context.Context, parse CLIParseFunc) (int, error) {
	sub, opts, usage, err := parse(os.Args[1:])
	if err != nil {
		return extractExitCodeAndError(err)
	}

	color.NoColor = !opts.Color
	if opts.LogLevel == "" {
		opts.LogLevel = DefaultLogLevel
	}
	if opts.LogFormat == "" {
		opts.LogFormat = "text"
	}
	logLevel := new(slog.LevelVar)
	switch opts.LogLevel {
	case "trace":
		logLevel.Set(slog.LevelDebug)
	case "debug":
		logLevel.Set(slog.LevelDebug)
	case "info":
		logLevel.Set(slog.LevelInfo)
	case "warn":
		logLevel.Set(slog.LevelWarn)
	case "error":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo)
	}

	var logger *slog.Logger
	switch opts.LogFormat {
	case "json":
		logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		}))
	case "text":
		fallthrough
	default:
		slogHandlerOptions := &sloghandler.HandlerOptions{
			Color: opts.Color,
			HandlerOptions: slog.HandlerOptions{
				Level: logLevel,
			},
		}
		logger = slog.New(sloghandler.NewLogHandler(os.Stderr, slogHandlerOptions))
	}
	slog.SetDefault(logger)

	err = dispatchCLI(ctx, sub, usage, opts)
	return extractExitCodeAndError(err)
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
