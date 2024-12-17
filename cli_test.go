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
			LogLevel: "info",
			Color:    true,
			Envfile:  []string{},
		},
	},
	{
		args: []string{"render", "--config", "debug.jsonnet"},
		sub:  "render",
		option: &lambroll.Option{
			LogLevel:       "debug",
			Color:          true,
			ConfigFilePath: "debug.jsonnet",
			Envfile:        []string{},
		},
	},
	{
		args: []string{"render", "--envfile=envfile.global"},
		sub:  "render",
		option: &lambroll.Option{
			LogLevel: "info",
			Color:    true,
			Envfile:  []string{"envfile.global"},
		},
	},
	{
		args: []string{
			"--envfile=envfile.local",
			"--config", "global.jsonnet",
			"--function", "function.jsonnet",
			"render",
		},
		sub: "render",
		option: &lambroll.Option{
			Function:       "function.jsonnet",
			ConfigFilePath: "global.jsonnet",
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
			LogLevel: "info",
			Color:    true,
			Envfile:  []string{"envfile.global", "envfile.local"},
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
			LogLevel: "error",
			Color:    false,
			Envfile:  []string{},
		},
	},
	{
		args: []string{"render", "--config", "useenv.jsonnet", "--envfile=envfile.useenv"},
		sub:  "render",
		option: &lambroll.Option{
			ConfigFilePath: "useenv.jsonnet",
			Function:       "hello",
			Color:          true,
			Envfile:        []string{"envfile.useenv"},
		},
	},
	{
		args: []string{"render", "--config", "useenv.jsonnet"},
		sub:  "render",
		env: map[string]string{
			"TEST_FUNCTION_NAME": "world",
		},
		option: &lambroll.Option{
			ConfigFilePath: "useenv.jsonnet",
			Function:       "world",
			Color:          true,
			Envfile:        []string{},
		},
	},
	{
		args: []string{"render", "--config", "missing.jsonnet"},
		sub:  "render",
		err:  os.ErrNotExist,
	},
	{
		args: []string{"render", "--config", "missing.json"},
		sub:  "render",
		err:  os.ErrNotExist,
	},
}

func TestParseCLI(t *testing.T) {
	cwd, _ := os.Getwd()
	os.Chdir("test/cli")
	defer os.Chdir(cwd)

	for _, tt := range cliTests {
		t.Run(strings.Join(tt.args, "_"), func(t *testing.T) {
			for k, v := range tt.env {
				reset := setenv(k, v)
				defer reset()
			}
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

func setenv(key, value string) func() {
	orig, found := os.LookupEnv(key)
	os.Setenv(key, value)
	return func() {
		if found {
			os.Setenv(key, orig)
		} else {
			os.Unsetenv(key)
		}
	}
}
