#!/bin/bash
set -o pipefail
set -o errexit

readonly PROJECT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}")" && pwd)/.."
ENV_FILE=${PROJECT_DIR}/hub/.env
[[ -f "${ENV_FILE}" ]] && source "${ENV_FILE}"
WORKSPACE_ID="${WORKSPACE_ID:-6311c90bfce04bd29e473a20}"

main() {
  check-tools
  setup-k3s

  if [[ "$AUTO_UPDATE_HOSTS" == "true" ]]; then
    update-local-hosts
  fi

  if [[ $2 == "--adsl" ]]; then
    prepare-docker-images
  fi

  export KUBECONFIG="$(k3d kubeconfig merge k3s-default-hub)"
  kubectl cluster-info

  TIMEOUT="${TIMEOUT:-180s}"

  [[ "x$GCLOUD_EMAIL" == "x" ]] && read -p "Enter gcloud email: " GCLOUD_EMAIL
  [[ "x$AWS_CLIENT_ID" == "x" ]] && read -p "Enter aws client id: " AWS_CLIENT_ID
  [[ "x$AWS_CLIENT_SECRET" == "x" ]] && read -p "Enter aws client secret: " AWS_CLIENT_SECRET
  [[ "x$HUB_USERNAME" == "x" ]] && read -p "Enter your hub username: " HUB_USERNAME
  [[ "x$HUB_PASSWORD" == "x" ]] && read -p "Enter your hub password: " HUB_PASSWORD

  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/broker/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub-agent/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/aws-secret-operator/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/pop/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/git/00-namespace.yaml

  apply-coredns-conf

  # Create secrets
  renew-gcr-token
  kubectl delete secret -n aws-secret-operator aws-secret || true
  kubectl create secret -n aws-secret-operator generic aws-secret --from-literal="api_key=$AWS_CLIENT_ID" --from-literal="api_secret_key=$AWS_CLIENT_SECRET"

  # Install AWS Secret Operator
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/aws-secret-operator/

  # Create CronJob for auth0 service token renewal.
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/auth0/auth0-service-token.yaml
  kubectl create job -n hub auth0-service-token --from cronjobs/auth0-service-token || true

  # Install Mongo
  echo "Deploying Mongo"
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/mongo/
  kubectl -n mongo wait --for condition=available --timeout="${TIMEOUT}" deployment/mongodb

  # Install Traefik
  echo "Deploying Traefik."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/traefik/
  kubectl -n traefik wait --for condition=available --timeout="${TIMEOUT}" deployment/traefik

  # Install Pebble
  echo "Deploying Pebble."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/pebble/
  kubectl -n pebble wait --for condition=available --timeout="${TIMEOUT}" deployment/pebble

  # Populate Mongo
  for dbcol in workspaces_workspaces users_users users_tos ; do
    db=$(echo $dbcol | awk -F '_' '{print $1}')
    col=$(echo $dbcol | awk -F '_' '{print $2}')
    kubectl cp "$PROJECT_DIR"/hub/documents/${dbcol}.json -n mongo $(kubectl get pods -n mongo -l app=mongodb --output=jsonpath={.items..metadata.name}):/tmp/${dbcol}.json
    kubectl exec -it -n mongo $(kubectl get pods -n mongo -l app=mongodb --output=jsonpath={.items..metadata.name}) -- bash -c "mongoimport --db ${db} --collection ${col} --file /tmp/${dbcol}.json --username root --password admin  --authenticationDatabase admin"
  done

  # Install Jaeger
  echo "Deploying Jaeger."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/jaeger/
  kubectl -n jaeger wait --for condition=available --timeout="${TIMEOUT}" deployment/jaeger

  # Install Git
  echo "Deploying Git."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/git/
  kubectl -n git wait --for condition=available --timeout="${TIMEOUT}" deployment/hub-git

  # Install Hub
  echo "Deploying Hub services."

  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/secrets/
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/

  # Create token
  kubectl rollout status deploy --timeout="${TIMEOUT}" -n hub hub-topology
  kubectl rollout status deploy --timeout="${TIMEOUT}" -n hub hub-cluster
  kubectl rollout status deploy --timeout="${TIMEOUT}" -n hub hub-token
  kubectl rollout status deploy --timeout="${TIMEOUT}" -n hub hub-offer
  sleep 2

  # Install Broker
  echo "Deploying Broker service."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/broker/
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/broker/secrets

  renew-jwt
  sleep 5

  ## Freemium US
  kubectl run --timeout="${TIMEOUT}" --command=true -it --rm --restart=Never --image=gcr.io/traefiklabs/hub-offer:latest \
  --image-pull-policy=IfNotPresent --namespace=hub \
  --overrides='{"apiVersion": "v1", "spec": {"imagePullSecrets": [{"name": "gcr-access-token"}]}}' -- hub-offer /hub-offer create-offer \
  --mongodb-uri="mongodb://root:admin@mongodb.mongo.svc.cluster.local:27017/offers?authSource=admin" \
  --log-level="debug" \
  --offer-name="freemium" \
  --offer-zone-name="default" \
  --offer-priceid="price_1J05aJEfHpKKvvELtLRaL1xl" \
  --offer-config-metrics-interval="1m" \
  --offer-config-metrics-tables="1m" \
  --offer-config-metrics-tables="10m" \
  --offer-config-metrics-tables="1h" \
  --offer-quotas-clusters="2" \
  --offer-config-access-control-max-secured-routes="3" \
  --offer-quotas-users="2" \
  --offer-quotas-domains="10" \
  --offer-quotas-gslb-bandwidth="1000000000" \
  --offer-quotas-alert-triggers="5" \
  --offer-quotas-alert-history="10" \
  --offer-quotas-edge-ingresses="10" \
  --offer-config-gslb-http-healthcheck-min-interval-seconds=60 \
  --offer-config-gslb-http-healthcheck-min-threshold-editable="false" \
  --offer-features="blue-green" --offer-features="canary" --offer-features="active-active" --offer-features="active-passive" || true

  ## Premium US
  kubectl run --timeout="${TIMEOUT}" --command=true -it --rm --restart=Never --image=gcr.io/traefiklabs/hub-offer:latest \
  --image-pull-policy=IfNotPresent --namespace=hub \
  --overrides='{"apiVersion": "v1", "spec": {"imagePullSecrets": [{"name": "gcr-access-token"}]}}' -- hub-offer /hub-offer create-offer \
  --mongodb-uri="mongodb://root:admin@mongodb.mongo.svc.cluster.local:27017/offers?authSource=admin" \
  --log-level="debug" \
  --offer-name="premium" \
  --offer-zone-name="default" \
  --offer-priceid="price_1J05awEfHpKKvvELb9boaOxw" \
  --offer-config-metrics-interval="1m" \
  --offer-config-metrics-tables="1m" \
  --offer-config-metrics-tables="10m" \
  --offer-config-metrics-tables="1h" \
  --offer-config-metrics-tables="1d" \
  --offer-quotas-clusters="5" \
  --offer-config-access-control-max-secured-routes="50" \
  --offer-quotas-users="20" \
  --offer-quotas-domains="100" \
  --offer-quotas-gslb-bandwidth="50000000000" \
  --offer-quotas-alert-triggers="100" \
  --offer-quotas-alert-history="200" \
  --offer-quotas-edge-ingresses="10" \
  --offer-config-gslb-http-healthcheck-min-interval-seconds=15 \
  --offer-config-gslb-http-healthcheck-min-threshold-editable="true" \
  --offer-features="team-management" --offer-features="geo-steering" \
  --offer-features="blue-green" --offer-features="canary" --offer-features="active-active" --offer-features="active-passive" || true

  ## Freemium EU
  kubectl run --timeout="${TIMEOUT}" --command=true -it --rm --restart=Never --image=gcr.io/traefiklabs/hub-offer:latest \
  --image-pull-policy=IfNotPresent --namespace=hub \
  --overrides='{"apiVersion": "v1", "spec": {"imagePullSecrets": [{"name": "gcr-access-token"}]}}' -- hub-offer /hub-offer create-offer \
  --mongodb-uri="mongodb://root:admin@mongodb.mongo.svc.cluster.local:27017/offers?authSource=admin" \
  --log-level="debug" \
  --offer-name="freemium" \
  --offer-zone-name="eu" \
  --offer-priceid="price_1J05XOAYmbimY05BcwhLKPgB" \
  --offer-config-metrics-interval="1m" \
  --offer-config-metrics-tables="1m" \
  --offer-config-metrics-tables="10m" \
  --offer-config-metrics-tables="1h" \
  --offer-quotas-clusters="2" \
  --offer-config-access-control-max-secured-routes="3" \
  --offer-quotas-users="2" \
  --offer-quotas-domains="10" \
  --offer-quotas-gslb-bandwidth="50000000000" \
  --offer-quotas-alert-triggers="5" \
  --offer-quotas-alert-history="10" \
  --offer-quotas-edge-ingresses="10" \
  --offer-config-gslb-http-healthcheck-min-interval-seconds=60 \
  --offer-config-gslb-http-healthcheck-min-threshold-editable="false" \
  --offer-features="blue-green" --offer-features="canary" --offer-features="active-active" --offer-features="active-passive" || true

  ## Premium EU
  kubectl run --timeout="${TIMEOUT}" --command=true -it --rm --restart=Never --image=gcr.io/traefiklabs/hub-offer:latest \
  --image-pull-policy=IfNotPresent --namespace=hub \
  --overrides='{"apiVersion": "v1", "spec": {"imagePullSecrets": [{"name": "gcr-access-token"}]}}' -- hub-offer /hub-offer create-offer \
  --mongodb-uri="mongodb://root:admin@mongodb.mongo.svc.cluster.local:27017/offers?authSource=admin" \
  --log-level="debug" \
  --offer-name="premium" \
  --offer-zone-name="eu" \
  --offer-priceid="price_1J05YJAYmbimY05BpjcSne7V" \
  --offer-config-metrics-interval="1m" \
  --offer-config-metrics-tables="1m" \
  --offer-config-metrics-tables="10m" \
  --offer-config-metrics-tables="1h" \
  --offer-config-metrics-tables="1d" \
  --offer-quotas-clusters="5" \
  --offer-config-access-control-max-secured-routes="50" \
  --offer-quotas-users="20" \
  --offer-quotas-domains="100" \
  --offer-quotas-gslb-bandwidth="1000000000" \
  --offer-quotas-alert-triggers="100" \
  --offer-quotas-alert-history="200" \
  --offer-quotas-edge-ingresses="10" \
  --offer-config-gslb-http-healthcheck-min-interval-seconds=15 \
  --offer-config-gslb-http-healthcheck-min-threshold-editable="true" \
  --offer-features="team-management" --offer-features="geo-steering" \
  --offer-features="blue-green" --offer-features="canary" --offer-features="active-active" --offer-features="active-passive" || true

  # Create subscription
  curl --silent --location --request POST 'http://platform.docker.localhost/offer/internal/subscriptions' \
  --header 'Content-Type: application/json' \
  --data-raw "{\"countryCode\": \"FR\", \"workspaceId\": \"${WORKSPACE_ID}\", \"priceId\": \"price_1J05YJAYmbimY05BpjcSne7V\"}"

  # Create topology
    curl --silent --location --request POST 'http://platform.docker.localhost/topology/internal/workspaces' \
    --header 'Content-Type: application/json' \
    --data-raw "{\"id\": \"${WORKSPACE_ID}\"}"

  export TOKEN_CLUSTER=$(curl --silent --location --request POST 'http://platform.docker.localhost/cluster/external/clusters' \
  --header "Authorization: Bearer ${JWT_EXTERNAL}" \
  --header 'Content-Type: application/json' \
  --data-raw "{\"name\": \"cluster\"}" | jq -r '.token' | tr -d '\n')

  if [ $TOKEN_CLUSTER != "null" ]; then
    envsubst < "$PROJECT_DIR"/hub/manifests/hub-agent/templates/values.yaml > "$PROJECT_DIR"/hub/manifests/hub-agent/01-values.yaml
  fi

  # Install Hub Agent
  helm repo add traefik-hub https://helm.traefik.io/hub
  helm repo update
  helm upgrade --install hub-agent traefik-hub/hub-agent --values="$PROJECT_DIR"/hub/manifests/hub-agent/01-values.yaml --namespace hub-agent

  # Patch Hub agent to expose debugging port
  kubectl patch svc -n hub-agent hub-agent-controller -p '{"spec":{"ports":[{"name":"hub-agent-debug","port":40000}]}}'
  kubectl patch svc -n hub-agent hub-agent-auth-server -p '{"spec":{"ports":[{"name":"hub-agent-debug","port":40000}]}}'

  # Wait for Hub agent to start
  kubectl -n hub-agent wait --for condition=available --timeout="${TIMEOUT}" deployment/hub-agent-controller
  kubectl -n hub-agent wait --for condition=available --timeout="${TIMEOUT}" deployment/hub-agent-auth-server

  # Install PoP
  echo "Deploying PoP services."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/pop/secrets
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/pop

  # Install Ingress-nginx
  echo "Deploying nginx."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/ingress-nginx/
  kubectl -n ingress-nginx wait --for condition=available --timeout="${TIMEOUT}" deployment/ingress-nginx-controller

  # Install HaProxy
  echo "Deploying haproxy."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/ingress-haproxy/
  kubectl -n haproxy-ingress-controller wait --for condition=available --timeout="${TIMEOUT}" deployment/haproxy-ingress

  # Install whoami
  echo "Deploying whoami."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/whoami/
  kubectl -n whoami wait --for condition=available --timeout="${TIMEOUT}" deployment/whoami

  # Install monitoring
  echo "Deploying monitoring."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/monitoring/00-namespace.yaml
  kubectl delete configmap -n monitoring grafana-dashboard || true
  kubectl create configmap -n monitoring grafana-dashboard --from-file="$PROJECT_DIR"/hub/manifests/monitoring/dashboards/
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/monitoring/
}

