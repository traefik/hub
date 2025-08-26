---
description: 'Traefik Hub API Gateway Quick Start - Publish your first APIs using CRDs.'
id: 'getting-started'
sidebar_label: 'Getting Started'
tags:
- API
- Kubernetes
- GitOps
- API Gateway
---

# Getting Started

Traefik Hub API Gateway is cloud-native and multi-platform.

We can start:

1. on [Kubernetes](#on-kubernetes)
2. on [Linux](#on-linux)

## Pre-requisites

- A Traefik Hub account and [license](https://doc.traefik.io/traefik-hub/legal/licensing).
- [Helm](https://helm.sh/) installed.
- Allow outgoing requests to api.traefik.io on HTTPS ports (TCP/443).

## On Kubernetes

For this tutorial, we deploy Traefik Hub API Gateway on a Kubernetes cluster using k3s. It's possible to use alternatives such as [kind](https://kind.sigs.k8s.io), [k3d](https://k3d.io/),
[k3s](https://k3s.io/) cloud providers, and others.

:::warning
It's important to disable the built-in Traefik ingress for [k3d](https://k3d.io/v5.3.0/design/concepts/#example) and [k3s](https://docs.k3s.io/networking/networking-services#:~:text=To%20remove%20Traefik%20from%20your,Release%20Notes%20for%20your%20version.)
clusters to avoid possible conflicts. Refer to their documentation to see how to disable it.
:::

First, clone the GitHub repository dedicated to tutorials:

```shell
git clone https://github.com/traefik/hub.git
cd hub
```

### Create a Kubernetes Cluster Using k3s

```shell
curl -sfL https://get.k3s.io | K3S_KUBECONFIG_MODE="644" INSTALL_K3S_EXEC="--disable traefik" sh -
```

:::info

In the command above:

- K3S_KUBECONFIG_MODE="644" lets non-root users run kubectl.
- INSTALL_K3S_EXEC="--disable traefik" disables the built-in Traefik to avoid conflicts.

Note: This configuration is intended for demonstration or development purposes only. It is not recommended for production environments.
For more advanced configuration options, refer to the official K3s documentation: [K3s Configuration](https://docs.k3s.io/installation/configuration)

:::

### Create a Kubernetes Cluster Using kind

kind requires some configuration to use an IngressController on localhost. See the following example:

<details>

<summary>Create the cluster</summary>

Ports need to be mapped for HTTP and HTTPS for kind with this config:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: traefik-hub
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30000
    hostPort: 80
    protocol: TCP
  - containerPort: 30001
    hostPort: 443
    protocol: TCP
```

```shell
kind create cluster --config=src/kind/config.yaml
kubectl cluster-info
kubectl wait --for=condition=ready nodes traefik-hub-control-plane
```

</details>

### Step 1: Install Traefik Hub API Gateway

Log in to the [Traefik Hub Online Dashboard](https://hub.traefik.io), navigate to the **Gateways** page to [create a new gateway](https://hub.traefik.io/gateways/new).

Click on Quick getting started instruction and select Kubernetes option.

Copy the content of 'Configuration' box.

Open a terminal and run the copied commands to install Traefik Hub using Helm.

For example:

```shell
# Add the Helm repository
helm repo add --force-update traefik https://traefik.github.io/charts

# Install the Ingress Controller
kubectl create namespace traefik
kubectl create secret generic traefik-hub-license --namespace traefik --from-literal=token=<traefik-hub-license>
helm upgrade --install --namespace traefik traefik traefik/traefik \
  --set hub.token=traefik-hub-license \
  --set image.registry=ghcr.io \
  --set image.repository=traefik/traefik-hub \
  --set image.tag=latest-v3
```

The following will be displayed in your terminal  after running the commands:

```shell
"traefik" has been added to your repositories
namespace/traefik created
secret/traefik-hub-license created
Release "traefik" does not exist. Installing it now.
NAME: traefik
LAST DEPLOYED: Mon Aug 12 20:00:00 2024
NAMESPACE: traefik
STATUS: deployed
REVISION: 1
TEST SUITE: None
NOTES:
traefik with ghcr.io/traefik/traefik-hub:v3.17.3 has been deployed successfully on traefik namespace !
```

**If** Traefik Hub API Gateway is **already** installed, copy the new token generated from the gateway creation page and use it to generate the `traefik-hub-license` secret :

```shell
export TRAEFIK_HUB_TOKEN= <paste-token-here>
```

```shell
kubectl create secret generic traefik-hub-license --namespace traefik --from-literal=token=$TRAEFIK_HUB_TOKEN
```

Next, upgrade Traefik helm chart:

```shell
# Upgrade CRDs
kubectl apply --server-side --force-conflicts -k https://github.com/traefik/traefik-helm-chart/traefik/crds/
# Update the Helm repository
helm repo update
#Create the traefik-hub-license secret
kubectl create secret generic traefik-hub-license --namespace traefik --from-literal=token=$TRAEFIK_HUB_TOKEN
# Upgrade the Helm chart
helm upgrade traefik -n traefik --wait traefik/traefik \
  --set hub.token=traefik-hub-license \
  --set image.registry=ghcr.io \
  --set image.repository=traefik/traefik-hub \
  --set image.tag=latest-v3 \
```

### Step 2: Deploy an API as an Ingress

Without Traefik Hub API Gateway, an API can be deployed as an `Ingress`, an `IngressRoute` or an `HTTPRoute`.

In this tutorial, APIs are implemented using a JSON server in Go; the source code is [here](https://github.com/traefik/hub/blob/main/src/api-server/).

Let's deploy a [weather app](https://github.com/traefik/hub/blob/main/src/manifests/weather-app.yaml) exposing an API.

```shell
kubectl apply -f src/manifests/weather-app.yaml
```

It should create the public app

```shell
namespace/apps created
configmap/weather-data created
middleware.traefik.io/stripprefix-weather created
deployment.apps/weather-app created
service/weather-app created
configmap/weather-app-openapispec created
```

It can be exposed with an `IngressRoute`:

```yaml
---
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: getting-started-apigateway
  namespace: apps
spec:
  entryPoints:
    - web
  routes:
  - match: Host(`localhost`) && PathPrefix(`/weather`)
    kind: Rule
    services:
    - name: weather-app
      port: 3000
    middlewares:
      - name: stripprefix-weather
```

```shell
kubectl apply -f api-gateway/1-getting-started/manifests/weather-app-ingressroute.yaml
```

After applying the resources, it creates the `IngressRoute` resource:

```shell
ingressroute.traefik.io/getting-started-apigateway created
```

This API can be accessed using curl:

```shell
curl http://localhost/weather | jq
```

```json
[
  {"city":"GopherCity","id":"0","weather":"Moderate rain"},
  {"city":"City of Gophers","id":"1","weather":"Sunny"},
  {"city":"GopherRocks","id":"2","weather":"Cloudy"}
]
```

### Step 3: Secure authentication on this API with Traefik Hub

Let's secure the weather API with an API Key.

First, we need to generate a password hash before proceeding. This can be done with `htpasswd`. If it is not already installed, install it by running:

```shell
sudo apt update
sudo apt install apache2-utils -y
```

```shell
htpasswd -nbs "" "Let's use API Key with Traefik Hub" | cut -c 2-
```

It should output a hash similar to this:

```shell
{SHA}dhiZGvSW60OMQ+J6hPEyJ+jfUoU=
```

Next, store this hash in a `Secret` for the API Key `Middleware` and create a new `IngressRoute`:

```yaml
cat <<'EOF' | kubectl apply -f -
---
apiVersion: v1
kind: Secret
metadata:
  name: getting-started-apigateway-apikey-auth
  namespace: apps
stringData:
  secretKey: "{SHA}dhiZGvSW60OMQ+J6hPEyJ+jfUoU=" # Generated API secretKey

---
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: getting-started-apigateway-apikey-auth
  namespace: apps
spec:
  plugin:
    apiKey:
      keySource:
        header: Authorization
        headerAuthScheme: Bearer
      secretValues:
        - urn:k8s:secret:getting-started-apigateway-apikey-auth:secretKey

---
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: getting-started-apigateway-api-key
  namespace: apps
spec:
  entryPoints:
    - web
  routes:
  - match: Host(`localhost`) && PathPrefix(`/api-key`)
    kind: Rule
    services:
    - name: weather-app
      port: 3000
    middlewares:
    - name: stripprefix-weather
    - name: getting-started-apigateway-apikey-auth
EOF
```

After applying the resources, it creates the `Secret`, `Middleware`, and `IngressRoute` resources:

```shell
secret/getting-started-apigateway-apikey-auth created
middleware.traefik.io/getting-started-apigateway-apikey-auth created
ingressroute.traefik.io/getting-started-apigateway-api-key created
```

And test it:

```shell
# This call is not authorized => 401
curl -i http://localhost/api-key/weather
# Let's set the API key
export API_KEY=$(echo -n "Let's use API Key with Traefik Hub" | base64)
# This call with the token is allowed => 200
curl -i -H "Authorization: Bearer $API_KEY" http://localhost/api-key/weather
```

The API is now secured.

## On Linux

This tutorial will show how to use Traefik Hub API Gateway on Linux using a shell command (for simplicity).
In production, we recommend using Infra-as-Code or even GitOps.

:::info
We will use a Debian Linux in this tutorial.
:::

First, clone the GitHub repository dedicated to tutorials:

```shell
git clone https://github.com/traefik/hub.git
cd hub
```

### Step 1: Install Traefik Hub API Gateway

Get the Traefik Hub API Gateway binary:

```shell
# Download the binary
LATEST_VERSION=$(curl -s https://api.github.com/repos/traefik/hub/releases/latest | grep 'tag_name' | cut -d '"' -f 4)
curl -L "https://github.com/traefik/hub/releases/download/$LATEST_VERSION/traefik-hub_${LATEST_VERSION}_linux_amd64.tar.gz" -o /tmp/traefik-hub.tar.gz
tar xvzf /tmp/traefik-hub.tar.gz -C /tmp traefik-hub-linux-amd64
rm -f /tmp/traefik-hub.tar.gz
# Install the binary with the required rights
sudo mv /tmp/traefik-hub-linux-amd64 /usr/local/bin/traefik-hub
sudo chown root:root /usr/local/bin/traefik-hub
sudo chmod 755 /usr/local/bin/traefik-hub
# Give the Traefik Hub binary ability to bind privileged ports like 80 or 443 as non-root
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/traefik-hub
```

Create the config resources:

```shell
# Create a user
sudo groupadd --system traefik-hub
sudo useradd \
  -g traefik-hub --no-user-group \
  --home-dir /var/www --no-create-home \
  --shell /usr/sbin/nologin \
  --system traefik-hub
# Create a config directory
sudo mkdir -p /etc/traefik-hub/dynamic
sudo chown root:root /etc/traefik-hub
sudo chown traefik-hub:traefik-hub /etc/traefik-hub/dynamic
# Create a log file
sudo touch /var/log/traefik-hub.log
sudo chown traefik-hub:traefik-hub /var/log/traefik-hub.log
```

Log in to the [Traefik Hub Online Dashboard](https://hub.traefik.io), open the page to [generate a new gateway](https://hub.traefik.io/gateways/new).

:::warning
Do not install the gateway, but copy the token.
:::

Export your token:

```shell
export TRAEFIK_HUB_TOKEN=SET_YOUR_TOKEN_HERE
```

With this token, we can add a [static configuration file](https://github.com/traefik/hub/blob/main/api-gateway/1-getting-started/linux/traefik-hub.toml) for Traefik Hub API Gateway and a [systemd service](https://github.com/traefik/hub/blob/main/api-gateway/1-getting-started/linux/traefik-hub.service):

```shell
sudo cp api-gateway/1-getting-started/linux/traefik-hub.toml /etc/traefik-hub/traefik-hub.toml
sudo sed -i -e "s/PASTE_YOUR_TOKEN_HERE/$TRAEFIK_HUB_TOKEN/g" /etc/traefik-hub/traefik-hub.toml
sudo cp api-gateway/1-getting-started/linux/traefik-hub.service /etc/systemd/system/traefik-hub.service
sudo chown root:root /etc/systemd/system/traefik-hub.service
sudo chmod 644 /etc/systemd/system/traefik-hub.service
sudo systemctl daemon-reload
sudo systemctl enable --now traefik-hub.service
```

Ensure the service is working as expected using the dedicated command:

```shell
sudo systemctl status traefik-hub.service
```

```shell
● traefik-hub.service - Traefik Hub
     Loaded: loaded (/etc/systemd/system/traefik-hub.service; enabled; preset: enabled)
     Active: active (running) since [...]; 2s ago
   Main PID: 2516 (traefik-hub)
      Tasks: 7 (limit: 1141)
     Memory: 30.6M
        CPU: 401ms
     CGroup: /system.slice/traefik-hub.service
             └─2516 /usr/local/bin/traefik-hub --configfile=/etc/traefik-hub/traefik-hub.toml

[...] systemd[1]: Started traefik-hub.service - Traefik Hub.
```

### Step 2: Expose an API

:::info
On Linux, we can use all the providers supported by Traefik Proxy and Traefik Hub API Gateway.
:::

In this example, we'll set a configuration using a YAML file.
We will deploy a _whoami_ application on systemd and reach it from Traefik Proxy.

```shell
# Install whoami
curl -L https://github.com/traefik/whoami/releases/download/v1.11.0/whoami_v1.11.0_linux_amd64.tar.gz -o /tmp/whoami.tar.gz
tar xvzf /tmp/whoami.tar.gz -C /tmp whoami
rm -f /tmp/whoami.tar.gz
sudo mv /tmp/whoami /usr/local/bin/whoami
sudo chown root:root /usr/local/bin/whoami
sudo chmod 755 /usr/local/bin/whoami
# Create a user for whoami
sudo groupadd --system whoami
sudo useradd \
  -g whoami --no-user-group \
  --home-dir /var/www --no-create-home \
  --shell /usr/sbin/nologin \
  --system whoami
```

Enable this app with a [systemd unit file](https://github.com/traefik/hub/blob/main/api-gateway/1-getting-started/linux/whoami.service):

```shell
sudo cp api-gateway/1-getting-started/linux/whoami.service /etc/systemd/system/whoami.service
sudo chmod 644 /etc/systemd/system/whoami.service
sudo chown root:root /etc/systemd/system/whoami.service
sudo systemctl daemon-reload
sudo systemctl enable --now whoami
sudo systemctl status whoami
```

And check that it's working as expected:

```shell
curl http://localhost:3000
```

```shell
Hostname: ip-172-31-26-184
IP: 127.0.0.1
IP: ::1
IP: 172.31.26.184
IP: fe80::8ff:eeff:fed5:2389
IP: 172.17.0.1
IP: 172.18.0.1
IP: fe80::42:92ff:fe17:7a6d
IP: fe80::4418:56ff:fe7b:4a46
RemoteAddr: 127.0.0.1:52412
GET / HTTP/1.1
Host: localhost:3000
User-Agent: curl/7.88.1
Accept: */*
```

Now, add a [dynamic configuration file](https://github.com/traefik/hub/blob/main/api-gateway/1-getting-started/linux/whoami.yaml) to expose it through Traefik Hub API Gateway.

Let's apply this tutorial configuration and test it:

```shell
# Not configured => 404
curl -I http://whoami.localhost
sudo cp api-gateway/1-getting-started/linux/whoami.yaml /etc/traefik-hub/dynamic/whoami.yaml
sleep 5
# Configured => 200
curl http://whoami.localhost
```

```shell
Hostname: ip-172-31-26-184
IP: 127.0.0.1
IP: ::1
IP: 172.31.26.184
IP: fe80::8ff:eeff:fed5:2389
IP: 172.17.0.1
IP: 172.18.0.1
IP: fe80::42:92ff:fe17:7a6d
IP: fe80::4418:56ff:fe7b:4a46
RemoteAddr: [::1]:59954
GET / HTTP/1.1
Host: whoami.localhost
User-Agent: curl/7.88.1
Accept: */*
Accept-Encoding: gzip
X-Forwarded-For: 127.0.0.1
X-Forwarded-Host: whoami.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Forwarded-Server: ip-172-31-26-184
X-Real-Ip: 127.0.0.1
```

### Step 3: Secure The Access Using an API Key

Let's secure the access with an API Key.

Next, we need to generate a password hash before proceeding. This can be done with `htpasswd`. If it is not already installed, install it by running:

```shell
sudo apt update
sudo apt install apache2-utils -y
```

```shell
htpasswd -nbs "" "Let's use API Key with Traefik Hub" | cut -c 2-
{SHA}dhiZGvSW60OMQ+J6hPEyJ+jfUoU=
```

```shell
{SHA}dhiZGvSW60OMQ+J6hPEyJ+jfUoU=
```

Put this password in the API Key middleware:

```yaml showLineNumbers {16}
http:
  routers:
    whoami:
      rule: Host(`whoami.localhost`)
      service: local
      middlewares:
      - apikey-auth

  middlewares:
    apikey-auth:
      plugin:
        apikey:
          keySource:
            header: Authorization
            headerAuthScheme: Bearer
          secretValues: "{SHA}dhiZGvSW60OMQ+J6hPEyJ+jfUoU="

  services:
    local:
      loadBalancer:
        servers:
          - url: http://localhost:3000
```

Let's apply it:

```shell
sudo cp api-gateway/1-getting-started/linux/whoami-apikey.yaml /etc/traefik-hub/dynamic/whoami.yaml
sleep 5
```

And test it:

```shell
# This call is not authorized => 401
curl -I http://whoami.localhost
# Let's set the token
export API_KEY=$(echo -n "Let's use API Key with Traefik Hub" | base64)
# This call with the token is allowed => 200
curl -I -H "Authorization: Bearer $API_KEY" http://whoami.localhost
```

The API is now secured.

## Docker Providers

We can also use the docker provider. You may have noticed that we already enabled it in the static configuration.

Let's see how it works. First, let's install docker:

```shell
sudo apt-get update
sudo apt-get install -y docker.io docker-compose
```

And give the _traefik-hub_ user access to docker resources:

```shell
sudo usermod -aG docker traefik-hub
sudo systemctl restart traefik-hub.service
```

Now we can test the service with a [docker compose](https://github.com/traefik/hub/blob/main/api-gateway/1-getting-started/linux/docker-compose.yaml) file:

```shell
sudo docker compose -f $(pwd)/api-gateway/1-getting-started/linux/docker-compose.yaml up -d
```

Since we already enabled the docker provider in Traefik Hub API Gateway configuration, we should now be able to curl it:

```shell
curl http://whoami.docker.localhost
```

```shell
Hostname: cfd52cc4b3a6
IP: 127.0.0.1
IP: 172.18.0.2
RemoteAddr: 172.18.0.1:35766
GET / HTTP/1.1
Host: whoami.docker.localhost
User-Agent: curl/7.88.1
Accept: */*
Accept-Encoding: gzip
X-Forwarded-For: 127.0.0.1
X-Forwarded-Host: whoami.docker.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Forwarded-Server: ip-172-31-26-184
X-Real-Ip: 127.0.0.1
```

## Related Content

- See how to secure your API with a [JWT token](./secure/middleware/jwt.md)
- See how to secure the [Traefik Hub API Gateway's dashboard](./secure/middleware/api.md).
- See how to secure your API access using [OIDC](./secure/middleware/oidc.md "Link to documentation about how to enable OIDC in Traefik Hub API Gateway").
