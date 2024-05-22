# Walkthrough

This document covers a complete journey: Traefik Proxy => Traefik Hub API Gateway => Traefik Hub API Management

## Deploy Kubernetes

In this tutorial, one can use [k3d](https://k3d.io/). Alternatives like [kind](https://kind.sigs.k8s.io), cloud providers, or others can also be used.

First, clone this GitHub repository:

```shell
git clone https://github.com/traefik/hub.git
cd hub
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

## Deploy an API with Traefik Proxy

First, we will install Traefik Proxy with Helm:

```shell
# Add the Helm repository
helm repo add --force-update traefik https://traefik.github.io/charts
# Create a namespace
kubectl create namespace traefik-hub
# Install the Helm chart
helm install traefik -n traefik-hub --wait \
  --set ingressRoute.dashboard.matchRule='Host(`dashboard.docker.localhost`)' \
  --set ingressRoute.dashboard.entryPoints={web} \
  --set ports.web.nodePort=30000 \
  --set ports.websecure.nodePort=30001 \
   traefik/traefik
```

Once it's installed, we can access the local dashboard: http://dashboard.docker.localhost/

![Local Traefik Proxy Dashboard](./src/images/dashboard.png)

Without Traefik Hub, an API can be deployed with an `Ingress`, an `IngressRoute` or a `HTTPRoute`.

This tutorial implements APIs using a simple JSON server in Go; the source code is [here](../../src/api-server/).

Let's deploy a [simple weather app](../../src/manifests/weather-app.yaml) exposing an API.

```shell
kubectl apply -f src/manifests/weather-app.yaml
```

It should create the public app:

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
  namespace: traefik-hub
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

With Traefik Proxy, we can protect this API with a simple Basic Authentication. To create an encoded user:password pair, the following command can be used: `htpasswd -nb user password | openssl base64`

So let's do it:

```shell
htpasswd -nb foo bar | openssl base64
```

```shell
Zm9vOiRhcHIxJDJHR0RyLjJPJDdUVXJlOEt6anQ1WFFOUGRoby5CQjEKCg==
```

```diff
--- src/manifests/weather-app-ingressroute.yaml
+++ src/manifests/weather-app-basic-auth.yaml
@@ -1,4 +1,24 @@
 ---
+apiVersion: v1
+kind: Secret
+metadata:
+  name: basic-auth
+  namespace: traefik-hub
+data:
+  users: |
+    Zm9vOiRhcHIxJDJHR0RyLjJPJDdUVXJlOEt6anQ1WFFOUGRoby5CQjEKCg==
+
+---
+apiVersion: traefik.io/v1alpha1
+kind: Middleware
+metadata:
+  name: basic-auth
+  namespace: traefik-hub
+spec:
+  basicAuth:
+    secret: basic-auth
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
+    - name: basic-auth
```

Let's apply it:

```shell
kubectl apply -f src/manifests/weather-app-basic-auth.yaml
```

```shell
secret/basic-auth created
middleware.traefik.io/basic-auth created
ingressroute.traefik.io/weather-api configured
```

And now, we can confirm it's protected with a simple basic auth :

```shell
# This call is not authorized => 401
curl -I http://api.docker.localhost/weather
# This call is allowed => 200
curl -I -u foo:bar http://api.docker.localhost/weather
```

[Basic Authentication](https://datatracker.ietf.org/doc/html/rfc7617) worked and was widely used in the early days of the web. However, it also has a security risk: credentials can be visible to any observer when using HTTP. It uses hard-coded credentials, potentially giving more authorization than required for a specific use case.

Nowadays, those issues are addressed when using [JSON Web Tokens (JWT)](https://datatracker.ietf.org/doc/html/rfc7519). A JWT can be cryptographically verified, detach authentication from user credentials, and has an issue and expiration date. JWT can be used with Traefik Hub API Gateway, so let's upgrade our setup to Traefik Hub

## Upgrade Traefik Proxy to Traefik Hub API Gateway

Log in to the [Traefik Hub Online Dashboard](https://hub.traefik.io), open the page to [generate a new agent](https://hub.traefik.io/agents/new).

**:warning: Do not install the agent, but copy the token.**

Now, open a terminal and run these commands to create the secret for Traefik Hub.

```shell
export TRAEFIK_HUB_TOKEN=
```

```shell
kubectl create secret generic license --namespace traefik-hub --from-literal=token=$TRAEFIK_HUB_TOKEN
```

After, we can upgrade Traefik Proxy to Traefik Hub using the same Helm chart:

```shell
helm upgrade traefik -n traefik-hub --wait \
  --reuse-values \
  --set hub.token=license \
  --set image.registry=ghcr.io \
  --set image.repository=traefik/traefik-hub \
  --set image.tag=v3.0.0 \
   traefik/traefik
```

Traefik Hub is 100% compatible with Traefik Proxy v3.

We can confirm it by taking a look at the local dashboard: http://dashboard.docker.localhost/

![Local Traefik Hub Dashboard](./src/images/hub-dashboard.png)

And also confirm _Basic Auth_ is still here:

```shell
# This call is not authorized => 401
curl -I http://api.docker.localhost/weather
# This call is allowed => 200
curl -I -u foo:bar http://api.docker.localhost/weather
```

Let's try to protect the weather API with a JWT Token. For the sake of simplicity, we'll generate the token using only a shared signing secret and the online https://jwt.io tool.

With Traefik Hub, we can use JWT as a middleware:

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
+  namespace: traefik-hub
+stringData:
+  signingSecret: "JWT on Traefik Hub!"
+
+---
+apiVersion: traefik.io/v1alpha1
+kind: Middleware
+metadata:
+  name: jwt-auth
+  namespace: traefik-hub
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

It's way better now in terms of security. It's possible to handle users with an Identity Provider, but what if we want to cover _internal_ and _external_ use cases? To protect API on HTTP _verb_ level? Or to test a new version with part of the production traffic?

We'll need Traefik Hub with API Management!

## Manage an API with Traefik Hub API Management

First, we need to enable API Management on Traefik Traefik Hub using the same Helm chart:

```shell
helm upgrade traefik -n traefik-hub --wait \
  --reuse-values \
  --set hub.apimanagement.enabled=true \
   traefik/traefik
```

Traefik Hub API Management is 100% compatible with Traefik Proxy v3 and Traefik Hub API Gateway.

We can confirm it by taking a look at the local dashboard: http://dashboard.docker.localhost/

![Local Traefik Hub Dashboard](./src/images/hub-dashboard.png)

And also, confirm _JWT Auth_ is still here:

```shell
# This call is not authorized => 401
curl -I http://api.docker.localhost/weather
# Let's set the token
export JWT_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.AuyxLr6YEAIdMxXujJ2icNvMCamR1SizrunWlyfLlJw"
# This call with the token is allowed => 200
curl -I -H "Authorization: Bearer $JWT_TOKEN" http://api.docker.localhost/weather
```

Now, let's try to manage it with Traefik Hub using `API` and `APIAccess` resources:

```yaml
---
apiVersion: hub.traefik.io/v1alpha1
kind: API
metadata:
  name: weather-api
  namespace: traefik-hub
spec: {}

---
apiVersion: hub.traefik.io/v1alpha1
kind: APIAccess
metadata:
  name: weather-api
  namespace: traefik-hub
spec:
  apis:
    - name: weather-api
      namespace: traefik-hub
  everyone: true
```

We'll need to reference this API in the `IngressRoute` with an annotation:

```yaml
---
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: weather-api
  namespace: traefik-hub
  annotations:
    hub.traefik.io/api: weather-api # <=== Link to the API using its name
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

:information_source: We've also removed the JWT authentication middleware, as we'll use Traefik Hub's built-in identity provider for user and credential management. Don't worry; the API is still secured, as you'll see it shortly.

Let's apply it:

```shell
kubectl apply -f api-management/1-getting-started/manifests/api.yaml
```

It will create `API`, `APIAccess` and link `IngressRoute` to this API.

```shell
api.hub.traefik.io/weather-api created
apiaccess.hub.traefik.io/weather-api created
ingressroute.traefik.io/weather-api configured
```

Now, we can confirm this API is not publicly exposed:

```shell
curl -i http://api.docker.localhost/weather
```

It returns the expected 401 Unauthorized HTTP code:

```shell
HTTP/1.1 401 Unauthorized
Date: Mon, 06 May 2024 12:09:56 GMT
Content-Length: 0
```

## Create a user for this API

Users can be created in the [Traefik Hub Online Dashboard](https://hub.traefik.io/users):

![Create user admin](./api-management/1-getting-started/images/create-user-admin.png)

This user will connect to an API Portal to generate an API key, so let's deploy the API Portal!

```yaml
---
apiVersion: hub.traefik.io/v1alpha1
kind: APIPortal
metadata:
  name: apiportal
  namespace: traefik-hub
spec:
  title: API Portal
  description: "Developer Portal"
  trustedUrls:
    - api.docker.localhost

---
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: apiportal
  namespace: traefik-hub
  annotations:
    hub.traefik.io/api-portal: apiportal
spec:
  entryPoints:
    - web
  routes:
  - match: Host(`api.docker.localhost`)
    kind: Rule
    services:
    - name: apiportal
      port: 9903
```

:information_source: This API Portal is routed with the internal _ClusterIP_ `Service` named apiportal.

```shell
kubectl apply -f api-management/1-getting-started/manifests/api-portal.yaml
sleep 30
```

```shell
apiportal.hub.traefik.io/apiportal created
ingressroute.traefik.io/apiportal created
```

The API Portal should be accessible on http://api.docker.localhost

You should be able to log in with the admin user.

![API Portal Log in](./api-management/1-getting-started/images/api-portal-login.png)

And create a token for this user:

![API Portal Create Token](./api-management/1-getting-started/images/api-portal-create-token.png)

```shell
export ADMIN_TOKEN="XXX"
```

With this token, it is possible to request the API: :tada:

```shell
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.docker.localhost/weather
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
