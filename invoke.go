package lambroll

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/mattn/go-isatty"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// InvokeOption represents option for Invoke()
type InvokeOption struct {
	Async     bool    `default:"false" help:"invocation type async"`
	LogTail   bool    `default:"false" help:"output tail of log to STDERR"`
	Qualifier *string `help:"version or alias to invoke"`
	Payload   *string `help:"payload to invoke. if not specified, read from STDIN"`
	TenantID  *string `help:"tenant ID for multi-tenant invocation"`
}

// Invoke invokes function
func (app *App) Invoke(ctx context.Context, opt *InvokeOption) error {
	fn, err := app.loadFunction(app.functionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}
	var invocationType types.InvocationType
	var logType types.LogType
	if opt.Async {
		invocationType = types.InvocationTypeEvent
	} else {
		invocationType = types.InvocationTypeRequestResponse
	}
	if opt.LogTail {
		logType = types.LogTypeTail
	}

	var payloadSrc io.Reader
	if opt.Payload != nil {
		payloadSrc = strings.NewReader(*opt.Payload)
	} else {
		if isatty.IsTerminal(os.Stdin.Fd()) {
			fmt.Println("Enter JSON payloads for the invoking function into STDIN. (Type Ctrl-D to close.)")
		}
		payloadSrc = os.Stdin
	}
	dec := json.NewDecoder(payloadSrc)
	stdout := bufio.NewWriter(os.Stdout)
	stderr := bufio.NewWriter(os.Stderr)
PAYLOAD:
	for {
		var payload any
		err := dec.Decode(&payload)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode payload as JSON: %w", err)
		}
		b, _ := json.Marshal(payload)
		in := &lambda.InvokeInput{
			FunctionName:   fn.FunctionName,
			InvocationType: invocationType,
			LogType:        logType,
			Payload:        b,
			TenantId:       opt.TenantID,
		}
		in.Qualifier = opt.Qualifier
		slog.Debug("invoking function", "input", in)
		res, err := app.lambda.Invoke(ctx, in)
		if err != nil {
			slog.Error("failed to invoke function", "error", err)
			continue PAYLOAD
		}
		stdout.Write(res.Payload)
		stdout.Write([]byte("\n"))
		stdout.Flush()

		slog.Info("invoke result", "statusCode", res.StatusCode)
		if res.ExecutedVersion != nil {
			slog.Info("execution version", "version", *res.ExecutedVersion)
		}
		if res.LogResult != nil {
			b, _ := base64.StdEncoding.DecodeString(*res.LogResult)
			stderr.Write(b)
			stderr.Flush()
		}
	}

	return nil
}
