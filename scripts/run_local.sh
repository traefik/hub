#!/bin/bash
set -x
set -o pipefail
set -o errexit

readonly PROJECT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}")" && pwd)/.."
ENV_FILE=${PROJECT_DIR}/hub/.env
[[ -f ${ENV_FILE} ]] && source ${ENV_FILE}

main() {
  check-tools
  setup-k3s

  if [[ $2 == "--adsl" ]]; then
    prepare-docker-images
  fi

  export KUBECONFIG="$(k3d kubeconfig merge k3s-default-hub)"
  kubectl cluster-info

  [[ "x$GCLOUD_EMAIL" == "x" ]] && read -p "Enter gcloud email: " GCLOUD_EMAIL
  [[ "x$GITHUB_ORG" == "x" ]] && read -p "Enter github organization: " GITHUB_ORG
  [[ "x$GITHUB_TOKEN" == "x" ]] && read -p "Enter github token: " GITHUB_TOKEN
  [[ "x$AWS_CLIENT_ID" == "x" ]] && read -p "Enter aws client id: " AWS_CLIENT_ID
  [[ "x$AWS_CLIENT_SECRET" == "x" ]] && read -p "Enter aws client secret: " AWS_CLIENT_SECRET
  [[ "x$HUB_USERNAME" == "x" ]] && read -p "Enter your hub username: " HUB_USERNAME
  [[ "x$HUB_PASSWORD" == "x" ]] && read -p "Enter your hub password: " HUB_PASSWORD

  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub-agent/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/aws-secret-operator/00-namespace.yaml

  apply-coredns-conf

  # Create secrets
  renew-gcr-token
  kubectl delete secret -n hub github || true
  kubectl delete secret -n aws-secret-operator aws-secret || true
  kubectl create secret -n hub generic github --from-literal="token=$GITHUB_TOKEN" --from-literal="org=$GITHUB_ORG"
  kubectl create secret -n aws-secret-operator generic aws-secret --from-literal="api_key=$AWS_CLIENT_ID" --from-literal="api_secret_key=$AWS_CLIENT_SECRET"

  # Install AWS Secret Operator
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/aws-secret-operator/

  # Install Mongo
  echo "Deploying Mongo"
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/mongo/
  kubectl -n mongo wait --for condition=available --timeout=180s deployment/mongodb

  # Install Traefik
  echo "Deploying Traefik."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/traefik/
  kubectl -n traefik wait --for condition=available --timeout=180s deployment/traefik

  # Install Pebble
  echo "Deploying Pebble."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/pebble/
  kubectl -n pebble wait --for condition=available --timeout=180s deployment/pebble

  # Populate Mongo
  kubectl cp "$PROJECT_DIR"/hub/documents/workspace.json -n mongo $(kubectl get pods -n mongo -l app=mongodb --output=jsonpath={.items..metadata.name}):/tmp/workspace.json
  kubectl exec -it -n mongo $(kubectl get pods -n mongo -l app=mongodb --output=jsonpath={.items..metadata.name}) -- bash -c "mongoimport --db workspaces --collection workspaces --file /tmp/workspace.json --username root --password admin  --authenticationDatabase admin"

  # Install Hub
  echo "Deploying Hub services."

  export GITHUB_TOKEN_B64=$(echo -n "${GITHUB_TOKEN}:" | base64)
  envsubst < "$PROJECT_DIR"/hub/manifests/hub/templates/github-token.yaml > "$PROJECT_DIR"/hub/manifests/hub/01-github-token.yaml

  export GITHUB_ORG
  envsubst < "$PROJECT_DIR"/hub/manifests/hub/templates/hub-cluster.yaml > "$PROJECT_DIR"/hub/manifests/hub/01-hub-cluster.yaml

  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/secrets/
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/

  # Create token
  kubectl rollout status deploy -n hub hub-topology
  kubectl rollout status deploy -n hub hub-cluster
  kubectl rollout status deploy -n hub hub-token
  sleep 2

  renew-jwt
  sleep 5

  CLUSTER_NAME=$(date +%s | sha256sum | base64 | head -c 32 ; echo)

  ### THIS IS TEMPORARY AND SHOULD GO AWAY SOON™ ###
   curl --location --request POST 'http://platform.docker.localhost/cluster/internal/quotas' \
  --header 'Content-Type: application/json' \
  --data-raw '{
      "name": "clusters",
      "scope": "607997e8406c62aace2d493d",
      "max": 10
  }'
  ### ----------------------------------------- ###

  export TOKEN_CLUSTER=$(curl --silent --location --request POST 'http://platform.docker.localhost/cluster/external/clusters' \
  --header "Authorization: Bearer ${JWT_EXTERNAL}" \
  --header 'Content-Type: application/json' \
  --data-raw "{\"name\": \"${CLUSTER_NAME}\"}" | jq -r '.token' | tr -d '\n')

  envsubst < "$PROJECT_DIR"/hub/manifests/hub-agent/templates/values.yaml > "$PROJECT_DIR"/hub/manifests/hub-agent/01-values.yaml

  # Install Hub Agent
  helm repo add hub https://helm.traefik.io/hub
  helm repo update
  helm upgrade --install hub hub/hub --values="$PROJECT_DIR"/hub/manifests/hub-agent/01-values.yaml --namespace hub-agent

  # Patch Hub agent to expose debugging port
  kubectl patch svc -n hub-agent hub-agent-controller -p '{"spec":{"ports":[{"name":"hub-agent-debug","port":40000}]}}'
  kubectl patch svc -n hub-agent hub-agent-auth-server -p '{"spec":{"ports":[{"name":"hub-agent-debug","port":40000}]}}'

  # Wait for Hub agent to start
  kubectl -n hub-agent wait --for condition=available --timeout=180s deployment/hub-agent-controller
  kubectl -n hub-agent wait --for condition=available --timeout=180s deployment/hub-agent-auth-server

  # Install PoP
  echo "Deploying PoP services."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/pop
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/pop/secrets

  # Install Jaeger
  echo "Deploying Jaeger."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/jaeger/
  kubectl -n jaeger wait --for condition=available --timeout=600s deployment/jaeger

  # Install Ingress-nginx
  echo "Deploying nginx."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/ingress-nginx/
  kubectl -n ingress-nginx wait --for condition=available --timeout=180s deployment/ingress-nginx-controller

  # Install HaProxy
  echo "Deploying haproxy."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/ingress-haproxy/
  kubectl -n haproxy-ingress-controller wait --for condition=available --timeout=180s deployment/haproxy-ingress

  # Install whoami
  echo "Deploying whoami."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/whoami/
  kubectl -n whoami wait --for condition=available --timeout=180s deployment/whoami

  # Install monitoring
  echo "Deploying monitoring."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/monitoring/00-namespace.yaml
  kubectl delete configmap -n monitoring grafana-dashboard || true
  kubectl create configmap -n monitoring grafana-dashboard --from-file="$PROJECT_DIR"/hub/manifests/monitoring/dashboards/
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/monitoring/
}

