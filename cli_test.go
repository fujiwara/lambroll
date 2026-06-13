package lambroll_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/fujiwara/lambroll"
	"github.com/google/go-cmp/cmp"
)

var cliTests = []struct {
	args   []string
	env    map[string]string
	sub    string
	option *lambroll.Option
	err    error
}{
	{
		args: []string{"render"},
		sub:  "render",
		option: &lambroll.Option{
			LogLevel:  "info",
			LogFormat: "text",
			Color:     true,
			Envfile:   []string{},
		},
	},
	{
		args: []string{"render", "--option", "debug.jsonnet"},
		sub:  "render",
		option: &lambroll.Option{
			LogLevel:       "debug",
			LogFormat:      "text",
			Color:          true,
			OptionFilePath: "debug.jsonnet",
			Envfile:        []string{},
		},
	},
	{
		args: []string{"render", "--envfile=envfile.global"},
		sub:  "render",
		option: &lambroll.Option{
			LogLevel:  "info",
			LogFormat: "text",
			Color:     true,
			Envfile:   []string{"envfile.global"},
		},
	},
	{
		args: []string{
			"--envfile=envfile.local",
			"--option", "global.jsonnet",
			"--function", "function.jsonnet",
			"render",
		},
		sub: "render",
		option: &lambroll.Option{
			Function:       "function.jsonnet",
			OptionFilePath: "global.jsonnet",
			LogLevel:       "info",
			LogFormat:      "text",
			Color:          true,
			Envfile:        []string{"envfile.global", "envfile.local"},
		},
	},
	{
		args: []string{
			"--envfile=envfile.global", "--envfile=envfile.local",
			"render",
		},
		sub: "render",
		option: &lambroll.Option{
			LogLevel:  "info",
			LogFormat: "text",
			Color:     true,
			Envfile:   []string{"envfile.global", "envfile.local"},
		},
	},
	{
		args: []string{"render"},
		sub:  "render",
		env: map[string]string{
			"LAMBROLL_LOGLEVEL": "error",
			"LAMBROLL_COLOR":    "false",
		},
		option: &lambroll.Option{
			LogLevel:  "error",
			LogFormat: "text",
			Color:     false,
			Envfile:   []string{},
		},
	},
	{
		args: []string{"render", "--option", "useenv.jsonnet", "--envfile=envfile.useenv"},
		sub:  "render",
		option: &lambroll.Option{
			OptionFilePath: "useenv.jsonnet",
			Function:       "hello",
			LogLevel:       "info",
			LogFormat:      "text",
			Color:          true,
			Envfile:        []string{"envfile.useenv"},
		},
	},
	{
		args: []string{"render", "--option", "useenv.jsonnet"},
		sub:  "render",
		env: map[string]string{
			"TEST_FUNCTION_NAME": "world",
		},
		option: &lambroll.Option{
			OptionFilePath: "useenv.jsonnet",
			Function:       "world",
			LogLevel:       "info",
			LogFormat:      "text",
			Color:          true,
			Envfile:        []string{},
		},
	},
	{
		args: []string{"render", "--option", "missing.jsonnet"},
		sub:  "render",
		err:  os.ErrNotExist,
	},
	{
		args: []string{"render", "--option", "missing.json"},
		sub:  "render",
		err:  os.ErrNotExist,
	},
	{
		args: []string{"render", "--option", "override.jsonnet",
			"--profile", "mine",
			"--region", "us-west-2",
		},
		env: map[string]string{
			"LAMBROLL_LOG_LEVEL": "debug",
		},
		sub: "render",
		option: &lambroll.Option{
			OptionFilePath: "override.jsonnet",
			Color:          true,
			Envfile:        []string{},
			LogLevel:       "trace", // option file's priority is higher than env
			LogFormat:      "text",
			Profile:        ptr("mine"),
			Region:         ptr("us-west-2"),
		},
	},
	{
		args: []string{"render",
			"--profile", "mine",
			"--region", "us-west-2",
		},
		env: map[string]string{
			"LAMBROLL_OPTION":    "override.jsonnet",
			"LAMBROLL_LOG_LEVEL": "debug",
		},
		sub: "render",
		option: &lambroll.Option{
			OptionFilePath: "override.jsonnet",
			Color:          true,
			Envfile:        []string{},
			LogLevel:       "trace", // option file's priority is higher than env
			LogFormat:      "text",
			Profile:        ptr("mine"),
			Region:         ptr("us-west-2"),
		},
	},
	{
		args: []string{"render", "--option", "ext_vars.jsonnet"},
		sub:  "render",
		option: &lambroll.Option{
			OptionFilePath: "ext_vars.jsonnet",
			LogLevel:       "info",
			LogFormat:      "text",
			Color:          true,
			Envfile:        []string{},
			ExtStr: map[string]string{
				"architecture": "x86_64",
				"description":  "Test function with ext_str and ext_code",
			},
			ExtCode: map[string]string{
				"memory_size":  "128",
				"storage_size": "512",
				"timeout":      "30",
			},
		},
	},
	{
		// Test: deprecated extstr/extcode keys are still accepted (v1 compat).
		args: []string{"render", "--option", "ext_vars_legacy.jsonnet"},
		sub:  "render",
		option: &lambroll.Option{
			OptionFilePath: "ext_vars_legacy.jsonnet",
			LogLevel:       "info",
			LogFormat:      "text",
			Color:          true,
			Envfile:        []string{},
			ExtStr: map[string]string{
				"architecture": "x86_64",
				"description":  "Test function with ext_str and ext_code",
			},
			ExtCode: map[string]string{
				"memory_size":  "128",
				"storage_size": "512",
				"timeout":      "30",
			},
		},
	},
	{
		// Test: CLI args are merged with option file, CLI takes precedence
		args: []string{
			"render", "--option", "ext_vars.jsonnet",
			"--ext-str", "cli_key=cli_val",
			"--ext-str", "architecture=arm64",
			"--ext-code", "memory_size=256",
			"--ext-code", "storage_size=1024",
		},
		sub: "render",
		option: &lambroll.Option{
			OptionFilePath: "ext_vars.jsonnet",
			LogLevel:       "info",
			LogFormat:      "text",
			Color:          true,
			Envfile:        []string{},
			ExtStr: map[string]string{
				"architecture": "arm64",                                   // CLI overrides option file (x86_64 -> arm64)
				"description":  "Test function with ext_str and ext_code", // from option file
				"cli_key":      "cli_val",                                 // from CLI
			},
			ExtCode: map[string]string{
				"memory_size":  "256",  // CLI overrides option file (128 -> 256)
				"storage_size": "1024", // CLI overrides option file (512 -> 1024)
				"timeout":      "30",   // from option file
			},
		},
	},
}

