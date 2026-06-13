package lambroll

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type RenderOption struct {
	Jsonnet     bool     `default:"false" help:"render function.json as jsonnet" json:"jsonnet,omitempty"`
	FunctionURL string   `help:"render function-url definition file" default:"" env:"LAMBROLL_FUNCTION_URL" json:"function_url,omitempty"`
	Mask        []string `help:"mask values in rendered output by jq selector or environment variable name (repeatable)" json:"mask,omitempty"`
}

// Invoke invokes function
func (app *App) Render(ctx context.Context, opt *RenderOption) error {
	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}
	maskSelectors := resolveMaskSelectors(opt.Mask)
	var b []byte
	if opt.FunctionURL != "" {
		fu, err := app.loadFunctionUrl(opt.FunctionURL, *fn.FunctionName)
		if err != nil {
			return fmt.Errorf("failed to load function-url: %w", err)
		}
		b, err = renderDefinition(fu, maskSelectors)
		if err != nil {
			return fmt.Errorf("failed to marshal function-url: %w", err)
		}
	} else {
		b, err = renderDefinition(fn, maskSelectors)
		if err != nil {
			return fmt.Errorf("failed to marshal function: %w", err)
		}
	}

	if opt.Jsonnet {
		b, err = jsonToJsonnet(b, app.functionFilePath)
		if err != nil {
			return fmt.Errorf("failed to render function.json as jsonnet: %w", err)
		}
	}
	if _, err := os.Stdout.Write(b); err != nil {
		return fmt.Errorf("failed to write function.json: %w", err)
	}
	return nil
}

// renderDefinition marshals a definition to indented JSON. When mask selectors
// are given, matched values are replaced with tokens first. With no selectors
// the output is byte-identical to marshalJSON.
func renderDefinition(s any, maskSelectors []string) ([]byte, error) {
	if len(maskSelectors) == 0 {
		return marshalJSON(s)
	}
	v, err := marshalAny(s)
	if err != nil {
		return nil, err
	}
	masked, err := maskValue(v, maskSelectors)
	if err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(masked, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
