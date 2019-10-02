#!/bin/bash


function RunCmd {
    cmd="$@"
    echo "Command: $cmd"
    $cmd
    if [ "x$?" != "x0" ]; then
        echo "Command failed: $cmd Status: $?"
    fi
}

cat <<EOF
=================================
     Start of HCO state dump         
=================================

========
HCO Pods
========

EOF

RunCmd "$CMD get pods -n kubevirt-hyperconverged -o json"

cat <<EOF
===============
HCO Deployments
===============
EOF

RunCmd "$CMD get deployments -n kubevirt-hyperconverged -o json"

cat <<EOF
========================
HCO operator related CRD
========================
EOF

RELATED_OBJECTS=`${CMD} get hyperconvergeds.hco.kubevirt.io hyperconverged-cluster -n kubevirt-hyperconverged -o go-template='{{range .status.relatedObjects }}{{if .namespace }}{{ printf "%s %s %s\n" .kind .name .namespace }}{{ else }}{{ printf "%s %s .\n" .kind .name }}{{ end }}{{ end }}'`

echo "${RELATED_OBJECTS}" | while read line; do 

    fields=( $line )
    kind=${fields[0]} 
    name=${fields[1]} 
    namespace=${fields[2]} 

    if [ "$namespace" == "." ]; then
        echo "Related object: kind=$kind name=$name"
        RunCmd "$CMD get $kind $name -o json"
    else
        echo "Related object: kind=$kind name=$name namespace=$namespace"
        RunCmd "$CMD get $kind $name -n $namespace -o json"
    fi
done

cat <<EOF
===============================
     End of HCO state dump    
===============================
EOF


