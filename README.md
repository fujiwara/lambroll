# lambroll

lambroll is a minimal deployment tool for [AWS Lambda](https://aws.amazon.com/lambda/).

lambroll does only,

- Create a function
- Update function code / configuration

lambrole does not,

- Manage resources related to the Lambda function
  - e.g. IAM Role, Triggers, API Gateway, etc.

When you hope to manage these resources, we recommend other deployment tools ([AWS SAM](https://aws.amazon.com/serverless/sam/), [Serverless Framework](https://serverless.com/), etc.).

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
  --help                     Show context-sensitive help (also try --help-long and --help-man).
  --region="ap-northeast-1"  AWS region
  --log-level="info"         log level (debug, info, warn, error)

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
    deploy function
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

Deploy or create function.

Flags:
  --function="function.json"  Function file path
  --src="."                   function zip archive src dir
  --exclude-file=".lambdaignore"
                              exclude file
```

`deplpoy` works as below.

- Create a zip archive from `--src` directory.
  - Excludes files matched (wildcard pattern) in `--exclude-file`.
- Create / Update Lambda function.

#### .lambdaignore

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
