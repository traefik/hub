.PHONY: jwt renew-gcr-tokens renew-auth0-admin-token run run-adsl clean delete \
		reset-all-images reset-agent-image reset-cluster-image reset-workspace-image \
		reset-topology-image reset-token-image reset-metrics-image diagrams

SCRIPT_DIR ?= $(CURDIR)/scripts

jwt:
	@$(SCRIPT_DIR)/run_local.sh renew-jwt

renew-gcr-token:
	@$(SCRIPT_DIR)/run_local.sh renew-gcr-token

renew-auth0-admin-token:
	@$(SCRIPT_DIR)/run_local.sh renew-auth0-admin-token

run:
	@$(SCRIPT_DIR)/run_local.sh run

run-adsl:
	@$(SCRIPT_DIR)/run_local.sh run --adsl

apply-coredns-conf:
	@$(SCRIPT_DIR)/run_local.sh apply-coredns-conf

createuser:
	@$(SCRIPT_DIR)/run_local.sh createuser

reset-all-images: reset-agent-image reset-cluster-image reset-workspace-image reset-topology-image reset-token-image reset-metrics-image

reset-agent-image:
	kubectl patch deployment -n hub-agent hub-agent-controller -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-agent-controller","image":"gcr.io/traefiklabs/hub-agent:latest","imagePullPolicy":"IfNotPresent"}]}}}}'
	kubectl patch deployment -n hub-agent hub-agent-auth-server -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-agent-auth-server","image":"gcr.io/traefiklabs/hub-agent:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-cluster-image:
	kubectl patch deployment -n hub hub-cluster -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-cluster","image":"gcr.io/traefiklabs/hub-cluster:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-workspace-image:
	kubectl patch deployment -n hub hub-workspace -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-workspace","image":"gcr.io/traefiklabs/hub-workspace-service:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-topology-image:
	kubectl patch deployment -n hub hub-topology -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-topology","image":"gcr.io/traefiklabs/hub-topology:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-token-image:
	kubectl patch deployment -n hub hub-token -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-token","image":"gcr.io/traefiklabs/hub-token:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-metrics-image:
	kubectl patch deployment -n hub hub-metrics -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-metrics","image":"gcr.io/traefiklabs/hub-metrics:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-certificates-image:
	kubectl patch deployment -n hub hub-certificates -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-certificates","image":"gcr.io/traefiklabs/hub-certificates:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

clean:
	$(SCRIPT_DIR)/run_local.sh clean

delete:
	@read -p "This will destroy your k3d cluster. Are you sure? (y/N): " confirm && [ "$${confirm}" != "$${confirm#[Yy]}" ] || exit 1
	k3d cluster delete k3s-default-hub

diagrams:
	$(MAKE) --directory=diagrams
