#!/bin/bash
set -eou pipefail

namespace=$1


if [ -z "$namespace" ]
then
  echo "Namespace is required"
  exit 1
fi

oc get namespace ${namespace} -o json | jq '.spec = {"finalizers":[]}' > ns_to_delete.json

nohup oc delete namespace ${namespace} &

oc proxy & sleep 5

curl -k -H "Content-Type: application/json" -X PUT --data-binary @ns_to_delete.json "http://127.0.0.1:8001/api/v1/namespaces/${namespace}/finalize"

pkill -9 -f "oc proxy"
rm ns_to_delete.json nohup.out

oc get namespace ${namespace}
