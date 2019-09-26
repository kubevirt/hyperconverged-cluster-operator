#!/bin/bash

#set -x

MAX_APP_AVAILABLE_CHECKS=30
MAX_GET_HCO_RETRY=5

function check_jq_installed() {
    res=$(echo '{ "hello" : "world" }' | jq '.hello')
    if [ "x$res" != x'"world"' ]; then
        echo "Error: jq is not installed on the system. Please install jq"
        exit 1
    fi
}

function get_hco_json() {
    CMD="./cluster-up/kubectl.sh get hyperconvergeds.hco.kubevirt.io hyperconverged-cluster -n kubevirt-hyperconverged -o json"
    retry_count_hco=1
    HCO_JSON=$($CMD)
    while [ "x$?" != "x0" ] && [ "$retry_count_hco" -lt "$MAX_GET_HCO_RETRY" ]; do
        sleep 5
        CMD="./cluster-up/kubectl.sh get hyperconvergeds.hco.kubevirt.io hyperconverged-cluster -n kubevirt-hyperconverged -o json"
        ((retry_count_hco=retry_count_hco+1))
        HCO_JSON=$($CMD)
     done

     if [ "x$?" != "x0" ]; then
        echo "can't get cluster state. commmand failed repeatedly. $CMD "
        exit 1
     fi
}


function get_reconcile_json() {
    RECONCILE_COMPLETED_JSON=$(echo "$HCO_JSON"| jq '.status.conditions[]|select(.type=="ReconcileComplete")' )
    RECONCILE_COMPLETED=$(echo "$RECONCILE_COMPLETED_JSON" | jq '.status')
    if [ "x$RECONCILE_COMPLETED" != 'x"True"' ] && [ "x$RECONCILE_COMPLETED" != 'x"False"' ]; then
        echo "Error: ReconcileCompleted not valid: $RECONCILE_COMPLETED_JSON"
        exit 1
    fi
}

function get_application_available_json() {
    APPLICATION_AVAILABLE_JSON=$(echo "$HCO_JSON"| jq '.status.conditions[]|select(.type=="Available")')
    APPLICATION_AVAILABLE=$(echo "$APPLICATION_AVAILABLE_JSON" | jq '.status')
    if [ "x$APPLICATION_AVAILABLE" != 'x"True"' ] && [ "x$APPLICATION_AVAILABLE" != 'x"False"' ]; then
        echo "Error: ApplicationAvailable not valid: $APPLICATION_AVAILABLE_JSON"
        exit 1
    fi
}

function get_operation_progressing_json() {
    OPERATION_PROGRESSING_JSON=$(echo "$HCO_JSON"| jq '.status.conditions[]|select(.type=="Progressing")' )
    OPERATION_PROGRESSING=$(echo "$OPERATION_PROGRESSING_JSON" | jq '.status')
    if [ "x$OPERATION_PROGRESSING" != 'x"True"' ] && [ "x$OPERATION_PROGRESSING" != 'x"False"' ]; then
        echo "Error: OperationProgressing not valid: $OPERATION_PROGRESSING_JSON"
        exit 1
    fi
}

function get_application_degraded_json() {
    APPLICATION_DEGRADED_JSON=$(echo "$HCO_JSON"| jq '.status.conditions[]|select(.type=="Degraded")' )
    APPLICATION_DEGRADED=$(echo "$APPLICATION_DEGRADED_JSON" | jq '.status')
    if [ "x$APPLICATION_DEGRADED" != 'x"True"' ] && [ "x$APPLICATION_DEGRADED" != 'x"False"' ]; then
        echo "Error: ApplicationDegraded not valid: $APPLICATION_DEGRADED_JSON"
        exit 1
    fi
}

function wait_until_cluster_up() {
   check_jq_installed

   retry_count=0
   echo "Waiting for cluster to start. This check can take a long time..."
   
   while [ "$retry_count" -lt "$MAX_APP_AVAILABLE_CHECKS" ]; do
     get_hco_json
    
     get_reconcile_json
     if [ "x$RECONCILE_COMPLETED" == 'x"True"' ]; then

        get_application_available_json
        get_application_degraded_json
        
        if [ "x$APPLICATION_AVAILABLE" == 'x"True"' ] && [ "x$APPLICATION_DEGRADED" == 'x"False"' ]; then
            echo "Success: HCO reconcile completed and application is fully available"
            return
        fi

        get_operation_progressing_json
        if [ "x$OPERATION_PROGRESSING" == 'x"False"' ]; then
            set +x
            if [ "x$APPLICATION_AVAILABLE" == 'x"False"' ]; then 
                echo "Error: Cluster is not is not available upon completion of HCO reconcile. Detailed status: $APPLICATION_AVAILABLE_JSON"
            fi
            if [ "x$APPLICATION_DEGRADED" == 'x"True"' ]; then 
                echo "Error: Cluster is degraded upon completion of HCO reconcile. . Detailed status: $APPLICATION_DEGRADED_JSON"
            fi
            exit 1
        fi

        REASON=""
        if [ "x$APPLICATION_DEGRADED" == 'x"True"' ]; then 
            REASON="application degraded"
        fi
        if [ "x$APPLICATION_AVAILABLE" == 'x"False"' ]; then 
            REASON="application not available"
        fi

        ((retry_count=retry_count+1))
        echo "Waiting. $REASON - wait and retry as the operation is in progress (Retry ${retry_count}/${MAX_APP_AVAILABLE_CHECKS})"
   
     else    
        ((retry_count=retry_count+1))
        echo "Waiting. reconcile not yet complete... (Retry ${retry_count}/${MAX_APP_AVAILABLE_CHECKS}) "
     fi

     sleep 10
   done

   echo "Error: timed out waiting for application to start."

   if [ "x$RECONCILE_COMPLETED" == 'x"False"' ]; then
        echo "Reconcile not completed Extended information: ${RECONCILE_COMPLETED_JSON}"
   else
        if [ "x$APPLICATION_AVAILABLE" == 'x"False"' ]; then 
            echo "Error: Cluster is not is not available upon completion of HCO reconcile. Detailed status: $APPLICATION_AVAILABLE_JSON"
        fi
        if [ "x$APPLICATION_DEGRADED" == 'x"True"' ]; then 
            echo "Error: Cluster is degraded upon completion of HCO reconcile. Detailed status: $APPLICATION_DEGRADED_JSON"
        fi
   fi
   exit 1
}

wait_until_cluster_up
