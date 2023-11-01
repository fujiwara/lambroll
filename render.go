package lambroll

import (
	"context"
	"fmt"
	"os"
)

type RenderOption struct {
	Jsonnet     bool   `default:"false" help:"render function.json as jsonnet"`
	FunctionURL string `help:"render function-url definiton file" default:""`
}

// Invoke invokes function
func (app *App) Render(ctx context.Context, opt *RenderOption) error {
	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}
	var b []byte
	if opt.FunctionURL != "" {
		fu, err := app.loadFunctionUrl(opt.FunctionURL, *fn.FunctionName)
		if err != nil {
			return fmt.Errorf("failed to load function-url: %w", err)
		}
		b, err = marshalJSON(fu)
		if err != nil {
			return fmt.Errorf("failed to marshal function-url: %w", err)
		}
	} else {
		b, err = marshalJSON(fn)
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
