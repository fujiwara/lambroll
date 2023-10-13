package lambroll

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-jsonnet/formatter"
)

type RenderOption struct {
	Jsonnet bool `default:"false" help:"render function.json as jsonnet"`
}

// Invoke invokes function
func (app *App) Render(ctx context.Context, opt RenderOption) error {
	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}
	b, err := marshalJSON(fn)
	if err != nil {
		return fmt.Errorf("failed to marshal function: %w", err)
	}
	if opt.Jsonnet {
		s, err := formatter.Format(app.functionFilePath, string(b), formatter.DefaultOptions())
		if err != nil {
			return fmt.Errorf("failed to format jsonnet: %w", err)
		}
		if _, err := os.Stdout.WriteString(s); err != nil {
			return fmt.Errorf("failed to write function: %w", err)
		}
	} else {
		if _, err := os.Stdout.Write(b); err != nil {
			return fmt.Errorf("failed to write function: %w", err)
		}
	}
	return nil
}
