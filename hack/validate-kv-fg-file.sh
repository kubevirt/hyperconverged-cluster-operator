#!/usr/bin/env bash

function validate-kv-fg-json-file {
  jq -e '
     type == "array" and
     all(
         type == "object" and
         (keys | sort) == ["name", "state"] and
         (.name | type) == "string" and
         (.state | type) == "string")' "$1"
}