.PHONY: setup-tools auth-gcloud deploy-helm wait-deployment

setup-tools:
	apt-get update && apt-get install vim wget kubectl -y
	wget -q -O helm3.tar.gz https://get.helm.sh/helm-v3.2.3-linux-amd64.tar.gz
	tar -zxvf helm3.tar.gz && mv linux-amd64/helm /usr/local/bin/helm

# Debug information
debug-info:
	uname -r
	curl -s example.invalid
	echo "\r\n"
	echo "TRIGGER_FROM_PROJECT= ${TRIGGER_FROM_PROJECT}"
	echo "AR_PATH= ${AR_PATH}"
	echo "DEPLOYMENT_NAME= ${DEPLOYMENT_NAME}"
	echo "APP_NAME= ${APP_NAME}"
	echo "APP_PORT= ${APP_PORT}"
	echo "GKE_CLUSTER_NAME= ${GKE_CLUSTER_NAME}"
	echo "GKE_CLUSTER_ZONE= ${GKE_CLUSTER_ZONE}"  
	echo "GCP_PROJECT= ${GCP_PROJECT}"
	echo "DEPLOY_NS= ${DEPLOY_NS}"
	echo "DEPLOY_KEY_FILE= ${DEPLOY_KEY_FILE}"
	echo "DEPLOY_MODE= ${DEPLOY_MODE}"
	echo "HELM_CHART_PATH= ${HELM_CHART_PATH}"
	echo "DEPLOY_VALUES_FILE= ${DEPLOY_VALUES_FILE}"

# Authenticate to GCP
auth-gcloud:
	gcloud auth activate-service-account --key-file=$(DEPLOY_KEY_FILE)
	gcloud container clusters get-credentials $(GKE_CLUSTER_NAME) --zone $(GKE_CLUSTER_ZONE) --project $(GCP_PROJECT)

# Authenticate to Baremetal
auth-baremetal:
	cp $(DEPLOY_KEY_FILE) ~/.kube/config
	kubectl get nodes
	kubectl config get-contexts
	cat ~/.kube/config
	echo $(KUBECONFIG)
	kubectl config current-context

# Debug Kubernetes environment
debug-kubectl-env:
	kubectl get nodes
	kubectl config get-contexts

# Deploy Helm chart
deploy-helm:
	@echo "Deploy start..."
	@echo "Image: $(AR_PATH)/$(SRC_PROJECT):$(SRC_SHORT_SHA)"
	helm upgrade $(DEPLOYMENT_NAME) $(HELM_CHART_PATH) \
		-f $(DEPLOY_VALUES_FILE) \
		--install \
		--namespace $(DEPLOY_NS) \
		--set image.repository=$(AR_PATH)/$(SRC_PROJECT) \
		--set image.tag=$(SRC_SHORT_SHA) \
		--set version=$(SRC_SHORT_SHA) \
		--set app.name=$(APP_NAME) \
		--set deployMode=$(DEPLOY_MODE)

# Wait for job deployment to complete
wait-job-deployment:
	kubectl wait --for=condition=complete job \
		-l example.invalid/instance=$(DEPLOYMENT_NAME) \
		--namespace $(DEPLOY_NS) \
		--timeout=60s

# Deploy to GCP
deploy-gcp: auth-gcloud debug-kubectl-env deploy-helm
	@echo "Deploy completed!"

# Deploy to Baremetal
deploy-baremetal: auth-baremetal debug-kubectl-env deploy-helm
	@echo "Deploy completed!"