.PHONY: renew-gcr-tokens renew-auth0-admin-token recreate-topology-token run run-adsl clean delete

SCRIPT_DIR ?= $(CURDIR)/scripts

renew-gcr-token:
	$(SCRIPT_DIR)/run_local.sh renew-gcr-token

renew-auth0-admin-token:
	$(SCRIPT_DIR)/run_local.sh renew-auth0-admin-token

recreate-topology-token:
	$(SCRIPT_DIR)/run_local.sh recreate-topology-token

run:
	$(SCRIPT_DIR)/run_local.sh run

run-adsl:
	$(SCRIPT_DIR)/run_local.sh run --adsl

clean:
	$(SCRIPT_DIR)/run_local.sh clean

delete:
	read -p "This will destroy your k3d cluster. Are you sure ? (Y/n): " confirm && [[ $$confirm == [yY] ]] || exit 1
	k3d cluster delete k3s-default-neo
