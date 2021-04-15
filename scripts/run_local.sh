#!/bin/bash
set -o pipefail
set -o errexit

readonly PROJECT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}")" && pwd)/.."
ENV_FILE=${PROJECT_DIR}/neo/.env
[[ -f ${ENV_FILE} ]] && source ${ENV_FILE}

main() {
  check-tools
  setup-k3s

  if [[ $2 == "--adsl" ]]; then
    prepare-docker-images
  fi

  export KUBECONFIG="$(k3d kubeconfig merge k3s-default-neo)"
  kubectl cluster-info

  [[ "x$GCLOUD_EMAIL" == "x" ]] && read -p "Enter gcloud email: " GCLOUD_EMAIL
  [[ "x$GITHUB_TOPOLOGY_REPO" == "x" ]] && read -p "Enter github topology repo name: " GITHUB_TOPOLOGY_REPO
  [[ "x$GITHUB_ORG" == "x" ]] && read -p "Enter github organization: " GITHUB_ORG
  [[ "x$GITHUB_TOKEN" == "x" ]] && read -p "Enter github token: " GITHUB_TOKEN

  kubectl apply -f "$PROJECT_DIR"/neo/manifests/neo/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/neo/manifests/neo-agent/00-namespace.yaml

  # Create CoreDNS configmap and rollout restart
  kubectl apply -f "$PROJECT_DIR"/coredns/00-configmap.yaml
  kubectl rollout restart -n kube-system deploy/coredns

  # Create secrets
  renew-gcr-token
  kubectl delete secret -n neo github || true
  kubectl create secret -n neo generic github --from-literal="token=$GITHUB_TOKEN" --from-literal="org=$GITHUB_ORG"

  # Install Mongo
  echo "Deploying Mongo"
  kubectl apply -f "$PROJECT_DIR"/neo/manifests/mongo/
  kubectl -n mongo wait --for condition=available --timeout=180s deployment/mongodb

  # Install Traefik
  echo "Deploying Traefik."
  kubectl apply -f "$PROJECT_DIR"/neo/manifests/traefik/
  kubectl -n traefik wait --for condition=available --timeout=180s deployment/traefik

  # Install Neo
  echo "Deploying Neo services."

  export GITHUB_TOKEN_B64=$(echo -n "${GITHUB_TOKEN}:" | base64)
  envsubst < "$PROJECT_DIR"/neo/manifests/neo/templates/github-token.yaml > "$PROJECT_DIR"/neo/manifests/neo/02-github-token.yaml

  export GITHUB_ORG
  envsubst < "$PROJECT_DIR"/neo/manifests/neo/templates/neo-cluster.yaml > "$PROJECT_DIR"/neo/manifests/neo/01-neo-cluster.yaml

  kubectl apply -f "$PROJECT_DIR"/neo/manifests/neo/

  # Create token
  kubectl -n neo wait --for condition=available --timeout=180s deployment/neo-topology
  sleep 10 # FIXME: waiting for the readiness implementation for all services

  recreate-topology-token

  # Install Neo Agent
  helm repo add neo https://helm.traefik.io/neo
  helm repo update
  helm upgrade --install neo neo/neo --values="$PROJECT_DIR"/neo/manifests/neo-agent/values.yaml --namespace neo-agent

  # Install Jaeger
  echo "Deploying Jaeger."
  kubectl apply -f "$PROJECT_DIR"/neo/manifests/jaeger/
  kubectl -n jaeger wait --for condition=available --timeout=600s deployment/jaeger

  # Install Ingress-nginx
  echo "Deploying nginx."
  kubectl apply -f "$PROJECT_DIR"/neo/manifests/ingress-nginx/
  kubectl -n ingress-nginx wait --for condition=available --timeout=180s deployment/ingress-nginx-controller

  # Install HaProxy
  echo "Deploying haproxy."
  kubectl apply -f "$PROJECT_DIR"/neo/manifests/ingress-haproxy/
  kubectl -n haproxy-ingress-controller wait --for condition=available --timeout=180s deployment/haproxy-ingress

  # Install whoami
  echo "Deploying whoami."
  kubectl apply -f "$PROJECT_DIR"/neo/manifests/whoami/
  kubectl -n whoami wait --for condition=available --timeout=180s deployment/whoami
}

