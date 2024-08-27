package lambroll

import "fmt"

// Option represents common option.

type ExcludeFileOption struct {
	ExcludeFile string `help:"exclude file" default:".lambdaignore"`
	KeepSymlink bool   `name:"symlink" help:"keep symlink (same as zip --symlink,-y)" default:"false"`

	excludes []string
}

func (opt *ExcludeFileOption) Expand() error {
	excludes, err := expandExcludeFile(opt.ExcludeFile)
	if err != nil {
		return fmt.Errorf("failed to parse exclude-file: %w", err)
	}
	opt.excludes = append(opt.excludes, excludes...)
	return nil
}
