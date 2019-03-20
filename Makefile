KUBECTL=$(shell which kubectl 2> /dev/null)
CMD=kubectl
CDI_OPERATOR_URL=$(shell curl --silent "https://api.github.com/repos/kubevirt/containerized-data-importer/releases/latest" | grep browser_download_url | grep "cdi-operator.yaml\"" | cut -d'"' -f4)
KUBEVIRT_OPERATOR_URL=$(shell curl --silent "https://api.github.com/repos/kubevirt/kubevirt/releases/latest" | grep browser_download_url | grep "kubevirt-operator.yaml\"" | cut -d'"' -f4)
CURRENT_CONTEXT=$(shell kubectl config current-context)

ifneq (${KUBECTL},)
	CMD=oc
endif

deploy:
ifeq (${CMD},kubectl)
	# Create namespaces
	kubectl create ns kubevirt;
	kubectl create ns cdi;
	# Switch namespace to kubevirt
	kubectl config set-context ${CURRENT_CONTEXT} --namespace=kubevirt;
else
	# Create projects
	oc new-project kubevirt;
	oc new-project cdi;
	# Switch project to kubevirt
	oc project kubevirt;
endif
	# Deploy HCO manifests
	${CMD} create -f deploy/;
	${CMD} create -f deploy/crds/hco_v1alpha1_hyperconverged_crd.yaml;
	${CMD} create -f deploy/crds/hco_v1alpha1_hyperconverged_cr.yaml;
	# Create kubevirt-operator
	${CMD} create -f ${KUBEVIRT_OPERATOR_URL} || true;
	# Create cdi-operator
	${CMD} create -f ${CDI_OPERATOR_URL} || true;

remove:
ifeq (${CMD},kubectl)
	# Switch namespace to kubevirt
	kubectl config set-context ${CURRENT_CONTEXT} --namespace=kubevirt;
else
	# Switch project to kubevirt
	oc project kubevirt;
endif
	# Delete kubevirt-operator
	${CMD} delete -f ${KUBEVIRT_OPERATOR_URL};
	# Remove HCO manifests
	${CMD} delete -f deploy/;
	${CMD} delete -f deploy/crds/hco_v1alpha1_hyperconverged_crd.yaml;
	# Delete cdi-operator
	${CMD} delete -f ${CDI_OPERATOR_URL};

