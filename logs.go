package lambroll

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type LogsOption struct {
	Since         *string `help:"From what time to begin displaying logs" default:"10m"`
	Follow        *bool   `help:"follow new logs" default:"false"`
	Format        *string `help:"The format to display the logs" default:"detailed" enum:"detailed,short,json"`
	FilterPattern *string `help:"The filter pattern to use"`
}

func (app *App) Logs(ctx context.Context, opt *LogsOption) error {
	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}

	logGroup := resolveLogGroup(fn)
	command := []string{
		"aws",
		"--profile", app.profile,
		"--region", app.awsConfig.Region,
		"logs",
		"tail",
		logGroup,
	}
	if opt.Since != nil {
		command = append(command, "--since", *opt.Since)
	}
	if opt.Follow != nil && *opt.Follow {
		command = append(command, "--follow")
	}
	if opt.Format != nil {
		command = append(command, "--format", *opt.Format)
	}
	if opt.FilterPattern != nil && *opt.FilterPattern != "" {
		command = append(command, "--filter-pattern", *opt.FilterPattern)
	}
	bin, err := exec.LookPath(command[0])
	if err != nil {
		return err
	}
	log.Println("[debug] invoking command", strings.Join(command, " "))
	if err := syscall.Exec(bin, command, os.Environ()); err != nil {
		return fmt.Errorf("failed to invoke aws logs tail: %w", err)
	}
	return nil
}
