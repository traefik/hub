

all: tutorials

tutorials: getting-started advanced-use-cases

prerequisites:
ifndef TRAEFIK_HUB_USE_CURRENT_KUBECTX
	kind create cluster --config=tutorials/0-prerequisites/kind/config.yaml
	kubectl cluster-info
	kubectl wait --for=condition=ready nodes traefik-hub-control-plane
	kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.11/config/manifests/metallb-native.yaml
	kubectl wait --namespace metallb-system --for=condition=ready pod --selector=app=metallb --timeout=90s
	kubectl apply -f tutorials/0-prerequisites/kind/metallb-config.yaml
	export TRAEFIK_HUB_USE_CURRENT_KUBECTX=true
endif

check-hub-token:
ifndef TRAEFIK_HUB_TOKEN
	$(error TRAEFIK_HUB_TOKEN is undefined)
endif

install-hub: prerequisites check-hub-token
ifndef TRAEFIK_HUB_DEPLOYED
	kubectl create namespace traefik-hub
	@kubectl create secret generic hub-agent-token --namespace traefik-hub --from-literal=token=$(TRAEFIK_HUB_TOKEN)
	helm repo add --force-update traefik https://traefik.github.io/charts
	helm install traefik-hub $(TRAEFIK_HUB_ARGS) -n traefik-hub -f tutorials/0-prerequisites/kind/values.yaml --wait traefik/traefik-hub
	export TRAEFIK_HUB_DEPLOYED=true
endif

getting-started: install-hub
	kubectl apply -f src/manifests/public-app.yaml
	kubectl apply -f tutorials/1-getting-started/manifests/public-api.yaml
	kubectl apply -f tutorials/1-getting-started/manifests/api-access.yaml
	kubectl apply -f tutorials/1-getting-started/manifests/api-gateway.yaml

.PHONY: clean

clean-hub:
	helm uninstall traefik-hub -n traefik-hub
	kubectl delete namespace traefik-hub
	kubectl delete namespace apps

clean:
	kind delete cluster --name traefik-hub
	unset TRAEFIK_HUB_USE_CURRENT_KUBECTX
	unset TRAEFIK_HUB_DEPLOYED
