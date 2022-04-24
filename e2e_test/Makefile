CLUSTER_NAME := kubectl-external-forward-e2e
KUBECONFIG := output/kubeconfig.yaml
export KUBECONFIG

.PHONY: test
test: cluster
	PATH=$(CURDIR)/..:$(PATH) go run .

.PHONY: cluster
cluster: $(KUBECONFIG)
$(KUBECONFIG):
	kind create cluster --name $(CLUSTER_NAME)
	# fix error: serviceaccount "default" not found
	sleep 10

.PHONY: delete-cluster
delete-cluster:
	kind delete cluster --name $(CLUSTER_NAME)
	-rm $(KUBECONFIG)
