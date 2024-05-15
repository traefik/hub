# Getting Started

## Deploy Kubernetes

In this tutorial, one can use [k3d](https://k3d.io/). Alternatives like [kind](https://kind.sigs.k8s.io), cloud providers, or others can also be used.

First, clone this GitHub repository:

```shell
git clone https://github.com/traefik/hub.git
cd traefik-hub
```

### Using k3d

```shell
k3d cluster create traefik-hub --port 80:80@loadbalancer --port 443:443@loadbalancer --port 8000:8000@loadbalancer --k3s-arg "--disable=traefik@server:0"
```

### Using Kind

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

And add a load balancer (LB) to it:

```shell
kubectl apply -f src/kind/metallb-native.yaml
kubectl wait --namespace metallb-system --for=condition=ready pod --selector=app=metallb --timeout=90s
kubectl apply -f src/kind/metallb-config.yaml
```

</details>

## Install Traefik Hub

Log in to the [Traefik Hub Online Dashboard](https://hub-preview.traefik.io), open the page to [generate a new agent](https://hub-preview.traefik.io/agents/new).

**:warning: Do not install the agent, but copy the token.**

Now, open a terminal and run these commands to create the secret for Traefik Hub.

```shell
export TRAEFIK_HUB_TOKEN=
```

```shell
kubectl create namespace traefik-hub
kubectl create secret generic license --namespace traefik-hub --from-literal=token=${TRAEFIK_HUB_TOKEN}
```

After, we can install Traefik Hub with Helm:

```shell

# Add the Helm repository

helm repo add --force-update traefik https://traefik.github.io/charts
helm install traefik-hub -n traefik-hub --wait \
  --set hub.token=license \
  --set hub.platformUrl=https://platform-preview.hub.traefik.io/agent \
  --set ingressRoute.dashboard.matchRule='Host(`dashboard.docker.localhost`)' \
  --set ingressRoute.dashboard.entryPoints={web} \
  --set image.registry=europe-west9-docker.pkg.dev/traefiklabs \
  --set image.repository=traefik-hub/traefik-hub \
  --set image.tag=latest-v3 \
  --set image.pullPolicy=Always \
  --set ports.web.nodePort=30000 \
  --set ports.websecure.nodePort=30001 \
  --devel --version v28.1.0-beta.3 traefik/traefik
```

**If** Traefik Hub is **already** installed, we can instead upgrade the Traefik Hub instance:

```shell
# Upgrade CRDs
kubectl apply --server-side --force-conflicts -k https://github.com/traefik/traefik-helm-chart/traefik/crds/
# Update Helm Repository
helm repo update
# Upgrade Helm Chart
helm upgrade traefik-hub -n traefik-hub --wait \
  --set hub.token=license \
  --set hub.platformUrl=https://platform-preview.hub.traefik.io/agent \
  --set ingressRoute.dashboard.matchRule='Host(`dashboard.docker.localhost`)' \
  --set ingressRoute.dashboard.entryPoints={web} \
  --set image.registry=europe-west9-docker.pkg.dev/traefiklabs \
  --set image.repository=traefik-hub/traefik-hub \
  --set image.tag=latest-v3 \
  --set image.pullPolicy=Always \
  --set ports.web.nodePort=30000 \
  --set ports.websecure.nodePort=30001 \
  --devel --version v28.1.0-beta.3 traefik/traefik
```

Now we can access the local dashboard: http://dashboard.docker.localhost/

## Deploy an API without Traefik Hub

Without Traefik Hub, an API can be deployed with an `Ingress`, an `IngressRoute` or a `HTTPRoute`.

In this tutorial, APIs are implemented using a simple JSON server in Go; the source code is [here](../../src/api-server/).

Let's deploy a [simple weather app](../../src/manifests/weather-app.yaml) exposing an API.

```shell
kubectl apply -f src/manifests/weather-app.yaml
```

It should create the public app

```shell
namespace/apps created
configmap/weather-data created
deployment.apps/weather-app created
service/weather-app created
```

It can be exposed with an `IngressRoute`:

```yaml
---
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: weather-api
  namespace: apps
spec:
  entryPoints:
    - web
  routes:
  - match: Host(`api.docker.localhost`) && PathPrefix(`/weather`)
    kind: Rule
    services:
    - name: weather-app
      port: 3000
```

```shell
kubectl apply -f src/manifests/weather-app-ingressroute.yaml
```

```shell
ingressroute.traefik.io/weather-api created
```

This API can be accessed using curl:

```shell
curl http://api.docker.localhost/weather
```

```json
{
  "public": [
    { "id": 1, "city": "GopherCity", "weather": "Moderate rain" },
    { "id": 2, "city": "City of Gophers", "weather": "Sunny" },
    { "id": 3, "city": "GopherRocks", "weather": "Cloudy" }
  ]
}
```

## Secure authentication using JWTs on this API with Traefik Hub

In order to keep this getting started short, we'll generate the token using only a shared signing secret and the online https://jwt.io tool.

```diff
diff -Nau src/manifests/weather-app-ingressroute.yaml src/manifests/weather-app-jwt.yaml
--- src/manifests/weather-app-ingressroute.yaml
+++ src/manifests/weather-app-jwt.yaml
@@ -1,4 +1,24 @@
 ---
+apiVersion: v1
+kind: Secret
+metadata:
+  name: jwt-auth
+  namespace: apps
+stringData:
+  signingSecret: "JWT on Traefik Hub!"
+
+---
+apiVersion: traefik.io/v1alpha1
+kind: Middleware
+metadata:
+  name: jwt-auth
+  namespace: apps
+spec:
+  plugin:
+    jwt:
+      signingsecret: urn:k8s:secret:jwt-auth:signingSecret
+
+---
 apiVersion: traefik.io/v1alpha1
 kind: IngressRoute
 metadata:
@@ -13,3 +33,5 @@
     services:
     - name: weather-app
       port: 3000
+    middlewares:
+    - name: jwt-auth
```

Let's apply it:

```shell
kubectl apply -f src/manifests/weather-app-jwt.yaml
```

```shell
secret/jwt-auth created
middleware.traefik.io/jwt-auth created
ingressroute.traefik.io/weather-api configured
```

Get the token from https://jwt.io using the same signing secret:

![JWT Token](./src/images/jwt-token.png)

With this token, we can test it:

```shell
# This call is not authorized => 401
curl -I http://api.docker.localhost/weather
# Let's set the token
export JWT_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.AuyxLr6YEAIdMxXujJ2icNvMCamR1SizrunWlyfLlJw"
# This call with the token is allowed => 200
curl -I -H "Authorization: Bearer $JWT_TOKEN" http://api.docker.localhost/weather
```
