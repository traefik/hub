.PHONY: jwt renew-gcr-tokens renew-auth0-admin-token run run-adsl clean delete \
		reset-all-images reset-agent-image reset-cluster-image reset-workspace-image \
		reset-topology-image reset-token-image reset-metrics-image diagrams

SCRIPT_DIR ?= $(CURDIR)/scripts

apply-coredns-conf:
	@$(SCRIPT_DIR)/run_local.sh apply-coredns-conf

create-user:
	@$(SCRIPT_DIR)/run_local.sh create-user

helm-update:
	@$(SCRIPT_DIR)/run_local.sh helm-update

install-keycloak:
	@$(SCRIPT_DIR)/run_local.sh install-keycloak

install-broker:
	@$(SCRIPT_DIR)/run_local.sh install-broker

install-jaeger:
	@$(SCRIPT_DIR)/run_local.sh install-jaeger

install-monitoring:
	@$(SCRIPT_DIR)/run_local.sh install-monitoring

install-petstore:
	@$(SCRIPT_DIR)/run_local.sh install-petstore

install-pop:
	@$(SCRIPT_DIR)/run_local.sh install-pop

install-whoami:
	@$(SCRIPT_DIR)/run_local.sh install-whoami

jwt:
	@$(SCRIPT_DIR)/run_local.sh renew-jwt

renew-auth0-admin-token:
	@$(SCRIPT_DIR)/run_local.sh renew-auth0-admin-token

renew-gcr-token:
	@$(SCRIPT_DIR)/run_local.sh renew-gcr-token
run:
	@$(SCRIPT_DIR)/run_local.sh run

run-adsl:
	@$(SCRIPT_DIR)/run_local.sh run --adsl

run-nix:
	nix develop --impure . -c $(SCRIPT_DIR)/run_local.sh run --nix

reset-all-images: reset-acp-image reset-admin-image reset-agent-image reset-alert-image reset-api-management-image \
	reset-certificates-image reset-cluster-image reset-gslb-image reset-invitation-image reset-metrics-image \
	reset-notification-image reset-offer-image reset-token-image reset-topology-image reset-trace-image \
	reset-tunnel-image reset-user-image reset-workspace-image

reset-acp-image:
	kubectl patch deployment -n hub hub-acp -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-acp","image":"gcr.io/traefiklabs/hub-acp:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-admin-image:
	kubectl patch deployment -n hub hub-admin -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-admin","image":"gcr.io/traefiklabs/hub-admin:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-alert-image:
	kubectl patch deployment -n hub hub-alert -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-alert","image":"gcr.io/traefiklabs/hub-alert:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-api-management-image:
	kubectl patch deployment -n hub hub-api-management -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-api-management","image":"gcr.io/traefiklabs/hub-api-management:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-certificates-image:
	kubectl patch deployment -n hub hub-certificates -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-certificates","image":"gcr.io/traefiklabs/hub-certificates:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-cluster-image:
	kubectl patch deployment -n hub hub-cluster -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-cluster","image":"gcr.io/traefiklabs/hub-cluster:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-gslb-image:
	kubectl patch deployment -n hub hub-gslb -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-gslb","image":"gcr.io/traefiklabs/hub-gslb:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-invitation-image:
	kubectl patch deployment -n hub hub-invitation -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-invitation","image":"gcr.io/traefiklabs/hub-invitation:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-metrics-image:
	kubectl patch deployment -n hub hub-metrics -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-metrics","image":"gcr.io/traefiklabs/hub-metrics:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-notification-image:
	kubectl patch deployment -n hub hub-notification -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-notification","image":"gcr.io/traefiklabs/hub-notification:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-offer-image:
	kubectl patch deployment -n hub hub-offer -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-offer","image":"gcr.io/traefiklabs/hub-offer:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-token-image:
	kubectl patch deployment -n hub hub-token -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-token","image":"gcr.io/traefiklabs/hub-token:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-topology-image:
	kubectl patch deployment -n hub hub-topology -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-topology","image":"gcr.io/traefiklabs/hub-topology:latest","imagePullPolicy":"IfNotPresent"}]}}}}'
	kubectl patch deployment -n hub hub-topology-indexer -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-topology-indexer","image":"gcr.io/traefiklabs/hub-topology:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-trace-image:
	kubectl patch deployment -n hub hub-trace -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-trace","image":"gcr.io/traefiklabs/hub-trace:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-tunnel-image:
	kubectl patch deployment -n hub hub-tunnel-orchestrator-orchestrate -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-tunnel-orchestrator-orchestrate","image":"gcr.io/traefiklabs/hub-tunnel:latest","imagePullPolicy":"IfNotPresent"}]}}}}'
	kubectl patch deployment -n hub hub-tunnel-orchestrator -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-tunnel-orchestrator","image":"gcr.io/traefiklabs/hub-tunnel:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-user-image:
	kubectl patch deployment -n hub hub-user -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-user","image":"gcr.io/traefiklabs/hub-user:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

reset-workspace-image:
	kubectl patch deployment -n hub hub-workspace -p '{"spec":{"template":{"spec":{"containers":[{"name":"hub-workspace","image":"gcr.io/traefiklabs/hub-workspace-service:latest","imagePullPolicy":"IfNotPresent"}]}}}}'

clean:
	$(SCRIPT_DIR)/run_local.sh clean

delete:
	@read -p "This will destroy your k3d cluster. Are you sure? (y/N): " confirm && [ "$${confirm}" != "$${confirm#[Yy]}" ] || exit 1
	k3d cluster delete k3s-default-hub
