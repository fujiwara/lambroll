package lambroll

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type LogsOption struct {
	FunctionFilePath *string
	Since            *string
	Follow           *bool
	Format           *string
	FilterPattern    *string
}

func (app *App) Logs(opt LogsOption) error {
	fn, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}

	logGroup := "/aws/lambda/" + *fn.FunctionName
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
