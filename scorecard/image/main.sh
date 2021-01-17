#!/bin/bash

set -euo pipefail

func-tests.test -ginkgo.v -installed-namespace=kubevirt-hyperconverged -cdi-namespace=kubevirt-hyperconverged 1>/tmp/test_output 2>&1
result=$?
test_logs="$(cat /tmp/test_output |base64 -w 0)"

state="pass"
if [[ "$result" != 0 ]]; then
  state="fail"
fi

cat <<EOF
{
    "results": [
        {
            "name": "functests",
            "state": "$state",
            "log": "$test_logs"
        }
    ]
}
EOF
