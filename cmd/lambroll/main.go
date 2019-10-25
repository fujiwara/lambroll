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
	logLevel := kingpin.Flag("log-level", "log level (debug, info, warn, error)").Default("info").String()

	init := kingpin.Command("init", "init function.json")
	initOption := lambroll.InitOption{
		FunctionName: init.Flag("function-name", "Function name for initialize").Required().String(),
		DownloadZip:  init.Flag("download", "Download function.zip").Default("false").Bool(),
	}

	kingpin.Command("list", "list functions")
	listOption := lambroll.ListOption{}

	deploy := kingpin.Command("deploy", "deploy function")
	deployOption := lambroll.DeployOption{
		FunctionFilePath: deploy.Flag("function", "Function file path").Default("function.json").String(),
		SrcDir:           deploy.Flag("src", "function zip archive src dir").Default(".").String(),
		ExcludeFile:      deploy.Flag("exclude-file", "exclude file").Default(".lambdaignore").String(),
		Excludes:         lambroll.DefaultExcludes,
	}

	command := kingpin.Parse()

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"debug", "info", "warn", "error"},
		MinLevel: logutils.LogLevel(*logLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)

	app, err := lambroll.New(*region)
	if err != nil {
		log.Println("[error]", err)
		return 1
	}
	switch command {
	case "version":
		fmt.Println("lambroll", Version)
		return 0
	case "init":
		err = app.Init(initOption)
	case "list":
		err = app.List(listOption)
	case "deploy":
		err = app.Deploy(deployOption)
	}

	if err != nil {
		log.Println("[error]", err)
		return 1
	}
	log.Println("[info] completed")
	return 0
}
