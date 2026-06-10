package lambroll

import (
	"context"
	"encoding/json"
	"fmt"
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

// newOptionFileResolver returns a kong.Resolver that resolves flag default
// values from an option file. Global flags are resolved from the top-level
// keys, while subcommand-specific flags are resolved from the nested object
// keyed by the subcommand name.
func newOptionFileResolver(values map[string]any) kong.Resolver {
	lookup := func(scope map[string]any, flag *kong.Flag) any {
		// option file keys use the snake_case form of the flag name
		// (e.g. --skip-archive -> "skip_archive").
		name := strings.ReplaceAll(flag.Name, "-", "_")
		if raw, ok := scope[name]; ok {
			return raw
		}
		return nil
	}
	return kong.ResolverFunc(func(_ *kong.Context, parent *kong.Path, flag *kong.Flag) (any, error) {
		if parent != nil && parent.Command != nil {
			// subcommand-specific flag: look up the nested section.
			sub, ok := values[parent.Command.Name].(map[string]any)
			if !ok {
				return nil, nil
			}
			return lookup(sub, flag), nil
		}
		// global flag: look up the top-level keys.
		return lookup(values, flag), nil
	})
}

// CLIOptions is both the kong parse target for the command line and the schema
// of the option file (--option). Global flags are placed at the top level
// (flat, via the embedded Option), while subcommand-specific flags are nested
// under the subcommand name. The json tag of each command field must match its
// cmd name so the option file resolver can scope lookups by subcommand.
type CLIOptions struct {
	Option

	Deploy   *DeployOption   `cmd:"deploy" help:"deploy or create function" json:"deploy,omitempty"`
	Init     *InitOption     `cmd:"init" help:"init function.json" json:"init,omitempty"`
	List     *ListOption     `cmd:"list" help:"list functions" json:"list,omitempty"`
	Rollback *RollbackOption `cmd:"rollback" help:"rollback function" json:"rollback,omitempty"`
	Invoke   *InvokeOption   `cmd:"invoke" help:"invoke function" json:"invoke,omitempty"`
	Archive  *ArchiveOption  `cmd:"archive" help:"archive function" json:"archive,omitempty"`
	Logs     *LogsOption     `cmd:"logs" help:"show logs of function" json:"logs,omitempty"`
	Diff     *DiffOption     `cmd:"diff" help:"show diff of function" json:"diff,omitempty"`
	Render   *RenderOption   `cmd:"render" help:"render function.json" json:"render,omitempty"`
	Status   *StatusOption   `cmd:"status" help:"show status of function" json:"status,omitempty"`
	Delete   *DeleteOption   `cmd:"delete" help:"delete function" json:"delete,omitempty"`
	Versions *VersionsOption `cmd:"versions" help:"show versions of function" json:"versions,omitempty"`

	Version struct{} `cmd:"version" help:"show version" json:"-"`
}

// RequireStrict marks CLIOptions as a strict definition loader: when loaded from
// a file, unknown keys are rejected instead of warned and ignored. An option
// file is equivalent to specifying flags, so an unknown key is treated the same
// as an unknown flag.
func (CLIOptions) RequireStrict() {}

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
	var defaultOpt *CLIOptions
	if optionFilePath != "" {
		var err error
		var src []byte
		defaultOpt, src, err = loadDefinitionFileWithBytes[CLIOptions](nil, optionFilePath, nil)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to load option file: %w", err)
		}
		// Build resolver values from the evaluated file contents rather than
		// re-marshaling defaultOpt, so explicit falsey values (e.g. color:
		// false) are preserved and only user-specified keys are present.
		var values map[string]any
		if err := json.Unmarshal(src, &values); err != nil {
			return "", nil, nil, fmt.Errorf("failed to parse default options: %w", err)
		}
		kongOpts = append(kongOpts, kong.Resolvers(newOptionFileResolver(values)))
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

	// Merge ext_str and ext_code from option file with CLI args
	// CLI args take precedence over option file values
	if defaultOpt != nil {
		if defaultOpt.ExtStr != nil || opts.ExtStr != nil {
			opts.ExtStr = lo.Assign(defaultOpt.ExtStr, opts.ExtStr)
		}
		if defaultOpt.ExtCode != nil || opts.ExtCode != nil {
			opts.ExtCode = lo.Assign(defaultOpt.ExtCode, opts.ExtCode)
		}
	}

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
		slog.Info("lambroll", "version", Version, "function", opts.Function)
	} else {
		slog.Info("lambroll", "version", Version)
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
