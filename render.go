package lambroll

import (
	"context"
	"fmt"
	"os"
)

type RenderOption struct {
	Jsonnet bool `default:"false" help:"render function.json as jsonnet"`
}

// Invoke invokes function
func (app *App) Render(ctx context.Context, opt *RenderOption) error {
	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}
	b, err := marshalJSON(fn)
	if err != nil {
		return fmt.Errorf("failed to marshal function: %w", err)
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
