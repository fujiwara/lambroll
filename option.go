package lambroll

import (
	"fmt"
	"log/slog"
)

// Option represents common option.

type DryRunOption struct {
	DryRun bool `default:"false" help:"dry run" json:"dry_run,omitempty"`
}

func (opt DryRunOption) logger() *slog.Logger {
	logger := slog.Default()
	if opt.DryRun {
		logger = logger.With("mode", "**DRY RUN**")
	}
	return logger
}

type ZipOption struct {
	ExcludeFile string `help:"exclude file" default:".lambdaignore" json:"exclude_file,omitempty"`
	KeepSymlink bool   `name:"symlink" help:"keep symlink (same as zip --symlink,-y)" default:"false" json:"symlink,omitempty"`

	excludes []string
}

func (opt *ZipOption) Expand() error {
	excludes, err := expandExcludeFile(opt.ExcludeFile)
	if err != nil {
		return fmt.Errorf("failed to parse exclude-file: %w", err)
	}
	opt.excludes = append(opt.excludes, excludes...)
	return nil
}
