# lambroll

lambroll is a minimal deployment tool for [AWS Lambda](https://aws.amazon.com/lambda/).

lambroll does,

- Create a function.
- Create a Zip archive from local directory.
- Update function code / configuration / tags / aliases.

That's all.

lambroll does not,

- Manage resources related to the Lambda function.
  - e.g. IAM Role, function triggers, API Gateway, etc.
- Build native binaries or extensions for Linux (AWS Lambda running environment).

When you hope to manage these resources, we recommend other deployment tools ([AWS SAM](https://aws.amazon.com/serverless/sam/), [Serverless Framework](https://serverless.com/), etc.).

## Install

### Homebrew (macOS only)

```console
$ brew install fujiwara/tap/lambroll
```

### Binary packages

[Releases](https://github.com/fujiwara/lambroll/releases)

### CircleCI Orb

https://circleci.com/orbs/registry/orb/fujiwara/lambroll

```yml
version: 2.1
orbs:
  ecspresso: fujiwara/lambroll@0.0.7
  jobs:
    steps:
      - checkout
      - lambroll/install:
          version: v0.9.1
      - run:
          command: |
            lambroll deploy
```

### GitHub Actions

Action fujiwara/lambroll@v0 installs lambroll binary for Linux into /usr/local/bin. This action runs install only.

```yml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: fujiwara/lambroll@v0
        with:
          version: v0.9.1
      - run: |
          lambroll deploy
```

## Quick start

Try migrate your existing Lambda function `hello`.

```console
$ mkdir hello
$ cd hello
$ lambroll init --function-name hello --download
2019/10/26 01:19:23 [info] function hello found
2019/10/26 01:19:23 [info] downloading function.zip
2019/10/26 01:19:23 [info] creating function.json
2019/10/26 01:19:23 [info] completed

$ unzip -l function.zip
Archive:  function.zip
  Length      Date    Time    Name
---------  ---------- -----   ----
      408  10-26-2019 00:30   index.js
---------                     -------
      408                     1 file

$ unzip function.zip
Archive:  function.zip
 extracting: index.js

$ rm function.zip
```

See or edit `function.json` or `index.js`.

Now you can deploy `hello` fuction using `lambroll deploy`.

```console
$ lambroll deploy
2019/10/26 01:24:52 [info] starting deploy function hello
2019/10/26 01:24:53 [info] creating zip archive from .
2019/10/26 01:24:53 [info] zip archive wrote 1042 bytes
2019/10/26 01:24:53 [info] updating function configuration
2019/10/26 01:24:53 [info] updating function code hello
2019/10/26 01:24:53 [info] completed
```

## Usage

```console
usage: lambroll [<flags>] <command> [<args> ...]

Flags:
  --help                      Show context-sensitive help (also try --help-long and --help-man).
  --region="ap-northeast-1"   AWS region
  --log-level=info            log level (trace, debug, info, warn, error)
  --function="function.json"  Function file path
  --tfstate=""                path to terraform.tfstate
  --endpoint=""               AWS API Lambda Endpoint

Commands:
  help [<command>...]
    Show help.

  version
    show version

  init --function-name=FUNCTION-NAME [<flags>]
    init function.json

  list
    list functions

  deploy [<flags>]
    deploy or create function

  rollback [<flags>]
    rollback function

  delete [<flags>]
    delete function

  invoke [<flags>]
    invoke function

  archive [<flags>]
    archive zip

  logs [<flags>]
    tail logs using `aws logs tail` (aws-cli v2 required)

  diff
    show display diff of function.json compared with latest function
```

### Init

`lambroll init` initialize function.json by existing function.

```console
usage: lambroll init --function-name=FUNCTION-NAME [<flags>]

init function.json

Flags:
  --function-name=FUNCTION-NAME  Function name for initialize
  --download                     Download function.zip
```

`init` creates `function.json` as a configuration file of the function.

### Deploy

```console
usage: lambroll deploy [<flags>]

deploy or create function

Flags:
  --function="function.json"  Function file path
  --profile="$AWS_PROFILE"    AWS credential profile name
  --region="$AWS_REGION"      AWS region
  --tfstate=""                path to terraform.tfstate
  --endpoint=""               AWS API Lambda Endpoint
  --src="."                   function zip archive or src dir
  --exclude-file=".lambdaignore"
                              exclude file
  --dry-run                   dry run
  --publish                   publish function
  --alias="current"           alias name for publish
  --skip-archive              skip to create zip archive. requires Code.S3Bucket and Code.S3Key in function definition
```

`deploy` works as below.

- Create a zip archive from `--src` directory.
  - Excludes files matched (wildcard pattern) in `--exclude-file`.
- Create / Update Lambda function
- Create an alias to the published version when `--publish` (default).

### Rollback

```
usage: lambroll rollback [<flags>]

rollback function

Flags:
  --help                      Show context-sensitive help (also try --help-long and --help-man).
  --region="ap-northeast-1"   AWS region
  --log-level=info            log level (trace, debug, info, warn, error)
  --function="function.json"  Function file path
  --delete-version            Delete rolled back version
  --dry-run                   dry run
```

`lambroll deploy` create/update alias `current` to the published function version on deploy.

`lambroll rollback` works as below.

1. Find previous one version of function.
2. Update alias `current` to the previous version.
3. When `--delete-version` specified, delete old version of function.

### Invoke

```
usage: lambroll invoke [<flags>]

invoke function

Flags:
  --help                      Show context-sensitive help (also try --help-long and --help-man).
  --region="ap-northeast-1"   AWS region
  --log-level=info            log level (trace, debug, info, warn, error)
  --function="function.json"  Function file path
  --async                     invocation type async
  --log-tail                  output tail of log to STDERR
  --qualifier=QUALIFIER       version or alias to invoke
```

`lambroll invoke` accepts multiple JSON payloads for invocations from STDIN.

Outputs from function are printed in STDOUT.

```console
$ echo '{"foo":1}{"foo":2}' | lambroll invoke --log-tail
{"success": true, payload{"foo:1}}
2019/10/28 23:16:43 [info] StatusCode:200 ExecutionVersion:$LATEST
START RequestId: 60140e16-018e-41b1-bb46-3f021d4960c0 Version: $LATEST
END RequestId: 60140e16-018e-41b1-bb46-3f021d4960c0
REPORT RequestId: 60140e16-018e-41b1-bb46-3f021d4960c0	Duration: 561.77 ms	Billed Duration: 600 ms	Memory Size: 128 MB	Max Memory Used: 50 MB
{"success": true, payload:{"foo":2}}
2019/10/28 23:16:43 [info] StatusCode:200 ExecutionVersion:$LATEST
START RequestId: dcc584f5-ceaf-4109-b405-8e59ca7ae92f Version: $LATEST
END RequestId: dcc584f5-ceaf-4109-b405-8e59ca7ae92f
REPORT RequestId: dcc584f5-ceaf-4109-b405-8e59ca7ae92f	Duration: 597.87 ms	Billed Duration: 600 ms	Memory Size: 128 MB	Max Memory Used: 50 MB
2019/10/28 23:16:43 [info] completed
```

### function.json

function.json is a definition for Lambda function. JSON structure is based from [`CreateFunction` for Lambda API](https://docs.aws.amazon.com/lambda/latest/dg/API_CreateFunction.html).

```json
{
  "Description": "hello function for {{ must_env `ENV` }}",
  "Environment": {
    "Variables": {
      "BAR": "baz",
      "FOO": "{{ env `FOO` `default for FOO` }}"
    }
  },
  "FunctionName": "{{ must_env `ENV` }}-hello",
  "FileSystemConfigs": [
    {
      "Arn": "arn:aws:elasticfilesystem:ap-northeast-1:123456789012:access-point/fsap-04fc0858274e7dd9a",
      "LocalMountPath": "/mnt/lambda"
    }
  ],
  "Handler": "index.js",
  "MemorySize": 128,
  "Role": "arn:aws:iam::123456789012:role/hello_lambda_function",
  "Runtime": "nodejs10.x",
  "Tags": {
    "Env": "dev"
  },
  "Timeout": 5,
  "TracingConfig": {
    "Mode": "PassThrough"
  }
}
```
#### Tags

When "Tags" key exists in function.json, lambroll set / remove tags to the lambda function at deploy.

```json5
{
  // ...
  "Tags": {
    "Env": "dev",
    "Foo": "Bar"
  }
}
```

When "Tags" key does not exist, lambroll doesn't manage tags.
If you hope to remove all tags, set `"Tags": {}` expressly.

#### Expand enviroment variables

At reading the file, lambrol evaluates `{{ env }}` and `{{ must_env }}` syntax in JSON.

For example,

```
{{ env `FOO` `default for FOO` }}
```

Environment variable `FOO` is expanded here. When `FOO` is not defined, use default value.

```
{{ must_env `FOO` }}
```

Environment variable `FOO` is expanded. When `FOO` is not defined, lambroll will panic and abort.

`json_escape` template function escapes JSON meta characters in string values. This is useful for inject structured values into environment variables.

```json
{
    "Environment": {
        "Variables": {
            "JSON": "{{ env `JSON` | json_escape }}"
        }
    }
}
```

#### Lookup resource attributes in tfstate ([Terraform state](https://www.terraform.io/docs/state/index.html))

When `--tfstate` option set to path to `terraform.tfstate`, tfstate template function enabled.

For example, define your AWS resources by terraform.

```terraform
data "aws_iam_role" "lambda" {
  name = "hello_lambda_function"
}
```

`terraform apply` creates a terraform.tfstate file.

`lambroll --tfstate terraform.tfstate ...` enables to lookup resource attributes in the tfstate.

```json
{
  "Description": "hello function",
  "FunctionName": "hello",
  "Handler": "index.js",
  "MemorySize": 128,
  "Role": "{{ tfstate `data.aws_iam_role.lambda.arn` }}",
  "Runtime": "nodejs12.x",
  "Timeout": 5,
  "TracingConfig": {
    "Mode": "PassThrough"
  }
}
```

### .lambdaignore

lambroll will ignore files defined in `.lambdaignore` file at creating a zip archive.

For example,

```
# comment

*.zip
*~
```

For each line in `.lambdaignore` are evaluated as Go's [`path/filepath#Match`](https://godoc.org/path/filepath#Match).

## LICENSE

MIT License

Copyright (c) 2019 FUJIWARA Shunichiro