func TestParseCLISubcommandOption(t *testing.T) {
	cwd, _ := os.Getwd()
	os.Chdir("test/cli")
	defer os.Chdir(cwd)

	t.Run("subcommand flags from option file", func(t *testing.T) {
		sub, opt, _, err := lambroll.ParseCLI([]string{"diff", "--option", "subcommand.jsonnet"})
		if err != nil {
			t.Fatal(err)
		}
		if sub != "diff" {
			t.Errorf("unexpected subcommand: %s", sub)
		}
		if opt.Region == nil || *opt.Region != "us-west-2" {
			t.Errorf("global flag region not resolved from option file: %v", opt.Region)
		}
		if opt.Diff == nil {
			t.Fatal("diff option is nil")
		}
		if opt.Diff.External != "dyff between" {
			t.Errorf("diff.external: expected 'dyff between', got %q", opt.Diff.External)
		}
		if !opt.Diff.CodeSha256 {
			t.Errorf("diff.code: expected true, got false")
		}
	})

	t.Run("CLI flag overrides option file", func(t *testing.T) {
		_, opt, _, err := lambroll.ParseCLI([]string{"diff", "--option", "subcommand.jsonnet", "--external", "cli-cmd"})
		if err != nil {
			t.Fatal(err)
		}
		if opt.Diff.External != "cli-cmd" {
			t.Errorf("diff.external: CLI should override option file, got %q", opt.Diff.External)
		}
	})

	// Falsey values must survive loading: they are the only way to negate
	// default-true flags (e.g. color, deploy.publish) from an option file.
	t.Run("falsey values from option file", func(t *testing.T) {
		sub, opt, _, err := lambroll.ParseCLI([]string{"deploy", "--option", "falsey.jsonnet"})
		if err != nil {
			t.Fatal(err)
		}
		if sub != "deploy" {
			t.Errorf("unexpected subcommand: %s", sub)
		}
		if opt.Color {
			t.Error("global color: expected false from option file, got true")
		}
		if opt.Deploy == nil {
			t.Fatal("deploy option is nil")
		}
		if opt.Deploy.Publish {
			t.Error("deploy.publish: expected false from option file, got true")
		}
	})

	t.Run("diff.mask from option file", func(t *testing.T) {
		_, opt, _, err := lambroll.ParseCLI([]string{"diff", "--option", "diff_mask.jsonnet"})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"DB_PASSWORD", ".Environment.Variables[]"}
		if diff := cmp.Diff(want, opt.Diff.Mask); diff != "" {
			t.Errorf("diff.mask not resolved from option file: %s", diff)
		}
	})

	// A repeatable slice flag given on the CLI replaces the option-file value
	// (uniform with every other flag: option file = default, CLI overrides).
	t.Run("CLI --mask replaces option file diff.mask", func(t *testing.T) {
		_, opt, _, err := lambroll.ParseCLI([]string{"diff", "--option", "diff_mask.jsonnet", "--mask", "FOO"})
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff([]string{"FOO"}, opt.Diff.Mask); diff != "" {
			t.Errorf("CLI --mask should replace option file diff.mask: %s", diff)
		}
	})

	t.Run("render.mask from option file", func(t *testing.T) {
		_, opt, _, err := lambroll.ParseCLI([]string{"render", "--option", "render_mask.jsonnet"})
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff([]string{"DB_PASSWORD"}, opt.Render.Mask); diff != "" {
			t.Errorf("render.mask not resolved from option file: %s", diff)
		}
	})
}

func TestParseCLI(t *testing.T) {
	cwd, _ := os.Getwd()
	os.Chdir("test/cli")
	defer os.Chdir(cwd)

	for _, tt := range cliTests {
		t.Run(strings.Join(tt.args, "_"), func(t *testing.T) {
			for k, v := range tt.env {
				lambroll.Setenv(k, v)
			}
			defer lambroll.ResetEnv()
			sub, opt, _, err := lambroll.ParseCLI(tt.args)
			if err != nil {
				if tt.err == nil {
					t.Errorf("error is expected, but got nil. got %v", err)
				} else if errors.Is(err, tt.err) {
					// OK
				} else {
					// unexpected, but OK
					t.Logf("expected %v, got %v", tt.err, err)
				}
				return
			}
			if sub != tt.sub {
				t.Errorf("unexpected subcommand: expected %s, got %s", tt.sub, sub)
			}
			if tt.option != nil {
				if diff := cmp.Diff(*tt.option, opt.Option); diff != "" {
					t.Errorf("unexpected option: diff %s", diff)
				}
			}
		})
	}
}
