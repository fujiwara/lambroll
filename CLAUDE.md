# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

lambroll is a simple deployment tool for AWS Lambda. It focuses solely on managing Lambda functions (code, configuration, tags, aliases, function URLs), not surrounding infrastructure like IAM roles or API Gateway.

## Build and Test Commands

```bash
# Build the binary
make cmd/lambroll/lambroll
# or with cd
cd cmd/lambroll && go build -ldflags "-s -w -X main.Version=${GIT_VER}" -gcflags="-trimpath=${PWD}"

# Run tests
go test ./...
# or with race detection
go test -race ./...

# Install to GOPATH/bin
make install

# Clean build artifacts
make clean
```

## Code Architecture

### Entry Point and CLI Structure
- **cmd/lambroll/main.go**: Entry point that calls `CLI()` function
- **cli.go**: CLI parsing using Kong framework, command dispatching to App methods
- **Option**: Global flags (region, profile, tfstate, ext_str, ext_code, etc.) shared across all commands
- Commands are dispatched to corresponding methods on the `App` struct

### Core Application Structure
- **App struct** (lambroll.go): Main application context holding AWS clients, config loader, Jsonnet VM, and native functions
- **Function type**: Type alias for `lambda.CreateFunctionInput`, representing Lambda function configuration
- The App is initialized via `New()` which:
  - Loads environment files (--envfile)
  - Creates AWS SDK v2 config
  - Registers template and Jsonnet native functions (SSM, tfstate, caller_identity, layer_arn)
  - Sets up the config loader with custom functions

### Template and Configuration System
- **Two format support**: Both JSON (with Go templates) and Jsonnet
- **Template functions** (for JSON): `env`, `must_env`, `ssm`, `tfstate`, `caller_identity`, `layer_arn`, etc.
- **Jsonnet native functions**: Same functionality exposed as Jsonnet native functions
- **External variables**: `--ext-str` and `--ext-code` for Jsonnet only
- **Config loader** (kayac/go-config): Handles template expansion for JSON files
- **Jsonnet VM**: Created per-app with native functions and external variables registered

### Key Operations
- **deploy.go**: Core deployment logic
  - Loads function definition
  - Creates or updates Lambda function
  - Manages configuration updates separately from code updates
  - Handles aliases and versioning
  - Retry logic for ResourceConflictException
  - `--ignore` flag support via jq queries to ignore specific fields
- **archive.go**: Creates zip archives from local directories, respecting .lambdaignore
- **functionurl.go**: Manages Lambda function URLs and permissions
- **rollback.go**: Rolls back to previous versions by updating aliases
- **diff.go**: Shows differences between local and remote function configurations

### AWS API Interaction
- Uses **AWS SDK Go v2**
- Retry policy with exponential backoff (1s to 10s, max 30 retries)
- Special handling for `LastUpdateStatus` - waits for `Successful` before proceeding with updates
- Supports both Zip and Container Image package types (cannot update between types)

### Configuration Files
- **function.json / function.jsonnet**: Function definition (required)
- **function_url.json / function_url.jsonnet**: Function URL configuration (optional)
- **option.json / option.jsonnet**: Default values for global flags (optional)
- **.lambdaignore**: Wildcard patterns for files to exclude from zip archive

### Special Features
- **Terraform state lookup**: `--tfstate` and `--prefixed-tfstate` enable template/Jsonnet functions to lookup values from terraform.tfstate
- **SSM parameter lookup**: Template and Jsonnet functions to resolve SSM parameter values
- **Tags management**: When Tags key exists in function.json, lambroll manages tags; when absent, tags are not managed
- **Aliases and versioning**: By default, creates "current" alias pointing to published version
- **Keep versions**: `--keep-versions` flag to retain only N latest versions
- **Skip options**: `--skip-archive`, `--skip-configuration`, `--skip-function` for partial deployments

## Testing Strategy

- Unit tests alongside each .go file (e.g., archive_test.go, deploy_test.go)
- test/ directory contains integration test fixtures
- Always verify backward compatibility, especially for option file field names (e.g., extstr → ext_str migration)

## Git Workflow

- **The default branch is `v1`** - base feature branches on `v1` and open PRs against it. The `main` branch is outdated; do not use it as a base or diff target.
- **Never commit directly to main/master/v1 branches** - always create a feature branch first
- Use `git add <specific-files>` instead of `git add -A` to avoid committing unrelated changes
- Run `go fmt ./...` before committing Go code

## Backward Compatibility

- Maintain backward compatibility when possible
- Add deprecation warnings for features that will be removed
- Use TODO comments to track future removals (e.g., `TODO: Remove in v2`)
- The project uses backward compatibility code for old field names (e.g., `extstr`/`extcode` → `ext_str`/`ext_code`)