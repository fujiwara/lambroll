package lambroll

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Songmu/prompter"
	"github.com/google/go-jsonnet/formatter"
)

func (app *App) saveFile(path string, b []byte, mode os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		ok := prompter.YN(fmt.Sprintf("Overwrite existing file %s?", path), false)
		if !ok {
			return nil
		}
	}
	return os.WriteFile(path, b, mode)
}

func marshalJSON(s interface{}) ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	x := make(map[string]interface{})
	if err := json.Unmarshal(b, &x); err != nil {
		return nil, err
	}
	if b, err := json.MarshalIndent(omitEmptyValues(x), "", "  "); err != nil {
		return nil, err
	} else {
		return append(b, '\n'), nil
	}
}

func unmarshalJSON(src []byte, v interface{}, path string) error {
	strict := json.NewDecoder(bytes.NewReader(src))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&v); err != nil {
		if !strings.Contains(err.Error(), "unknown field") {
			return err
		}
		log.Printf("[warn] %s in %s", err, path)

		// unknown field -> try lax decoder
		lax := json.NewDecoder(bytes.NewReader(src))
		return lax.Decode(&v)
	}
	return nil
}

func findDefinitionFile(preffered string, defaults []string) (string, error) {
	if preffered != "" {
		if _, err := os.Stat(preffered); err == nil {
			return preffered, nil
		} else {
			return "", err
		}
	}
	for _, name := range defaults {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("function file (%s) not found", strings.Join(DefaultFunctionFilenames, " or "))
}

func jsonToJsonnet(src []byte, filepath string) ([]byte, error) {
	s, err := formatter.Format(filepath, string(src), formatter.DefaultOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to format jsonnet: %w", err)
	}
	return []byte(s), nil
}

func resolveLogGroup(fn *Function) string {
	if fn.LoggingConfig != nil && fn.LoggingConfig.LogGroup != nil {
		return *fn.LoggingConfig.LogGroup
	}
	return fmt.Sprintf("/aws/lambda/%s", *fn.FunctionName)
}
