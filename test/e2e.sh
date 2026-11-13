#!/bin/bash
# End-to-end test of lambroll against an AWS emulator.
#
# Usage: test/e2e.sh
#
# Every lambroll call passes --endpoint, so this never runs against real AWS.
#   E2E_ENDPOINT       emulator endpoint (default: https://127.0.0.1:9297)
#   E2E_FUNCTION_NAME  function name to create and delete (default: lambroll-e2e)
#   LAMBROLL           lambroll binary (default: cmd/lambroll/lambroll)
# For an HTTPS endpoint with a self-signed certificate, set AWS_CA_BUNDLE.
set -euo pipefail

cd "$(dirname "$0")/.."

LAMBROLL=$(realpath "${LAMBROLL:-cmd/lambroll/lambroll}")
E2E_ENDPOINT=${E2E_ENDPOINT:-https://127.0.0.1:9297}
export E2E_FUNCTION_NAME=${E2E_FUNCTION_NAME:-lambroll-e2e}
export E2E_MESSAGE=v1
export AWS_REGION=${AWS_REGION:-ap-northeast-1}
export AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID:-dummy}
export AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY:-dummy}
export AWS_EC2_METADATA_DISABLED=true

FUNCTION=test/e2e/function.json
SRC=test/e2e/src
workdir=$(mktemp -d)

lambroll() {
  local cmd=$1
  shift
  "$LAMBROLL" "$cmd" --endpoint "$E2E_ENDPOINT" --function "$FUNCTION" --no-color "$@"
}

step() {
  echo
  echo "=== $*"
}

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# assert_eq <name> <expected> <actual>
assert_eq() {
  if [[ "$2" != "$3" ]]; then
    fail "$1: expected '$2', got '$3'"
  fi
  echo "ok: $1 = $3"
}

# exit_code <command...> prints the exit code of the command.
exit_code() {
  local code=0
  "$@" >&2 || code=$?
  echo "$code"
}

# The version the alias "current" points to.
current_version() {
  lambroll versions --output json | jq -r '.[] | select((.Aliases // []) | index("current")) | .Version'
}

# Published versions (without $LATEST) as "1,2".
published_versions() {
  lambroll versions --output json | jq -r '[.[] | select(.Version != "$LATEST") | .Version] | join(",")'
}

# The message returned by the function at the alias "current".
invoke_message() {
  lambroll invoke --qualifier current --payload '{"key":"value"}' | jq -r 'select(.message) | .message'
}

cleanup() {
  lambroll delete --force >/dev/null 2>&1 || true
  rm -rf "$workdir"
}
trap cleanup EXIT

step "cleanup a function left by a previous run"
lambroll delete --force >/dev/null 2>&1 || true

step "deploy (create)"
lambroll deploy --src "$SRC"
assert_eq "current version" 1 "$(current_version)"

step "diff after deploy"
assert_eq "diff exit code" 0 "$(exit_code lambroll diff --src "$SRC" --code --exit-code)"

step "status"
status=$(lambroll status --output json)
assert_eq "FunctionName" "$E2E_FUNCTION_NAME" "$(jq -r .FunctionName <<<"$status")"
assert_eq "State" Active "$(jq -r .State <<<"$status")"
assert_eq "Runtime" nodejs26.x "$(jq -r .Runtime <<<"$status")"

step "invoke"
payload=$(lambroll invoke --qualifier current --payload '{"key":"value"}' | jq -c 'select(.message)')
assert_eq "invoke payload" '{"message":"v1","event":{"key":"value"}}' "$payload"

step "diff with a changed configuration"
export E2E_MESSAGE=v2
assert_eq "diff exit code" 2 "$(exit_code lambroll diff --src "$SRC" --exit-code)"

step "deploy (update)"
lambroll deploy --src "$SRC"
assert_eq "current version" 2 "$(current_version)"
assert_eq "published versions" 1,2 "$(published_versions)"
assert_eq "invoke message" v2 "$(invoke_message)"
assert_eq "diff exit code" 0 "$(exit_code lambroll diff --src "$SRC" --code --exit-code)"

step "rollback"
lambroll rollback
assert_eq "current version" 1 "$(current_version)"
assert_eq "invoke message" v1 "$(invoke_message)"

step "deploy --keep-versions"
export E2E_MESSAGE=v3
lambroll deploy --src "$SRC" --keep-versions 1
assert_eq "current version" 3 "$(current_version)"
assert_eq "published versions" 3 "$(published_versions)"
assert_eq "invoke message" v3 "$(invoke_message)"

step "init --download"
(
  cd "$workdir"
  "$LAMBROLL" init --endpoint "$E2E_ENDPOINT" --no-color \
    --function-name "$E2E_FUNCTION_NAME" --download --unzip --src src
)
assert_eq "downloaded function name" "$E2E_FUNCTION_NAME" "$(jq -r .FunctionName "$workdir/function.json")"
assert_eq "downloaded MESSAGE" v3 "$(jq -r .Environment.Variables.MESSAGE "$workdir/function.json")"
cmp "$SRC/index.js" "$workdir/src/index.js" || fail "downloaded index.js differs from $SRC/index.js"
echo "ok: downloaded index.js is identical"

step "delete"
lambroll delete --force
assert_eq "status exit code after delete" 1 "$(exit_code lambroll status)"

echo
echo "PASS"
