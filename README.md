# lambroll

lambroll is a simple deployment tool for [AWS Lambda](https://aws.amazon.com/lambda/).

lambroll does,

- Create a function.
- Create a Zip archive from a local directory.
- Deploy function code / configuration / tags / aliases / function URLs.
- Rollback a function to the previous version.
- Invoke a function with payloads.
- Manage function versions.
- Show status of a function.
- Show function logs.
- Show diff of function code / configuration.
- Delete a function.

lambroll does not,

- Manage resources related to the Lambda function.
  - For example, IAM Role, function triggers, API Gateway, and etc.
  - Only the function URLs can be managed by lambroll if you want.
- Build native binaries or extensions for Linux (AWS Lambda running environment).

When you hope to manage these resources, we recommend other deployment tools ([AWS SAM](https://aws.amazon.com/serverless/sam/), [Serverless Framework](https://serverless.com/), etc.).

## Differences of lambroll v0 and v1.

See [docs/v0-v1.md](docs/v0-v1.md).

## Install

### Homebrew (macOS and Linux)

```console
$ brew install fujiwara/tap/lambroll
```

### aqua

[aqua](https://aquaproj.github.io/) is a declarative CLI Version Manager.

```console
$ aqua g -i fujiwara/lambroll
```

### Binary packages

[Releases](https://github.com/fujiwara/lambroll/releases)

### CircleCI Orb

https://circleci.com/orbs/registry/orb/fujiwara/lambroll

```yml
version: 2.1
orbs:
  lambroll: fujiwara/lambroll@2.0.1
jobs:
  deploy:
    docker:
      - image: cimg/base
    steps:
      - checkout
      - lambroll/install:
          version: v1.1.0
      - run:
          command: |
            lambroll deploy
```

### GitHub Actions

Action fujiwara/lambroll@v1 installs lambroll binary for Linux into /usr/local/bin. This action runs install only.

```yml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: fujiwara/lambroll@v1
        with:
          version: v1.1.0
          # version-file: .lambroll-version
      - run: |
          lambroll deploy
```

Note:
- `version` is not required, but it is recommended that the version be specified.
  - The default version is not fixed and may change in the future.
- `version-file` can also be used to specify lambroll version by using the file that contains lambroll version (e.g. 1.1.0).
- `os` and `arch` are automatically detected. (Some previous versions use `os` and `arch` as inputs, but they are deprecated.)

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

Now you can deploy `hello` function using `lambroll deploy`.

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
Usage: lambroll <command>

Flags:
  -h, --help                              Show context-sensitive help.
      --function=STRING                   Function file path ($LAMBROLL_FUNCTION)
      --log-level="info"                  log level (trace, debug, info, warn, error) ($LAMBROLL_LOGLEVEL)
      --[no-]color                        enable colored output ($LAMBROLL_COLOR)
      --region=REGION                     AWS region ($AWS_REGION)
      --profile=PROFILE                   AWS credential profile name ($AWS_PROFILE)
      --tfstate=TFSTATE                   URL to terraform.tfstate ($LAMBROLL_TFSTATE)
      --prefixed-tfstate=KEY=VALUE;...    key value pair of the prefix for template function name and URL to
                                          terraform.tfstate ($LAMBROLL_PREFIXED_TFSTATE)
      --endpoint=ENDPOINT                 AWS API Lambda Endpoint ($AWS_LAMBDA_ENDPOINT)
      --envfile=ENVFILE,...               environment files ($LAMBROLL_ENVFILE)
      --ext-str=KEY=VALUE;...             external string values for Jsonnet ($LAMBROLL_EXTSTR)
      --ext-code=KEY=VALUE;...            external code values for Jsonnet ($LAMBROLL_EXTCODE)

Commands:
  deploy
    deploy or create function

  init --function-name=
    init function.json

  list
    list functions

  rollback
    rollback function

  invoke
    invoke function

  archive
    archive function

  logs
    show logs of function

  diff
    show diff of function

  render
    render function.json

  status
    show status of function

  delete
    delete function

  versions
    show versions of function

  version
    show version

Run "lambroll <command> --help" for more information on a command.
```

### Init

`lambroll init` initialize function.json by existing function.

```console
Usage: lambroll init --function-name=

init function.json

Flags:
      --function-name=                    Function name for init
      --download                          Download function.zip
      --jsonnet                           render function.json as jsonnet
      --qualifier=QUALIFIER               function version or alias
      --function-url                      create function url definition file
```

`init` creates `function.json` as a configuration file of the function.

### Deploy

```console
Usage: lambroll deploy

deploy or create function

Flags:
      --src="."                           function zip archive or src dir
      --publish                           publish function
      --alias="current"                   alias name for publish
      --alias-to-latest                   set alias to unpublished $LATEST version
      --dry-run                           dry run
      --skip-archive                      skip to create zip archive. requires Code.S3Bucket and Code.S3Key in function definition
      --keep-versions=0                   Number of latest versions to keep. Older versions will be deleted. (Optional value: default 0).
      --ignore=""                         ignore fields by jq queries in function.json
      --function-url=""                   path to function-url definition ($LAMBROLL_FUNCTION_URL)
      --skip-function                     skip to deploy a function. deploy function-url only
      --exclude-file=".lambdaignore"      exclude file
      --symlink                           keep symlink (same as zip --symlink,-y)
```

`deploy` works as below.

- Create a zip archive from `--src` directory.
  - Excludes files matched (wildcard pattern) in `--exclude-file`.
- Create / Update Lambda function
- Create an alias to the published version when `--publish` (default).


#### Deploy via S3

When the zip archive is too large to upload directly, you can deploy via S3.

Set `Code.S3Bucket` and `Code.S3Key` in function.json. lambroll uploads the zip archive to the specified S3 bucket and deploys the function.

```json
{
  "Code": {
    "S3Bucket": "my-bucket",
    "S3Key": "function.zip"
  }
}
```

If you want to upload the zip archive yourself, you can skip creating the zip archive by using the `--skip-archive` flag.

#### Deploy container image

lambroll also support to deploy a container image for Lambda.

`PackageType=Image` and `Code.ImageUri` are required in function.json.
`ImageConfig` is optional.

```json
{
  "FunctionName": "container",
  "MemorySize": 128,
  "Role": "arn:aws:iam::012345678912:role/test_lambda_function",
  "PackageType": "Image",
  "Code": {
    "ImageUri": "012345678912.dkr.ecr.ap-northeast-1.amazonaws.com/lambda/test:latest"
  },
  "ImageConfig": {
    "Command": [
      "/path/to/bootstrap"
    ],
    "WorkingDirectory": "/var/task",
    "EntryPoint": [
      "/path/to/entrypoint"
    ],
  }
}
```

### Rollback

```
Usage: lambroll rollback

rollback function

Flags:
      --dry-run                   dry run
      --alias="current"           alias to rollback
      --version=""                version to rollback (default: previous version auto detected)
      --delete-version            delete rolled back version
```

`lambroll deploy` create/update alias to the published function version on deploy.

`lambroll rollback` works as below.

1. Find the previous version from the alias with no other aliases.
2. Update the alias to the previous version.
   - If `--version` is specified, update the alias to the specified version.
3. When `--delete-version` is specified, delete the old version of the function.

If you add multiple aliases to the function, `lambroll rollback --alias={some-alias}` may not work as expected. Because the previous version that auto-detected may be the older version of other aliases.

So you should specify the version to rollback with `--version` flag to clear the ambiguity.

### Invoke

```
Usage: lambroll invoke

invoke function

Flags:
      --async                             invocation type async
      --log-tail                          output tail of log to STDERR
      --qualifier=QUALIFIER               version or alias to invoke
      --payload=PAYLOAD                   payload to invoke. if not specified, read from STDIN
```

`lambroll invoke` accepts multiple JSON payloads for invocations from `--payload` flag or STDIN.

If the payload is a concatenation of multiple JSON payloads, `lambroll invoke` will invoke the function for each JSON payload.

Outputs from the function invoked are printed to STDOUT.

```console
$ lambroll invoke --payload='{"foo":1} --log-tail'
{"success": true, "payload": {"foo":1}}
2019/10/28 23:16:43 [info] StatusCode:200 ExecutionVersion:$LATEST
START RequestId: aa38233f-a179-4192-8469-c86414fe463c Version: $LATEST
END RequestId: aa38233f-a179-4192-8469-c86414fe463c
REPORT RequestId: 60140e16-018e-41b1-bb46-3f021d4960c0	Duration: 561.77 ms	Billed Duration: 600 ms	Memory Size: 128 MB	Max Memory Used: 50 MB

$ echo '{"foo":1}{"foo":2}' | lambroll invoke --log-tail
{"success": true, payload{"foo":1}}
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
  "Architectures": [
    "arm64"
  ],
  "Description": "hello function for {{ must_env `ENV` }}",
  "EphemeralStorage": {
    "Size": 1024
  },
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
  "Runtime": "nodejs18.x",
  "Tags": {
    "Env": "dev"
  },
  "Timeout": 5,
  "TracingConfig": {
    "Mode": "PassThrough"
  }
}
```

The template functions is available in `{{ }}`.
- `env` function expands environment variables.
- `must_env` function expands environment variables. If the environment variable is not defined, lambroll will panic and abort.

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

#### Environment variables from envfile

`lambroll --envfile .env1 .env2` reads files named .env1 and .env2 as environment files and export variables in these files.

These files are parsed by [hashicorp/go-envparse](https://github.com/hashicorp/go-envparse).

```env
FOO=foo
export BAR="bar"
```

#### Jsonnet support for function configuration

lambroll also can read function.jsonnet as [Jsonnet](https://jsonnet.org/) format instead of plain JSON.

```jsonnet
{
  FunctionName: 'hello',
  Handler: 'index.handler',
  MemorySize: std.extVar('memorySize'),
  Role: 'arn:aws:iam::%s:role/lambda_role' % [ std.extVar('accountID') ],
  Runtime: 'nodejs20.x',
}
```

```console
$ lambroll \
    --function function.jsonnet \
    --ext-str accountID=0123456789012 \
    --ext-code memorySize="128 * 4" \
    deploy
```

- `--ext-str` sets external string values for Jsonnet.
- `--ext-code` sets external code values for Jsonnet.

v1.1.0 and later, lambroll supports Jsonnet native functions. See below for details.

#### Expand SSM parameter values

At reading the file, lambroll evaluates `{{ ssm }}` syntax in JSON.

For example,

```
{{ ssm `/path/to/param` }}
```

SSM parameter value of `/path/to/param` is expanded here.

For Jsonnet, the `ssm` function is available.

```jsonnet
local ssm = std.native('ssm');
{
  Environment: {
    Variables: {
      FOO: ssm('/path/to/param'),
    },
  },
}
```

#### Expand environment variables

At reading the file, lambroll evaluates `{{ env }}` and `{{ must_env }}` syntax in JSON.

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

For Jsonnet, the `env` and `must_env` native functions are available.

```jsonnet
local env = std.native('env');
local must_env = std.native('must_env');
{
  Environment: {
    Variables: {
      FOO: env('FOO', 'default for FOO'),
      BAR: must_env('BAR'),
    },
  },
}
```

#### Resolve AWS caller identity

The `caller_identity` template function resolves the AWS caller identity.

```json
{
  "Account": "{{ caller_identity.Account }}",
  "Arn": "{{ caller_identity.Arn }}",
  "UserId": "{{ caller_identity.UserId }}"
}
```

The `caller_identity` native function also available in Jsonnet.

```jsonnet
local caller = std.native('caller_identity')();
{
  Account: caller.Account,
  Arn: caller.Arn,
  UserId: caller.UserId,
}
```

The `caller_identity` function returns an object containing the following fields: `Account`, `Arn`, and `UserId`.

This object is the same as the result of [GetCallerIdentity](https://docs.aws.amazon.com/STS/latest/APIReference/API_GetCallerIdentity.html) API.

#### Lookup resource attributes in tfstate ([Terraform state](https://www.terraform.io/docs/state/index.html))

When `--tfstate` option set to an URL to `terraform.tfstate`, tfstate template function enabled.

For example, define your AWS resources by terraform.

```terraform
data "aws_iam_role" "lambda" {
  name = "hello_lambda_function"
}
```

`terraform apply` creates a terraform.tfstate file.

`lambroll --tfstate URL ...` enables to lookup resource attributes in the tfstate URL.

```json
{
  "Description": "hello function",
  "FunctionName": "hello",
  "Handler": "index.js",
  "MemorySize": 128,
  "Role": "{{ tfstate `data.aws_iam_role.lambda.arn` }}",
  "Runtime": "nodejs20.x",
  "Timeout": 5,
  "TracingConfig": {
    "Mode": "PassThrough"
  },
  "VpcConfig": {
    "SubnetIds": [
      "{{ tfstate `aws_subnet.lambda['az-a'].id` }}",
      "{{ tfstate `aws_subnet.lambda['az-b'].id` }}"
    ],
    "SecurityGroupIds": [
      "{{ tfstatef `aws_security_group.internal['%s'].id` (must_env `WORLD`) }}"
    ]
  }
}
```

For Jsonnet, the `tfstate` native function is available.

```jsonnet
local tfstate = std.native('tfstate');
{
  Description: 'hello function',
  FunctionName: 'hello',
  Handler: 'index.js',
  MemorySize: 128,
  Role: tfstate('data.aws_iam_role.lambda.arn'),
  Runtime: 'nodejs20.x',
  Timeout: 5,
  TracingConfig: {
    Mode: 'PassThrough',
  },
  VpcConfig: {
    SubnetIds: [
      tfstate('aws_subnet.lambda["az-a"].id'),
      tfstate('aws_subnet.lambda["az-b"].id'),
    ],
    SecurityGroupIds: [
      tfstate('aws_security_group.internal["%s"].id' % must_env('WORLD')),
    ],
  },
}
```

Likewise, if you have AWS resource definitions spread across multiple tfstate files, you can utilize `--prefixed-tfstate` option:

e.g.
```shell
lambroll --prefixed-tfstate="my_first_=s3://my-bucket/first.tfstate" --prefixed-tfstate="my_second_=s3://my-bucket/second.tfstate" ...
```

which then exposes additional template functions available like:

```json
{
  "Description": "hello function",
  "Environment": {
    "Variables": {
      "FIRST_VALUE": "{{ my_first_tfstate `data.aws_iam_role.lambda.arn` }}",
      "SECOND_VALUE": "{{ my_second_tfstate `data.aws_iam_role.lambda.arn` }}"
    }
  },
  "rest of the parameters": "..."
}
```

For Jsonnet, a `{prefix}_tfstate` native function is generated by the `--prefixed-tfstate` option.

```jsonnet
local first_tfstate = std.native('my_first_tfstate');
local second_tfstate = std.native('my_second_tfstate');
{
  Description: 'hello function',
  Environment: {
    Variables: {
      FIRST_VALUE: first_tfstate('data.aws_iam_role.lambda.arn'),
      SECOND_VALUE: second_tfstate('data.aws_iam_role.lambda.arn'),
    },
  },
  "rest of the parameters": "...",
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

### Lambda@Edge support

lambroll can deploy [Lambda@Edge](https://aws.amazon.com/lambda/edge/) functions.

Edge functions require two preconditions:

- `--region` must set to `us-east-1`.
- The IAM Role must be assumed by `lambda.amazonaws.com` and `edgelambda.amazonaws.com` both.

Otherwise, it works as usual.

### Lambda function URLs support

lambroll can deploy [Lambda function URLs](https://docs.aws.amazon.com/lambda/latest/dg/lambda-urls.html).

`lambroll deploy --function-url=function_url.json` deploys a function URL after the function deploied.

Even if your Lambda function already has a function URL, `lambroll deploy` without `--function-url` option does not touch the function URLs resources.

When you want to deploy a public (without authentication) function URL, `function_url.json` is shown below.

```json
{
  "Config": {
    "AuthType": "NONE"
  }
}
```

When you want to deploy a private (requires AWS IAM authentication) function URL, `function_url.json` is shown below.

```json
{
  "Config": {
    "AuthType": "AWS_IAM",
    "Cors": {
      "AllowOrigins": [
        "*"
      ],
      "AllowMethods": [
        "GET",
        "POST"
      ]
    },
  },
  "Permissions": [
    {
      "Principal": "0123456789012"
    },
    {
      "PrincipalOrgID": "o-123456789",
      "Principal": "*"
    }
  ]
}
```

- `Config` maps to [CreateFunctionUrlConfigInput](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/lambda#CreateFunctionUrlConfigInput) in AWS SDK Go v2.
  - `Config.AuthType` must be `AWS_IAM` or `NONE`.
  - `Config.Qualifier` is optional. Default is `$LATEST`.
- `Permissions` is optional.
  - If `Permissions` is not defined and `AuthType` is `NONE`, `Principal` is set to `*` automatically.
  - When `AuthType` is `AWS_IAM`, you must define `Permissions` to specify allowed principals.
  - Each elements of `Permissions` maps to [AddPermissionInput](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/lambda#AddPermissionInput) in AWS SDK Go v2.
- `function_url.jsonnet` is also supported like `function.jsonnet`.

#### CloudFront origin access control (OAC) support

CloudFront provides origin access control (OAC) for restricting access to a Lambda function URL origin.

When you want to restrict access to a Lambda function URL origin by CloudFront, you can specify `Principal` as `cloudfront.amazonaws.com` and `SourceArn` as the ARN of the CloudFront distribution.

See also [Restricting access to an AWS Lambda function URL origin](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/private-content-restricting-access-to-lambda.html).

```json
{
  "Config": {
    "AuthType": "AWS_IAM",
  },
  "Permissions": [
    {
      "Principal": "cloudfront.amazonaws.com",
      "SourceArn": "arn:aws:cloudfront::123456789012:distribution/EXXXXXXXX"
    }
  ]
}
```

If you need to allow access from any CloudFront distributions in your account, you can specify `SourceArn` as `arn:aws:cloudfront::123456789012:distribution/*`.

Specifying `SourceArn` as `*` is not recommended because it allows access from any CloudFront distribution in any AWS account.

## LICENSE

MIT License

Copyright (c) 2019 FUJIWARA Shunichiro
