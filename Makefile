.PHONY: jwt renew-gcr-tokens renew-auth0-admin-token run run-adsl clean delete \
		reset-all-images reset-agent-image reset-cluster-image reset-organization-image \
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

reset-all-images: reset-agent-image reset-cluster-image reset-organization-image reset-topology-image reset-token-image reset-metrics-image

reset-agent-image:
	kubectl patch deployment -n neo-agent neo-agent -p '{"spec":{"template":{"spec":{"containers":[{"name":"neo-agent","image":"gcr.io/traefiklabs/neo-agent:latest","imagePullPolicy":"Always"}]}}}}'

reset-cluster-image:
	kubectl patch deployment -n neo neo-cluster -p '{"spec":{"template":{"spec":{"containers":[{"name":"neo-cluster","image":"gcr.io/traefiklabs/neo-cluster:latest","imagePullPolicy":"Always"}]}}}}'

reset-organization-image:
	kubectl patch deployment -n neo neo-organization -p '{"spec":{"template":{"spec":{"containers":[{"name":"neo-organization","image":"gcr.io/traefiklabs/neo-organization-service:latest","imagePullPolicy":"Always"}]}}}}'

reset-topology-image:
	kubectl patch deployment -n neo neo-topology -p '{"spec":{"template":{"spec":{"containers":[{"name":"neo-topology","image":"gcr.io/traefiklabs/neo-topology:latest","imagePullPolicy":"Always"}]}}}}'

reset-token-image:
	kubectl patch deployment -n neo neo-token -p '{"spec":{"template":{"spec":{"containers":[{"name":"neo-token","image":"gcr.io/traefiklabs/neo-token:latest","imagePullPolicy":"Always"}]}}}}'

reset-metrics-image:
	kubectl patch deployment -n neo neo-metrics -p '{"spec":{"template":{"spec":{"containers":[{"name":"neo-metrics","image":"gcr.io/traefiklabs/neo-metrics:latest","imagePullPolicy":"Always"}]}}}}'

reset-certificate-image:
	kubectl patch deployment -n neo neo-certificates -p '{"spec":{"template":{"spec":{"containers":[{"name":"neo-certificates","image":"gcr.io/traefiklabs/neo-certificates:latest","imagePullPolicy":"Always"}]}}}}'

clean:
	$(SCRIPT_DIR)/run_local.sh clean

delete:
	@read -p "This will destroy your k3d cluster. Are you sure? (Y/n): " confirm && [ "$${confirm}" != "$${confirm#[Yy]}" ] || exit 1
	k3d cluster delete k3s-default-neo

diagrams:
	$(MAKE) --directory=diagrams