renew-gcr-token() {
    for namespace in neo neo-agent; do
        set +o errexit
        kubectl delete secret -n $namespace gcr-access-token
        set -o errexit

        kubectl create secret -n $namespace docker-registry gcr-access-token \
                --docker-server=gcr.io \
                --docker-username=oauth2accesstoken \
                --docker-password="$(gcloud auth print-access-token)" \
                --docker-email=${GCLOUD_EMAIL}
    done
}

recreate-topology-token() {
  curl --insecure --silent -X POST -d "{\"id\": \"${GITHUB_TOPOLOGY_REPO}\", \"token\": \"4a585aab-f00e-4548-8528-222ef086bebb\"}" https://platform.docker.localhost/topology/repos
}

check-tools() {
  cmdList="kubectl k3d gcloud helm"
  for cmd in $cmdList; do
    echo -n "checking ${cmd}: "
    command -v "$cmd" >/dev/null 2>&1 || {
      echo "I require $cmd but it's not installed. Aborting"
      exit 1
      }
    done
}

prepare-docker-images() {
  for image in $(find $PROJECT_DIR -type f -name '*.yaml' | xargs grep 'image: ' | awk -F ':' '{ print $3":"$4 }'); do
    docker pull "${image}"
    k3d image import ${image}
  done
}

setup-k3s() {
  if errlog=$(mktemp) && k3d cluster list | grep k3s-default-neo 2> "$errlog" && ! test -s "$errlog"; then
    echo "Starting existing k3s cluster."
    k3d cluster start k3s-default-neo
  else
    echo "Setting up k3s cluster."
    k3d cluster create k3s-default-neo --agents=2 --k3s-server-arg "--no-deploy=traefik" --image="rancher/k3s:v1.20.5-k3s1" --port 80:80@loadbalancer --port 443:443@loadbalancer --port 8000:8000@loadbalancer --port 9000:9000@loadbalancer
  fi

  # Wait until cluster is ready
  echo "Waiting for k3s cluster state: RUNNING."
  until k3d kubeconfig get k3s-default-neo >/dev/null 2>&1
  do
    sleep 1
    echo -n .
  done
  echo
  echo "Kubernetes cluster is ready."
}

clean() {
  # Uninstall whoami
  echo "Undeploying whoami."
  kubectl delete -f "$PROJECT_DIR"/neo/manifests/whoami/

  # Uninstall Ingress-nginx-inc
  echo "Undeploying haproxy."
  kubectl delete -f "$PROJECT_DIR"/neo/manifests/ingress-haproxy/

  # Uninstall Ingress-nginx
  echo "Undeploying nginx."
  kubectl delete -f "$PROJECT_DIR"/neo/manifests/ingress-nginx/

  # Uninstall Jaeger
  echo "Undeploying Jaeger."
  kubectl delete -f "$PROJECT_DIR"/neo/manifests/jaeger/

  # Uninstall Neo Agent
  helm uninstall neo --namespace neo-agent

  # Uninstall Neo
  echo "Undeploying Neo services."
  kubectl delete -f "$PROJECT_DIR"/neo/manifests/neo/

  # Uninstall Traefik
  echo "Undeploying Traefik."
  kubectl delete -f "$PROJECT_DIR"/neo/manifests/traefik/

  # Uninstall Mongo
  echo "Undeploying Mongo"
  kubectl delete -f "$PROJECT_DIR"/neo/manifests/mongo/

  # Delete ClusterRole, secrets and namespaces
  kubectl delete -f "$PROJECT_DIR"/neo/manifests/neo-agent/00-namespace.yaml
}

cmd=$1

case $cmd in
    renew-gcr-token)
        renew-gcr-token
    ;;
    recreate-topology-token)
        recreate-topology-token
    ;;
    run)
        main "$@"
    ;;
    clean)
        clean
    ;;
    *)
        echo "Commands available: recreate-topology-token, renew-gcr-token, run, clean"
        exit 1
    ;;
esac
