package lambroll

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/mattn/go-isatty"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	typesv2 "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// InvokeOption represents option for Invoke()
type InvokeOption struct {
	FunctionFilePath *string
	Async            *bool
	LogTail          *bool
	Qualifier        *string
}

// Invoke invokes function
func (app *App) Invoke(ctx context.Context, opt InvokeOption) error {
	fn, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return fmt.Errorf("failed to load function: %w", err)
	}
	var invocationType typesv2.InvocationType
	var logType typesv2.LogType
	if *opt.Async {
		invocationType = typesv2.InvocationTypeEvent
	} else {
		invocationType = typesv2.InvocationTypeRequestResponse
	}
	if *opt.LogTail {
		logType = typesv2.LogTypeTail
	}

	if isatty.IsTerminal(os.Stdin.Fd()) {
		fmt.Println("Enter JSON payloads for the invoking function into STDIN. (Type Ctrl-D to close.)")
	}

	dec := json.NewDecoder(os.Stdin)
	stdout := bufio.NewWriter(os.Stdout)
	stderr := bufio.NewWriter(os.Stderr)
PAYLOAD:
	for {
		var payload interface{}
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
		}
		if len(*opt.Qualifier) > 0 {
			in.Qualifier = opt.Qualifier
		}
		log.Println("[debug] invoking function", in)
		res, err := app.lambda.Invoke(ctx, in)
		if err != nil {
			log.Println("[error] failed to invoke function", err.Error())
			continue PAYLOAD
		}
		stdout.Write(res.Payload)
		stdout.Write([]byte("\n"))
		stdout.Flush()

		log.Printf("[info] StatusCode:%d", res.StatusCode)
		if res.ExecutedVersion != nil {
			log.Printf("[info] ExecutionVersion:%s", *res.ExecutedVersion)
		}
		if res.LogResult != nil {
			b, _ := base64.StdEncoding.DecodeString(*res.LogResult)
			stderr.Write(b)
			stderr.Flush()
		}
	}

	return nil
}
