package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/fatih/color"
	"github.com/fujiwara/lambroll"
	"github.com/fujiwara/logutils"
	"github.com/mattn/go-isatty"
)

// Version number
var Version = "current"

func main() {
	os.Exit(_main())
}

func _main() int {
	kingpin.Command("version", "show version")
	logLevel := kingpin.Flag("log-level", "log level (trace, debug, info, warn, error)").Default("info").Enum("trace", "debug", "info", "warn", "error")
	function := kingpin.Flag("function", "Function file path").Default(lambroll.FindFunctionFilename()).String()
	colorDefault := "false"
	if isatty.IsTerminal(os.Stdout.Fd()) {
		colorDefault = "true"
	}
	colorOpt := kingpin.Flag("color", "enable colored output").Default(colorDefault).Bool()

	opt := lambroll.Option{
		Profile:         kingpin.Flag("profile", "AWS credential profile name").Default(os.Getenv("AWS_PROFILE")).String(),
		Region:          kingpin.Flag("region", "AWS region").Default(os.Getenv("AWS_REGION")).String(),
		TFState:         kingpin.Flag("tfstate", "URL to terraform.tfstate").Default("").String(),
		PrefixedTFState: kingpin.Flag("prefixed-tfstate", "key value pair of the prefix for template function name and URL to terraform.tfstate").PlaceHolder("PREFIX=URL").StringMap(),
		Endpoint:        kingpin.Flag("endpoint", "AWS API Lambda Endpoint").Default("").String(),
		Envfile:         kingpin.Flag("envfile", "environment files").Strings(),
		ExtStr:          kingpin.Flag("ext-str", "external string values for Jsonnet").StringMap(),
		ExtCode:         kingpin.Flag("ext-code", "external code values for Jsonnet").StringMap(),
	}

	init := kingpin.Command("init", "init function.json")
	initOption := lambroll.InitOption{
		FunctionName: init.Flag("function-name", "Function name for initialize").Required().String(),
		DownloadZip:  init.Flag("download", "Download function.zip").Default("false").Bool(),
	}

	kingpin.Command("list", "list functions")
	listOption := lambroll.ListOption{}

	deploy := kingpin.Command("deploy", "deploy or create function")
	deployOption := lambroll.DeployOption{
		FunctionFilePath: function,
		Src:              deploy.Flag("src", "function zip archive or src dir").Default(".").String(),
		ExcludeFile:      deploy.Flag("exclude-file", "exclude file").Default(lambroll.IgnoreFilename).String(),
		DryRun:           deploy.Flag("dry-run", "dry run").Bool(),
		Publish:          deploy.Flag("publish", "publish function").Default("true").Bool(),
		AliasName:        deploy.Flag("alias", "alias name for publish").Default(lambroll.CurrentAliasName).String(),
		AliasToLatest:    deploy.Flag("alias-to-latest", "set alias to unpublished $LATEST version").Default("false").Bool(),
		SkipArchive:      deploy.Flag("skip-archive", "skip to create zip archive. requires Code.S3Bucket and Code.S3Key in function definition").Default("false").Bool(),
		KeepVersions:     deploy.Flag("keep-versions", "Number of latest versions to keep. Older versions will be deleted. (Optional value: default 0).").Default("0").Int(),
	}

	rollback := kingpin.Command("rollback", "rollback function")
	rollbackOption := lambroll.RollbackOption{
		FunctionFilePath: function,
		DeleteVersion:    rollback.Flag("delete-version", "Delete rolled back version").Bool(),
		DryRun:           rollback.Flag("dry-run", "dry run").Bool(),
	}

	delete := kingpin.Command("delete", "delete function")
	deleteOption := lambroll.DeleteOption{
		FunctionFilePath: function,
		DryRun:           delete.Flag("dry-run", "dry run").Bool(),
	}

	invoke := kingpin.Command("invoke", "invoke function")
	invokeOption := lambroll.InvokeOption{
		FunctionFilePath: function,
		Async:            invoke.Flag("async", "invocation type async").Bool(),
		LogTail:          invoke.Flag("log-tail", "output tail of log to STDERR").Bool(),
		Qualifier:        invoke.Flag("qualifier", "version or alias to invoke").String(),
	}

	archive := kingpin.Command("archive", "archive zip")
	archiveOption := lambroll.DeployOption{
		Src:         archive.Flag("src", "function src dir for archive").Default(".").String(),
		ExcludeFile: archive.Flag("exclude-file", "exclude file").Default(lambroll.IgnoreFilename).String(),
	}

	logs := kingpin.Command("logs", "tail logs using `aws logs tail` (aws-cli v2 required)")
	logsOption := lambroll.LogsOption{
		FunctionFilePath: function,
		Since:            logs.Flag("since", "From what time to begin displaying logs").Default("10m").String(),
		Follow:           logs.Flag("follow", "follow new logs").Default("false").Bool(),
		Format:           logs.Flag("format", "The format to display the logs").Default("detailed").String(),
		FilterPattern:    logs.Flag("filter-pattern", "The filter  pattern to use").Default("").String(),
	}

	kingpin.Command("diff", "show display diff of function.json compared with latest function")
	diffOption := lambroll.DiffOption{
		FunctionFilePath: function,
	}

	versions := kingpin.Command("versions", "manage function versions")
	versionsOption := lambroll.VersionsOption{
		FunctionFilePath: function,
		Output:           versions.Flag("output", "output format").Default("table").Enum("table", "json", "tsv"),
		Delete:           versions.Flag("delete", "delete older versions").Default("false").Bool(),
		KeepVersions:     versions.Flag("keep-versions", "Number of latest versions to keep. Older versions will be deleted with --delete.").Default("-1").Int(),
	}

	command := kingpin.Parse()
	if command == "version" {
		fmt.Println("lambroll", Version)
		return 0
	}
	color.NoColor = !*colorOpt

	filter := &logutils.LevelFilter{
		Levels: []logutils.LogLevel{"debug", "info", "warn", "error"},
		ModifierFuncs: []logutils.ModifierFunc{
			logutils.Color(color.FgHiBlack), // debug
			nil,                             // info
			logutils.Color(color.FgYellow),  // warn
			logutils.Color(color.FgRed),     // error
		},
		MinLevel: logutils.LogLevel(*logLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)

	app, err := lambroll.New(&opt)
	if err != nil {
		log.Println("[error]", err)
		return 1
	}

	log.Printf("[info] lambroll %s with %s", Version, *function)
	switch command {
	case "init":
		err = app.Init(initOption)
	case "list":
		err = app.List(listOption)
	case "deploy":
		err = app.Deploy(deployOption)
	case "rollback":
		err = app.Rollback(rollbackOption)
	case "delete":
		err = app.Delete(deleteOption)
	case "invoke":
		err = app.Invoke(invokeOption)
	case "archive":
		err = app.Archive(archiveOption)
	case "logs":
		err = app.Logs(logsOption)
	case "diff":
		err = app.Diff(diffOption)
	case "versions":
		err = app.Versions(versionsOption)
	}

	if err != nil {
		log.Println("[error]", err)
		return 1
	}
	log.Println("[info] completed")
	return 0
}
