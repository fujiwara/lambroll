package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/fujiwara/lambroll"
	"github.com/hashicorp/logutils"
)

// Version number
var Version = "current"

func main() {
	os.Exit(_main())
}

func _main() int {
	kingpin.Command("version", "show version")
	region := kingpin.Flag("region", "AWS region").Default(os.Getenv("AWS_REGION")).String()
	logLevel := kingpin.Flag("log-level", "log level (trace, debug, info, warn, error)").Default("info").Enum("trace", "debug", "info", "warn", "error")
	function := kingpin.Flag("function", "Function file path").Default(lambroll.FunctionFilename).String()
	profile := kingpin.Flag("profile", "AWS credential profile name").Default(os.Getenv("AWS_PROFILE")).String()

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
		SrcDir:           deploy.Flag("src", "function zip archive src dir").Default(".").String(),
		ExcludeFile:      deploy.Flag("exclude-file", "exclude file").Default(lambroll.IgnoreFilename).String(),
		DryRun:           deploy.Flag("dry-run", "dry run").Bool(),
		Publish:          deploy.Flag("publish", "publish function").Default("true").Bool(),
		AliasName:        deploy.Flag("alias", "alias name for publish").Default(lambroll.CurrentAliasName).String(),
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
	}

	archive := kingpin.Command("archive", "archive zip")
	archiveOption := lambroll.DeployOption{
		SrcDir:      archive.Flag("src", "function zip archive src dir").Default(".").String(),
		ExcludeFile: archive.Flag("exclude-file", "exclude file").Default(lambroll.IgnoreFilename).String(),
	}

	command := kingpin.Parse()

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"trace", "debug", "info", "warn", "error"},
		MinLevel: logutils.LogLevel(*logLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)

	app, err := lambroll.New(*region, *profile)
	if err != nil {
		log.Println("[error]", err)
		return 1
	}
	if command == "version" {
		fmt.Println("lambroll", Version)
		return 0
	}

	log.Println("[info] lambroll", Version)
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
	}

	if err != nil {
		log.Println("[error]", err)
		return 1
	}
	log.Println("[info] completed")
	return 0
}
