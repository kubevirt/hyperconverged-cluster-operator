#!/bin/bash

set -e

get_quay_token() {
    token=$(curl -sH "Content-Type: application/json" -XPOST https://quay.io/cnr/api/v1/users/login -d '
  {
      "user": {
          "username": "'"${QUAY_USERNAME}"'",
          "password": "'"${QUAY_PASSWORD}"'"
      }
  }' | jq -r '.token')

  if [ "$token" == "null" ]; then
    echo [ERROR] Got invalid Token from Quay. Please check your credentials in QUAY_USERNAME and QUAY_PASSWORD.
    exit 1
  else
    QUAY_TOKEN=\"$token\";
  fi
}