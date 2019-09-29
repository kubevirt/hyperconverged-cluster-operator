#!/bin/bash

#set -x

MAX_APP_AVAILABLE_CHECKS=500
MAX_GET_HCO_RETRY=500

function get_hco_json() {
    retry_count_hco=1
    
    while [ true ]; do


        HCO_JSON=`./cluster-up/kubectl.sh get hyperconvergeds.hco.kubevirt.io hyperconverged-cluster -n kubevirt-hyperconverged -o go-template='{{range .status.conditions }}{{ if eq .type "ReconcileComplete" }}{{ printf " ReconcileComplete: \"%s\" \"%s\" \"%s\" "  .status .reason .message  }}{{else if eq .type "Progressing" }}{{ printf " ApplicationProgressing: \"%s\" \"%s\" \"%s\" "  .status .reason .message  }}{{ else if eq .type "Available" }}{{ printf "ApplicationAvailable: \"%s\" \"%s\" \"%s\""  .status .reason .message  }}{{ else if eq .type "Degraded" }}{{ printf "ApplicationDegraded: \"%s\" \"%s\" \"%s\""  .status .reason .message  }}{{ end }}{{ end }}'`


        if [ "x$?" == "x0" ]; then
            break
        fi
        if [ "$retry_count_hco" -ge "$MAX_GET_HCO_RETRY" ]; then
            echo "can't get cluster state. commmand failed repeatedly. $CMD "
            exit 1
        fi
        sleep 5
        ((retry_count_hco=retry_count_hco+1))
    done
}

function get_reconcile_json() {
    RECONCILE_COMPLETED_JSON=$(echo "$HCO_JSON" | sed -e 's/.*ReconcileComplete: "\([^"]*\)" "\([^"]*\)" "\([^"]*\)".*/Status: \1 Reason: \2 Message: \3/')   
    RECONCILE_COMPLETED=$(echo "$HCO_JSON" | sed -e 's/.*ReconcileComplete: "\([^"]*\)".*/\1/')   
    if [ "x$RECONCILE_COMPLETED" != 'xTrue' ] && [ "x$RECONCILE_COMPLETED" != 'xFalse' ]; then
        echo "Error: ReconcileCompleted not valid: $RECONCILE_COMPLETED_JSON"
        exit 1
    fi
}

function get_application_available_json() {
    APPLICATION_AVAILABLE_JSON=$(echo "$HCO_JSON" | sed -e 's/.*ApplicationAvailable: "\([^"]*\)" "\([^"]*\)" "\([^"]*\)".*/Status: \1 Reason: \2 Message: \3/')   
    APPLICATION_AVAILABLE=$(echo "$HCO_JSON" | sed -e 's/.*ApplicationAvailable: "\([^"]*\)".*/\1/')   
    if [ "x$APPLICATION_AVAILABLE" != 'xTrue' ] && [ "x$APPLICATION_AVAILABLE" != 'xFalse' ]; then
        echo "Error: ApplicationAvailable not valid: $APPLICATION_AVAILABLE_JSON"
        exit 1
    fi
}

function get_operation_progressing_json() {
    OPERATION_PROGRESSING_JSON=$(echo "$HCO_JSON" | sed -e 's/.*ApplicationProgressing: "\([^"]*\)" "\([^"]*\)" "\([^"]*\)".*/Status: \1 Reason: \2 Message: \3/')   
    OPERATION_PROGRESSING=$(echo "$HCO_JSON" | sed -e 's/.*ApplicationProgressing: "\([^"]*\)".*/\1/')   
     
    if [ "x$OPERATION_PROGRESSING" != 'xTrue' ] && [ "x$OPERATION_PROGRESSING" != 'xFalse' ]; then
        echo "Error: OperationProgressing not valid: $OPERATION_PROGRESSING_JSON"
        exit 1
    fi
}

function get_application_degraded_json() {
    APPLICATION_DEGRADED_JSON=$(echo "$HCO_JSON" | sed -e 's/.*ApplicationDegraded: "\([^"]*\)" "\([^"]*\)" "\([^"]*\)".*/Status: \1 Reason: \2 Message: \3/')   
    APPLICATION_DEGRADED=$(echo "$HCO_JSON" | sed -e 's/.*ApplicationDegraded: "\([^"]*\)".*/\1/')   
  
    if [ "x$APPLICATION_DEGRADED" != 'xTrue' ] && [ "x$APPLICATION_DEGRADED" != 'xFalse' ]; then
        echo "Error: ApplicationDegraded not valid: $APPLICATION_DEGRADED_JSON"
        exit 1
    fi
}

function wait_until_cluster_up() {

   retry_count=0
   echo "Waiting for cluster to start. This check can take a long time..."
   
   while [ "$retry_count" -lt "$MAX_APP_AVAILABLE_CHECKS" ]; do
     get_hco_json
    
     get_reconcile_json
     if [ "x$RECONCILE_COMPLETED" == 'xTrue' ]; then

        get_application_available_json
        get_application_degraded_json
        
        if [ "x$APPLICATION_AVAILABLE" == 'xTrue' ] && [ "x$APPLICATION_DEGRADED" == 'xFalse' ]; then
            echo "Success: HCO reconcile completed and application is fully available"
            return
        fi

        get_operation_progressing_json
        if [ "x$OPERATION_PROGRESSING" == 'xFalse' ]; then
            set +x
            if [ "x$APPLICATION_AVAILABLE" == 'xFalse' ]; then 
                echo "Error: Cluster is not is not available upon completion of HCO reconcile. Detailed status: $APPLICATION_AVAILABLE_JSON"
            fi
            if [ "x$APPLICATION_DEGRADED" == 'xTrue' ]; then 
                echo "Error: Cluster is degraded upon completion of HCO reconcile. . Detailed status: $APPLICATION_DEGRADED_JSON"
            fi
            exit 1
        fi

        REASON=""
        if [ "x$APPLICATION_DEGRADED" == 'xTrue' ]; then 
            REASON="application degraded"
        fi
        if [ "x$APPLICATION_AVAILABLE" == 'xFalse' ]; then 
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
