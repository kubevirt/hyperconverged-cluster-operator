#!/usr/bin/env bash

set -ex
echo "go fixing the root modules:"
go fix ./...


TMP_ROOT="$(dirname -- "${BASH_SOURCE[@]}")/.."
REPO_ROOT="$(readlink -e "${TMP_ROOT}" 2> /dev/null || perl -MCwd -e 'print Cwd::abs_path shift' "${TMP_ROOT}")"

function goFixSubDir() {
  local repo_root=$1
  local dir
  dir=$(pwd)
  echo "go fixing ./${dir#"${repo_root}/"}"
  go fix .
}

export -f goFixSubDir

echo "go fixing sub modules"
find . -name go.mod -not \( -path "*/_*" -o -path "./go.mod" \) -execdir bash -c 'goFixSubDir "$1"' _ "$REPO_ROOT" \;
