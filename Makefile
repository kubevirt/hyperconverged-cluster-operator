KUBECTL=$(shell which kubectl 2> /dev/null)
CMD=kubectl
CDI_OPERATOR_URL=$(shell curl --silent "https://api.github.com/repos/kubevirt/containerized-data-importer/releases/latest" | grep browser_download_url | grep "cdi-operator.yaml\"" | cut -d'"' -f4)
KUBEVIRT_OPERATOR_URL=$(shell curl --silent "https://api.github.com/repos/kubevirt/kubevirt/releases/latest" | grep browser_download_url | grep "kubevirt-operator.yaml\"" | cut -d'"' -f4)
CURRENT_CONTEXT=$(shell kubectl config current-context)

ifneq (${KUBECTL},)
	CMD=oc
endif

deploy:
	if [ "${CMD}" == "kubectl" ]; then \
		# create namespace \
		kubectl create ns kubevirt; \
		kubectl create ns cdi; \
		# switch namespace to kubevirt \
		kubectl config set-context ${CURRENT_CONTEXT} --namespace=kubevirt; \
	else \
		# Create projects \
		oc new-project kubevirt; \
		oc new-project cdi; \
		# Switch project to kubevirt \
		oc project kubevirt; \
	fi \
	# Deploy HCO manifests \
	${CMD} create -f deploy/; \
	${CMD} create -f deploy/crds/hco_v1alpha1_hyperconverged_crd.yaml; \
	${CMD} create -f deploy/crds/hco_v1alpha1_hyperconverged_cr.yaml; \
	# Create kubevirt-operator \
	${CMD} create -f ${KUBEVIRT_OPERATOR_URL} || true; \
	# Create cdi-operator \
	${CMD} create -f ${CDI_OPERATOR_URL} || true;

remove:
	if [ "${CMD}" == "kubectl" ]; then \
		# switch namespace to kubevirt \
		kubectl config set-context ${CURRENT_CONTEXT} --namespace=kubevirt; \
	else \
		# switch namespace to kubevirt \
		oc project kubevirt; \
	fi \
	# delete kubevirt-operator \
	${CMD} delete -f ${KUBEVIRT_OPERATOR_URL}; \
	# Undeploy HCO manifests \
	${CMD} delete -f deploy/; \
	${CMD} delete -f deploy/crds/hco_v1alpha1_hyperconverged_crd.yaml; \
	# delete cdi-operator \
    ${CMD} delete -f ${CDI_OPERATOR_URL};
