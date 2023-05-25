#!/bin/bash

main() {
  checkTools
  setupK3S

  if [[ "$AUTO_UPDATE_HOSTS" == "true" ]]; then
    updateLocalHosts
  fi

  if [[ $2 == "--adsl" ]]; then
    prepareDockerImages
  fi

  export KUBECONFIG="$(k3d kubeconfig merge k3s-default-hub)"
  kubectl cluster-info

  applyCoreDNSConf
  createNamespaces
  createSecrets

  renewGCRToken

  # Base
  installMongo
  installHydra
  installTraefik
  installPebble
  installNats
  installHub

  renewJWT

  initializeWorkspace
  installTraefikHub

  # Optional
  if ${INSTALL_POP}; then
    installPoP
  fi

  if ${INSTALL_BROKER}; then
    installBroker
  fi

  if ${INSTALL_JAEGER}; then
    installJaeger
  fi

  if ${INSTALL_MONITORING}; then
    installMonitoring
  fi

  if ${INSTALL_WHOAMI}; then
      installWhoami
  fi

  if ${INSTALL_PETSTORE}; then
      installPetstore
  fi
}

updateLocalHosts() {
  HUB_DOMAIN=${HUB_DOMAIN:-docker.localhost}
  # TODO - could fetch real LB address (k3d)
  lb_address="127.0.0.1"
  hub_hosts="platform.$HUB_DOMAIN
    webapp.$HUB_DOMAIN
    sso.portal.$HUB_DOMAIN
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

applyCoreDNSConf() {
    # Create CoreDNS configmap and rollout restart
    kubectl apply -f "$PROJECT_DIR"/coredns/00-configmap.yaml
    echo "Waiting coredns availability";
    until kubectl wait deployment/coredns -n kube-system --for=condition=available; do
      sleep 1;
    done
    kubectl rollout restart -n kube-system deploy/coredns
}

renewGCRToken() {
    for namespace in hub pop broker; do
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

renewAuth0AdminToken() {
  kubectl delete job -n hub auth0-admin-token-renew || true
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/01-auth0-admin-token-renew.yaml
}

renewJWT() {
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

checkTools() {
  cmdList="kubectl k3d gcloud helm jq"
  for cmd in $cmdList; do
    echo -n "checking ${cmd}: "
    command -v "$cmd" >/dev/null 2>&1 || {
      echo "I require $cmd but it's not installed. Aborting"
      exit 1
      }
    done
}

prepareDockerImages() {
  images=$(find "${PROJECT_DIR}" -type f -name '*.yaml' | xargs grep 'image: ' | tr -d \" | awk -F '@' '{ print $1 }' | awk -F ':' '{ print $3":"$4 }' | sort | uniq)

  images=$(echo "${images}" | grep -v "hub-ui")

  echo "${images}" | xargs -P4 -n1 docker pull
  k3d image import -c k3s-default-hub ${images}
}

setupK3S() {
  if errlog=$(mktemp) && k3d cluster list | grep k3s-default-hub 2> "$errlog" && ! test -s "$errlog"; then
    echo "Starting existing k3s cluster."
    k3d cluster start k3s-default-hub
  else
    echo "Setting up k3s cluster."
    k3d cluster create k3s-default-hub --agents=2 \
      --k3s-arg "--no-deploy=traefik@servers:*" \
      --image="$K3S_IMAGE" \
      --port 80:80@loadbalancer \
      --port 443:443@loadbalancer \
      --port 4222:4222@loadbalancer \
      --port 8000:8000@loadbalancer \
      --port 8443:8443@loadbalancer \
      --port 9000:9000@loadbalancer \
      --port 9443:9443@loadbalancer \
      --port 9090:9090@loadbalancer
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

  # Uninstall Jaeger
  echo "Undeploying Jaeger."
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/jaeger/ 2> /dev/null || true

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

  # Uninstall Traefik Hub
  echo "Undeploying Traefik Hub"
  kubectl delete -f "https://hub.traefik.io/install/crd" 2> /dev/null || true

  # Uninstall Mongo
  echo "Undeploying Mongo"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/mongo/ 2> /dev/null || true

  # Uninstall Hydra
  echo "Undeploying Hydra"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/hydra/ 2> /dev/null || true

  # Uninstall Nats
  echo "Undeploying Nats"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/nats/ 2> /dev/null || true

  # Delete webhook
  kubectl delete mutatingwebhookconfigurations.admissionregistration.k8s.io hub 2> /dev/null || true

  # Delete ClusterRole, secrets and namespaces
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/aws-secret-operator/00-namespace.yaml 2> /dev/null || true

  # Uninstall Monitoring
  echo "Undeploying Monitoring"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/monitoring/00-namespace.yaml 2> /dev/null || true

  # Uninstall Pebble
  echo "Undeploying Pebble"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/pebble/ 2> /dev/null || true

  # Uninstall Petstore
  echo "Undeploying Petstore"
  kubectl delete -f "$PROJECT_DIR"/hub/manifests/petstore/ 2> /dev/null || true
}

createNamespaces() {
  echo "Create Namespaces."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/broker/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/aws-secret-operator/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/pop/00-namespace.yaml
}

createSecrets() {
  echo "Create Secrets."
  kubectl delete secret -n aws-secret-operator aws-secret || true
  kubectl create secret -n aws-secret-operator generic aws-secret --from-literal="api_key=$AWS_CLIENT_ID" --from-literal="api_secret_key=$AWS_CLIENT_SECRET"

  # Install AWS Secret Operator
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/aws-secret-operator/

  # Create CronJob for auth0 service token renewal.
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/auth0/auth0-service-token.yaml
  kubectl create job -n hub auth0-service-token --from cronjobs/auth0-service-token || true

  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/secrets/hub-k3d.yaml
}

installMongo() {
  echo "Deploying Mongo."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/mongo/
  kubectl -n mongo rollout status --watch --timeout="${TIMEOUT}" statefulset/mongodb

  # Insert required data.
  for filename in "${PROJECT_DIR}"/hub/documents/*; do
      # Skip directories.
      if [ -d "$filename" ]; then
          continue
      fi

      file=$(basename "${filename}")
      db=$(echo "${file}" | cut -d '_' -f1)
      col=$(echo "${file}" | cut -d '_' -f2-4 | cut -d '.' -f1)

      kubectl cp "${PROJECT_DIR}"/hub/documents/"${file}" -n mongo "$(kubectl get pods -n mongo -l app=mongodb --output=jsonpath={.items..metadata.name})":/tmp/"${file}"
      kubectl exec -it -n mongo "$(kubectl get pods -n mongo -l app=mongodb --output=jsonpath={.items..metadata.name})" -- bash -c "mongoimport --db ${db} --collection ${col} --file /tmp/${file} --jsonArray --username root --password admin  --authenticationDatabase admin"
  done
}

installHydra() {
  echo "Deploying Hydra."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/hydra/
  kubectl -n hydra rollout status --watch --timeout="${TIMEOUT}" deployment/hydra
}

installTraefik() {
  echo "Deploying Traefik."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/traefik/
  kubectl -n traefik wait --for condition=available --timeout="${TIMEOUT}" deployment/traefik
}

installPebble() {
  echo "Deploying Pebble."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/pebble/
  kubectl -n pebble wait --for condition=available --timeout="${TIMEOUT}" deployment/pebble
}

installJaeger() {
  echo "Deploying Jaeger."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/jaeger/
  kubectl -n jaeger wait --for condition=available --timeout="${TIMEOUT}" deployment/jaeger
}

installNats() {
  echo "Deploying Nats."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/nats/
  kubectl -n nats rollout status --watch --timeout="${TIMEOUT}" statefulset.apps/nats
}

installHub() {
    echo "Deploying Hub services."

    export GCLOUD_EMAIL
    envsubst < "$PROJECT_DIR"/hub/manifests/hub/templates/02-hub-notification.yaml > "$PROJECT_DIR"/hub/manifests/hub/02-hub-notification.yaml
    kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/secrets/
    kubectl apply -f "$PROJECT_DIR"/hub/manifests/hub/

    kubectl rollout status deploy --timeout="${TIMEOUT}" -n hub hub-topology
    kubectl rollout status deploy --timeout="${TIMEOUT}" -n hub hub-cluster
    kubectl rollout status deploy --timeout="${TIMEOUT}" -n hub hub-token
    kubectl rollout status deploy --timeout="${TIMEOUT}" -n hub hub-offer
}

installBroker() {
  echo "Deploying Broker service."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/broker/
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/broker/secrets
}

installPoP() {
  echo "Deploying PoP services."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/pop/secrets
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/pop
}

installTraefikHub() {
  echo "Installing Traefik Hub Sidecar."

  # Create cluster on Hub
  export TOKEN_CLUSTER=$(curl --silent --location --request POST 'http://platform.docker.localhost/cluster/external/clusters' \
  --header "Authorization: Bearer ${JWT_EXTERNAL}" \
  --header 'Content-Type: application/json' \
  --data-raw "{\"name\": \"cluster\"}" | jq -r '.token' | tr -d '\n')

  # Deploy custom resources.
  kubectl apply -f "https://hub.traefik.io/install/crd"
  kubectl apply -f "https://hub.traefik.io/install/rbac?serviceAccountName=traefik&serviceAccountNamespace=traefik"

  # Patch Traefik deployment.
  if [ "$TOKEN_CLUSTER" != "null" ]; then
    envsubst < "$PROJECT_DIR"/hub/manifests/traefik-hub/templates/traefik-deployment-patch.yaml > "$PROJECT_DIR"/hub/manifests/traefik-hub/01-traefik-deployment-patch.yaml
  fi

  kubectl patch -n traefik deployments/traefik --patch-file "$PROJECT_DIR"/hub/manifests/traefik-hub/01-traefik-deployment-patch.yaml

  # Configure the Admission Service.
  kubectl expose deployment --namespace traefik traefik --port=443 --target-port=9943 --name admission
  kubectl apply -f "https://hub-preview.traefik.io/install/admission?namespace=traefik&token=${TOKEN_CLUSTER}"
}

installMonitoring() {
  echo "Deploying monitoring."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/monitoring/00-namespace.yaml
  kubectl delete configmap -n monitoring grafana-dashboard || true
  kubectl create configmap -n monitoring grafana-dashboard --from-file="$PROJECT_DIR"/hub/manifests/monitoring/dashboards/
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/monitoring/
}

installWhoami() {
  echo "Deploying whoami."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/whoami/00-namespace.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/whoami/01-whoami.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/whoami/02-acp.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/whoami/03-ingress-traefik.yaml
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/whoami/03-ingress-traefik-tls.yaml

  kubectl -n whoami wait --for condition=available --timeout="${TIMEOUT}" deployment/whoami
}

installPetstore() {
  echo "Deploying petstore."
  kubectl apply -f "$PROJECT_DIR"/hub/manifests/petstore/

  set +o errexit
  kubectl delete secret -n petstore gcr-access-token
  set -o errexit

  kubectl create secret -n petstore docker-registry gcr-access-token \
          --docker-server=gcr.io \
          --docker-username=oauth2accesstoken \
          --docker-password="$(gcloud auth print-access-token)" \
          --docker-email=${GCLOUD_EMAIL}

  kubectl -n petstore wait --for condition=available --timeout="${TIMEOUT}" deployment/petstore
}

createUser() {
  for filename in "${PROJECT_DIR}"/hub/documents/user_data/*; do
      file=$(basename "${filename}")
      db=$(echo "${file}" | cut -d '_' -f1)
      col=$(echo "${file}" | cut -d '_' -f2-4 | cut -d '.' -f1)

      kubectl cp "${PROJECT_DIR}"/hub/documents/user_data/"${file}" -n mongo "$(kubectl get pods -n mongo -l app=mongodb --output=jsonpath={.items..metadata.name})":/tmp/"${file}"
      kubectl exec -it -n mongo "$(kubectl get pods -n mongo -l app=mongodb --output=jsonpath={.items..metadata.name})" -- bash -c "mongoimport --db ${db} --collection ${col} --file /tmp/${file} --jsonArray --username root --password admin  --authenticationDatabase admin"
    done
}

initializeWorkspace() {
  # Create subscription
  curl --silent --location --request POST 'http://platform.docker.localhost/offer/internal/subscriptions' \
  --header 'Content-Type: application/json' \
  --data-raw "{\"userId\": \"fd016582-3e6a-4951-a9c5-e03e81d63761\", \"workspaceId\": \"${WORKSPACE_ID}\"}"

  # Create topology
    curl --silent --location --request POST 'http://platform.docker.localhost/topology/internal/workspaces' \
    --header 'Content-Type: application/json' \
    --data-raw "{\"id\": \"${WORKSPACE_ID}\"}"
}

initializeVariables() {
  readonly PROJECT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}")" && pwd)/.."
  export ENV_FILE=${PROJECT_DIR}/hub/.env
  [[ -f "${ENV_FILE}" ]] && source "${ENV_FILE}"

  export PROJECT_DIR
  export TIMEOUT="${TIMEOUT:-180s}"
  export K3S_IMAGE="${K3S_IMAGE-"rancher/k3s:v1.24.4-k3s1"}"

  export WORKSPACE_ID="${WORKSPACE_ID:-6311c90bfce04bd29e473a20}"

  [[ "$GCLOUD_EMAIL" == "" ]] && read -p "Enter gcloud email: " GCLOUD_EMAIL
  [[ "$AWS_CLIENT_ID" == "" ]] && read -p "Enter aws client id: " AWS_CLIENT_ID
  [[ "$AWS_CLIENT_SECRET" == "" ]] && read -p "Enter aws client secret: " AWS_CLIENT_SECRET
  [[ "$HUB_USERNAME" == "" ]] && read -p "Enter your hub username: " HUB_USERNAME
  [[ "$HUB_PASSWORD" == "" ]] && read -p "Enter your hub password: " HUB_PASSWORD

  INSTALL_POP="${INSTALL_POP:-false}"
  INSTALL_BROKER="${INSTALL_BROKER:-false}"
  INSTALL_JAEGER="${INSTALL_JAEGER:-false}"
  INSTALL_MONITORING="${INSTALL_MONITORING:-false}"
  INSTALL_WHOAMI="${INSTALL_WHOAMI:-false}"
  INSTALL_PETSTORE="${INSTALL_PETSTORE:-false}"
}

initializeVariables

cmd=$1

case $cmd in
    apply-coredns-conf)
        applyCoreDNSConf
    ;;
    clean)
        clean
    ;;
    create-user)
        createUser
    ;;
    helm-update)
        helmUpdate
    ;;
    install-broker)
        installBroker
    ;;
    install-jaeger)
        installJaeger
    ;;
    install-monitoring)
        installMonitoring
    ;;
    install-petstore)
        installPetstore
    ;;
    install-pop)
        installPoP
    ;;
    install-whoami)
        installWhoami
    ;;
    renew-auth0-admin-token)
        renewAuth0AdminToken
    ;;
    renew-gcr-token)
        renewGCRToken
    ;;
    renew-jwt)
        renewJWT
    ;;
    run)
        main "$@"
    ;;
    *)
        echo "Commands available: apply-core-dns-conf, clean, create-user, helm-update, install-broker," \
          "install-jaeger, install-monitoring, install-petstore, install-pop, install-whoami, renew-auth0-admin-token," \
          "renew-gcr-token, renew-jwt, run"
        exit 1
    ;;
esac
