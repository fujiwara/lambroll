package lambroll

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

// jsonObjectKeys returns the set of JSON object keys defined by a struct type.
// Embedded structs without a json tag are flattened (their keys are promoted),
// matching how kong flattens embedded option structs into the same command.
func jsonObjectKeys(t reflect.Type) map[string]bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	keys := map[string]bool{}
	if t.Kind() != reflect.Struct {
		return keys
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if f.Anonymous && name == "" {
			for k := range jsonObjectKeys(f.Type) {
				keys[k] = true
			}
			continue
		}
		if name == "" || name == "-" {
			continue
		}
		keys[name] = true
	}
	return keys
}

// TestOptionFileSubcommandTags guards against drift between kong flag names and
// the json tags on each subcommand option struct, and between each command
// field's cmd name and its json tag. The option file resolver scopes lookups by
// the cmd name and resolves flags by the snake_case form of the flag name, so
// every flag must have a matching json tag and the json tag must equal the cmd
// name.
func TestOptionFileSubcommandTags(t *testing.T) {
	p, err := kong.New(&CLIOptions{})
	if err != nil {
		t.Fatalf("failed to build kong model: %v", err)
	}

	// flag name (snake_case) -> exists, per subcommand
	flagsByCommand := map[string]map[string]bool{}
	for _, node := range p.Model.Children {
		names := map[string]bool{}
		for _, f := range node.Flags {
			names[strings.ReplaceAll(f.Name, "-", "_")] = true
		}
		flagsByCommand[node.Name] = names
	}

	cliT := reflect.TypeFor[CLIOptions]()
	for i := 0; i < cliT.NumField(); i++ {
		field := cliT.Field(i)
		cmd, ok := field.Tag.Lookup("cmd")
		if !ok {
			continue // embedded Option etc.
		}
		// kong uses the field name lowercased when the cmd tag has no value.
		if cmd == "" {
			cmd = strings.ToLower(field.Name)
		}
		jsonTag, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if jsonTag == "-" {
			continue // commands without options (e.g. version)
		}
		if jsonTag != cmd {
			t.Errorf("CLIOptions.%s: json tag %q must equal cmd name %q", field.Name, jsonTag, cmd)
			continue
		}
		flags := flagsByCommand[cmd]
		jsonKeys := jsonObjectKeys(field.Type)
		for f := range flags {
			if !jsonKeys[f] {
				t.Errorf("command %q flag %q has no matching json tag in %s", cmd, f, field.Type)
			}
		}
		for k := range jsonKeys {
			if !flags[k] {
				t.Errorf("command %q json tag %q has no matching kong flag in %s", cmd, k, field.Type)
			}
		}
	}
}

// TestNewOptionFileResolver verifies that the resolver scopes lookups by
// subcommand name and falls back to the top level for global flags.
func TestNewOptionFileResolver(t *testing.T) {
	values := map[string]any{
		"region": "us-west-2",
		"diff": map[string]any{
			"external": "dyff between",
		},
	}
	resolver := newOptionFileResolver(values)

	globalFlag := &kong.Flag{Value: &kong.Value{Name: "region"}}
	if v, _ := resolver.Resolve(nil, &kong.Path{}, globalFlag); v != "us-west-2" {
		t.Errorf("global flag region: expected us-west-2, got %v", v)
	}

	diffFlag := &kong.Flag{Value: &kong.Value{Name: "external"}}
	diffPath := &kong.Path{Command: &kong.Command{Name: "diff"}}
	if v, _ := resolver.Resolve(nil, diffPath, diffFlag); v != "dyff between" {
		t.Errorf("diff flag external: expected 'dyff between', got %v", v)
	}

	// a subcommand flag absent from the file resolves to nil (use default/env)
	otherPath := &kong.Path{Command: &kong.Command{Name: "deploy"}}
	if v, _ := resolver.Resolve(nil, otherPath, diffFlag); v != nil {
		t.Errorf("deploy section absent: expected nil, got %v", v)
	}
}

// TestUnmarshalJSONStrict verifies that CLIOptions (a RequireStrictLoader)
// rejects unknown keys at both levels, while a non-strict type warns and
// continues.
func TestUnmarshalJSONStrict(t *testing.T) {
	t.Run("valid keys", func(t *testing.T) {
		var c CLIOptions
		if err := unmarshalJSON([]byte(`{"region":"us-west-2","diff":{"external":"dyff","code":true}}`), &c, "test"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("unknown top-level key", func(t *testing.T) {
		var c CLIOptions
		if err := unmarshalJSON([]byte(`{"regionn":"x"}`), &c, "test"); err == nil {
			t.Error("expected error for unknown top-level key")
		}
	})
	t.Run("unknown nested key", func(t *testing.T) {
		var c CLIOptions
		if err := unmarshalJSON([]byte(`{"diff":{"externall":"x"}}`), &c, "test"); err == nil {
			t.Error("expected error for unknown nested key")
		}
	})
	t.Run("deprecated extstr/extcode aliases accepted", func(t *testing.T) {
		var c CLIOptions
		if err := unmarshalJSON([]byte(`{"extstr":{"a":"b"},"extcode":{"c":"d"}}`), &c, "test"); err != nil {
			t.Errorf("deprecated extstr/extcode aliases should be accepted: %v", err)
		}
		c.applyLegacyExtVars()
		if c.ExtStr["a"] != "b" {
			t.Errorf("extstr should be folded into ExtStr: %v", c.ExtStr)
		}
		if c.ExtCode["c"] != "d" {
			t.Errorf("extcode should be folded into ExtCode: %v", c.ExtCode)
		}
	})
	t.Run("non-strict type warns and continues", func(t *testing.T) {
		type nonStrict struct {
			Known string `json:"known"`
		}
		var n nonStrict
		if err := unmarshalJSON([]byte(`{"known":"v","unknownnn":1}`), &n, "test"); err != nil {
			t.Errorf("non-strict type should not error on unknown key: %v", err)
		}
	})
}
