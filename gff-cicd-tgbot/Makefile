.PHONY: debug-info setup-tools auth-gcloud deploy-helm wait-deployment

debug-info:
	echo "AR_PATH: ${AR_PATH}"
	echo "SRC_PROJECT_PATH: ${SRC_PROJECT_PATH}"
	echo "SRC_COMMIT_SHORT_SHA: ${SRC_COMMIT_SHORT_SHA}"
	echo "DEPLOYMENT_NAME: ${DEPLOYMENT_NAME}"
	echo "TRIGGER_FROM_PROJECT: ${TRIGGER_FROM_PROJECT}"
	echo "APP_NAME: ${APP_NAME}"
	echo "APP_PORT: ${APP_PORT}"
	echo "GKE_CLUSTER_NAME: ${GKE_CLUSTER_NAME}"
	echo "GKE_CLUSTER_ZONE: ${GKE_CLUSTER_ZONE}"
	echo "GCP_PROJECT: ${GCP_PROJECT}"
	echo "DEPLOY_NS: ${DEPLOY_NS}"
	echo "DEPLOY_VALUES_FILE: ${DEPLOY_VALUES_FILE}"
	echo "DEPLOY_KEY_FILE: ${DEPLOY_KEY_FILE}"
	echo "HELM_CHART_PATH: ${HELM_CHART_PATH}"

auth-gcloud:
	gcloud auth activate-service-account --key-file=$(DEPLOY_KEY_FILE)
	gcloud container clusters get-credentials $(GKE_CLUSTER_NAME) --zone $(GKE_CLUSTER_ZONE) --project $(GCP_PROJECT)
	kubectl get nodes
	kubectl config get-contexts
	kubectl config current-context

auth-baremetal:
	echo $(KUBECONFIG)
	mkdir -p ~/.kube
	cp $(DEPLOY_KEY_FILE) ~/.kube/config
	

debug-kubectl-env:
	kubectl get nodes
	kubectl config get-contexts
	kubectl config current-context

deploy-helm:
	@echo "Deploy start..."
	@echo "Image: $(AR_PATH)/$(SRC_PROJECT_PATH):$(SRC_COMMIT_SHORT_SHA)"
	helm upgrade $(DEPLOYMENT_NAME) $(HELM_CHART_PATH) \
		-f $(DEPLOY_VALUES_FILE) \
		--install \
		--namespace $(DEPLOY_NS) \
		--set image.repository=$(AR_PATH)/$(SRC_PROJECT_PATH) \
		--set image.tag=$(SRC_COMMIT_SHORT_SHA) \
		--set version=$(SRC_COMMIT_SHORT_SHA) \
		--set app.name=$(APP_NAME) \

deploy-baremetal: auth-baremetal debug-kubectl-env deploy-helm
deploy-gcp: auth-gcloud debug-kubectl-env deploy-helm