update-local-hosts() {
  HUB_DOMAIN=${HUB_DOMAIN:-docker.localhost}
  # TODO - could fetch real LB address (k3d)
  lb_address="127.0.0.1"
  hub_hosts="platform.$HUB_DOMAIN
    webapp.$HUB_DOMAIN
    jaeger-ui.$HUB_DOMAIN
    prometheus.$HUB_DOMAIN
    grafana.$HUB_DOMAIN"

  echo "Updating /etc/hosts"
  for hostname in $hub_hosts; do
    sedi "/[[:space:]]\+$(echo $hostname | sed 's/\./\\\./g')/d" /etc/hosts
    echo "$lb_address $hostname" | sudo tee -a /etc/hosts
  done
}

# Because MacOS exists
sedi() {
  sed --version >/dev/null 2>&1 && sudo sed -i -- "$@" || sudo sed -i "" "$@"
}

apply-coredns-conf() {
    # Create CoreDNS configmap and rollout restart
    kubectl apply -f "$PROJECT_DIR"/coredns/00-configmap.yaml
    echo "Waiting coredns availability";
    until kubectl wait deployment/coredns -n kube-system --for=condition=available; do
      sleep 1;
    done
    kubectl rollout restart -n kube-system deploy/coredns
}

renew-gcr-token() {
    for namespace in hub hub-agent pop git broker; do
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

  JWT_EXTERNAL=$(curl --silent --location --request POST 'https://traefiklabs-hub-preview.eu.auth0.com/oauth/token' \
    --header 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode 'grant_type=password' \
    --data-urlencode "username=${HUB_USERNAME}" \
    --data-urlencode "password=${HUB_PASSWORD}" \
    --data-urlencode 'audience=https://clients.hub.traefik.io/' \
    --data-urlencode "client_id=${JWT_CLIENT_ID}" \
    --data-urlencode "client_secret=${JWT_CLIENT_SECRET}" \
    --data-urlencode 'scope=openid' \
    --data-urlencode 'realm=Username-Password-Authentication' \
    --data-urlencode "workspaceId=${WORKSPACE_ID}" \
      | jq -r '.access_token' | tr -d '\n')

  echo "${JWT_EXTERNAL}"
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
    k3d cluster create k3s-default-hub --agents=2 --k3s-arg "--no-deploy=traefik@servers:*" --image="rancher/k3s:v1.21.5-k3s1" --port 80:80@loadbalancer --port 443:443@loadbalancer --port 8000:8000@loadbalancer --port 8443:8443@loadbalancer --port 9000:9000@loadbalancer --port 9443:9443@loadbalancer --port 9090:9090@loadbalancer
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
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/whoami/ 2> /dev/null || true

  # Uninstall Ingress-nginx-inc
  echo "Undeploying haproxy."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/ingress-haproxy/ 2> /dev/null || true

  # Uninstall Ingress-nginx
  echo "Undeploying nginx."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/ingress-nginx/ 2> /dev/null || true

  # Uninstall Jaeger
  echo "Undeploying Jaeger."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/jaeger/ 2> /dev/null || true

  # Uninstall Hub Agent
  helm uninstall hub-agent --namespace hub-agent 2> /dev/null || true

  # Uninstall Hub
  echo "Undeploying Hub services."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/hub/ 2> /dev/null || true

  # Uninstall Broker
  echo "Undeploying Broker service"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/broker/secrets 2> /dev/null || true
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/broker 2> /dev/null || true

  # uninstall PoP
  echo "Undeploying PoP services."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/pop/secrets 2> /dev/null || true
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/pop 2> /dev/null || true

  # Uninstall Traefik
  echo "Undeploying Traefik."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/traefik/ 2> /dev/null || true

  # Uninstall Mongo
  echo "Undeploying Mongo"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/mongo/ 2> /dev/null || true

  # Delete webhook
  kubectl delete mutatingwebhookconfigurations.admissionregistration.k8s.io hub 2> /dev/null || true

  # Delete ClusterRole, secrets and namespaces
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/hub-agent/00-namespace.yaml 2> /dev/null || true
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/aws-secret-operator/00-namespace.yaml 2> /dev/null || true

  # Uninstall Monitoring
  echo "Undeploying Monitoring"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/monitoring/00-namespace.yaml 2> /dev/null || true

  # Uninstall Pebble
  echo "Undeploying Pebble"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/pebble/ 2> /dev/null || true
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
