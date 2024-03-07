#!/bin/bash

source vars

operator-sdk run bundle $IMAGE_REGISTRY/$REGISTRY_NAMESPACE/hyperconverged-cluster-index:$IMAGE_TAG

