package lambroll

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
)

// InvokeOption represents option for Invoke()
type InvokeOption struct {
	FunctionFilePath *string
	Async            *bool
	LogTail          *bool
	Qualifier        *string
}

// Invoke invokes function
func (app *App) Invoke(opt InvokeOption) error {
	fn, err := app.loadFunction(*opt.FunctionFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to load function")
	}
	var invocationType, logType *string
	if *opt.Async {
		invocationType = aws.String("Event")
	} else {
		invocationType = aws.String("RequestResponse")
	}
	if *opt.LogTail {
		logType = aws.String("Tail")
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
			return errors.Wrap(err, "failed to decode payload as JSON")
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
		log.Println("[debug] invoking function", in.String())
		res, err := app.lambda.Invoke(in)
		if err != nil {
			log.Println("[error] failed to invoke function", err.Error())
			continue PAYLOAD
		}
		stdout.Write(res.Payload)
		stdout.Write([]byte("\n"))
		stdout.Flush()

		log.Printf("[info] StatusCode:%d", *res.StatusCode)
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
