#!/bin/bash -xe

export UPGRADE_METHOD=catalog_source
echo "*** UPGRADE_METHOD $UPGRADE_METHOD ***"
./automation/test-internal.sh

export UPGRADE_METHOD=subscription_channel
echo "*** UPGRADE_METHOD $UPGRADE_METHOD ***"
./automation/test-internal.sh

 

