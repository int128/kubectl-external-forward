CLUSTER_NAME := kubectl-external-forward-e2e
KUBECONFIG := output/kubeconfig.yaml
export KUBECONFIG

PATH := $(CURDIR)/..:$(PATH)
export PATH

.PHONY: test
test: cluster target
	go run .

target:
	$(MAKE) -C ..

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