apply-coredns-conf() {
    # Create CoreDNS configmap and rollout restart
    kubectl apply -f "$PROJECT_DIR"/coredns/00-configmap.yaml
    kubectl rollout restart -n kube-system deploy/coredns
}

renew-gcr-token() {
    for namespace in hub hub-agent; do
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

renew-auth0-admin-token() {
  kubectl delete job -n hub auth0-admin-token-renew
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/01-auth0-admin-token-renew.yaml
}

renew-jwt() {
  JWT_CLIENT_ID=$(kubectl get secret hub-secret -n hub -o json | jq -r '.data["auth0-client-id"]' | tr -d '\n' | base64 -d)
  JWT_CLIENT_SECRET=$(kubectl get secret hub-secret -n hub -o json | jq -r '.data["auth0-client-secret"]' | tr -d '\n' | base64 -d)

  JWT_EXTERNAL=$(curl --silent --location --request POST 'https://traefiklabs-neo-dev.eu.auth0.com/oauth/token' \
    --header 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode 'grant_type=password' \
    --data-urlencode "username=${HUB_USERNAME}" \
    --data-urlencode "password=${HUB_PASSWORD}" \
    --data-urlencode 'audience=https://clients.hub.traefik.io/' \
    --data-urlencode "client_id=${JWT_CLIENT_ID}" \
    --data-urlencode "client_secret=${JWT_CLIENT_SECRET}" \
    --data-urlencode 'scope=openid' \
    --data-urlencode 'realm=Username-Password-Authentication' \
    --data-urlencode 'workspaceId=607997e8406c62aace2d493d' | jq -r '.access_token' | tr -d '\n')

  echo $JWT_EXTERNAL
}

check-tools() {
  cmdList="kubectl k3d gcloud helm jq"
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
    k3d image import ${image} -c k3s-default-hub
  done
}

setup-k3s() {
  if errlog=$(mktemp) && k3d cluster list | grep k3s-default-hub 2> "$errlog" && ! test -s "$errlog"; then
    echo "Starting existing k3s cluster."
    k3d cluster start k3s-default-hub
  else
    echo "Setting up k3s cluster."
    k3d cluster create k3s-default-hub --agents=2 --k3s-server-arg "--no-deploy=traefik" --image="rancher/k3s:v1.20.5-k3s1" --port 80:80@loadbalancer --port 443:443@loadbalancer --port 8000:8000@loadbalancer --port 8443:8443@loadbalancer --port 9000:9000@loadbalancer --port 9443:9443@loadbalancer
  fi

  # Wait until cluster is ready
  echo "Waiting for k3s cluster state: RUNNING."
  until k3d kubeconfig get k3s-default-hub >/dev/null 2>&1
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
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/whoami/

  # Uninstall Ingress-nginx-inc
  echo "Undeploying haproxy."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/ingress-haproxy/

  # Uninstall Ingress-nginx
  echo "Undeploying nginx."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/ingress-nginx/

  # Uninstall Jaeger
  echo "Undeploying Jaeger."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/jaeger/

  # Uninstall Hub Agent
  helm uninstall hub --namespace hub-agent

  # Uninstall Hub
  echo "Undeploying Hub services."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/hub/

  # Uninstall Traefik
  echo "Undeploying Traefik."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/traefik/

  # Uninstall Mongo
  echo "Undeploying Mongo"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/mongo/

  # Delete webhook
  kubectl delete mutatingwebhookconfigurations.admissionregistration.k8s.io hub || true

  # Delete ClusterRole, secrets and namespaces
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/hub-agent/00-namespace.yaml
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/aws-secret-operator/00-namespace.yaml

  # Uninstall Monitoring
  echo "Undeploying Monitoring"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/monitoring/00-namespace.yaml

  # Uninstall Pebble
  echo "Undeploying Pebble"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/pebble/
}

cmd=$1

case $cmd in
    apply-coredns-conf)
        apply-coredns-conf
    ;;
    renew-gcr-token)
        renew-gcr-token
    ;;
    renew-jwt)
        renew-jwt
    ;;
    renew-auth0-admin-token)
        renew-auth0-admin-token
    ;;
    run)
        main "$@"
    ;;
    clean)
        clean
    ;;
    *)
        echo "Commands available: apply-coredns-conf, renew-auth0-admin-token, renew-gcr-token, renew-jwt, run, clean"
        exit 1
    ;;
esac
