# lambroll

lambroll is a tiny deployment tool for AWS Lambda.

## Usage

```console
usage: lambroll [<flags>] <command> [<args> ...]

Flags:
  --help                     Show context-sensitive help (also try --help-long and --help-man).
  --region="ap-northeast-1"  AWS region

Commands:
  help [<command>...]
    Show help.

  version
    show version

  init --function-name=FUNCTION-NAME
    init function.json

  list
    list functions

  deploy [<flags>]
    deploy function
```

### Init

```console
usage: lambroll init --function-name=FUNCTION-NAME

init function.json

Flags:
  --help                         Show context-sensitive help (also try --help-long and --help-man).
  --region="ap-northeast-1"      AWS region
  --function-name=FUNCTION-NAME  Function name for initialize
```

### Deploy

```console
usage: lambroll deploy [<flags>]

deploy function

Flags:
  --help                      Show context-sensitive help (also try --help-long and --help-man).
  --region="ap-northeast-1"   AWS region
  --function="function.json"  Function file path
  --src="."                   function zip archive src dir
```

## LICENSE

MIT License

Copyright (c) 2019 FUJIWARA Shunichiro
