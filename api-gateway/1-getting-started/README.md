# Getting Started

Traefik Hub API Gateway is cloud-native and multi-platform.

We can start:

1. on [Kubernetes](#on-kubernetes)
2. on [Linux](#on-linux)

## On Kubernetes

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

## Install Traefik Hub

Log in to the [Traefik Hub Online Dashboard](https://hub.traefik.io), open the page to [generate a new agent](https://hub.traefik.io/agents/new).

**:warning: Do not install the agent, but copy the token.**

Now, open a terminal and run these commands to create the secret for Traefik Hub.

```shell
export TRAEFIK_HUB_TOKEN=
```

```shell
kubectl create namespace traefik-hub
kubectl create secret generic license --namespace traefik-hub --from-literal=token=$TRAEFIK_HUB_TOKEN
```

After, we can install Traefik Hub with Helm:

```shell
# Add the Helm repository
helm repo add --force-update traefik https://traefik.github.io/charts
# Install the Helm chart
helm install traefik-hub -n traefik-hub --wait \
  --set hub.token=license \
  --set ingressRoute.dashboard.matchRule='Host(`dashboard.docker.localhost`)' \
  --set ingressRoute.dashboard.entryPoints={web} \
  --set image.registry=ghcr.io \
  --set image.repository=traefik/traefik-hub \
  --set image.tag=v3.0.0 \
  --set ports.web.nodePort=30000 \
  --set ports.websecure.nodePort=30001 \
   traefik/traefik
```

**If** Traefik Hub is **already** installed, we can instead upgrade the Traefik Hub instance:

```shell
# Upgrade CRDs
kubectl apply --server-side --force-conflicts -k https://github.com/traefik/traefik-helm-chart/traefik/crds/
# Update the Helm repository
helm repo update
# Upgrade the Helm chart
helm upgrade traefik-hub -n traefik-hub --wait \
  --set hub.token=license \
  --set ingressRoute.dashboard.matchRule='Host(`dashboard.docker.localhost`)' \
  --set ingressRoute.dashboard.entryPoints={web} \
  --set image.registry=ghcr.io \
  --set image.repository=traefik/traefik-hub \
  --set image.tag=v3.0.0 \
  --set ports.web.nodePort=30000 \
  --set ports.websecure.nodePort=30001 \
   traefik/traefik
```

Now, we can access the local dashboard: http://dashboard.docker.localhost/

## Deploy an API without Traefik Hub

Without Traefik Hub, an API can be deployed with an `Ingress`, an `IngressRoute` or an `HTTPRoute`.

In this tutorial, APIs are implemented using a JSON server in Go; the source code is [here](../../src/api-server/).

Let's deploy a [weather app](../../src/manifests/weather-app.yaml) exposing an API.

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

Get the token from https://jwt.io using the same signing secret or get one with command line:

```bash
jwt_header=$(echo -n '{"alg":"HS256","typ":"JWT"}' | base64 | sed s/\+/-/g | sed 's/\//_/g' | sed -E s/=+$//)
payload=$(echo -n '{"sub": "123456789","name":"John Doe","iat":'$(date +%s)'}' | base64 | sed s/\+/-/g |sed 's/\//_/g' |  sed -E s/=+$//)
secret='JWT on Traefik Hub!'
hexsecret=$(echo -n "$secret" | od -A n -t x1  | sed 's/ *//g' | tr -d '\n')
hmac_signature=$(echo -n "${jwt_header}.${payload}" |  openssl dgst -sha256 -mac HMAC -macopt hexkey:$hexsecret -binary | base64  | sed s/\+/-/g | sed 's/\//_/g' | sed -E s/=+$//)
export JWT_TOKEN="${jwt_header}.${payload}.${hmac_signature}"
```

![JWT Token](../../src/images/jwt-token.png)

With this token, we can test it:

```shell
# This call is not authorized => 401
curl -I http://api.docker.localhost/weather
# Let's set the token
export JWT_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.AuyxLr6YEAIdMxXujJ2icNvMCamR1SizrunWlyfLlJw"
# This call with the token is allowed => 200
curl -I -H "Authorization: Bearer $JWT_TOKEN" http://api.docker.localhost/weather
```

## On Linux

This tutorial will show how to use Traefik Hub on Linux. It's using simple shell code for simplicity. In production, we recommend to use Infra-as-Code or even GitOps.

:information_source: We will use a Debian Linux in this tutorial.

First, clone this GitHub repository:

```shell
git clone https://github.com/traefik/hub.git
cd hub
```

After, we'll need to get the Traefik Hub binary:

```shell
curl -L https://github.com/traefik/hub/releases/download/v3.0.1/traefik-hub_v3.0.1_linux_amd64.tar.gz -o /tmp/traefik-hub.tar.gz
tar xvzf /tmp/traefik-hub.tar.gz -C /tmp traefik-hub
rm -f /tmp/traefik-hub.tar.gz
sudo mv traefik-hub /usr/local/bin/traefik-hub
```

Now, we can move it to a binary `PATH` folder and set the expected rights on it:

```shell
sudo chown root:root /usr/local/bin/traefik-hub
sudo chmod 755 /usr/local/bin/traefik-hub
# Give the Traefik Hub binary ability to bind privileged ports like 80 or 443 as non-root
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/traefik-hub
```

Finally, we can create the config resources:

```shell
# Create a user
sudo groupadd ---system traefik-hub
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

Log in to the [Traefik Hub Online Dashboard](https://hub.traefik.io), open the page to [generate a new gateway](https://hub.traefik.io/agents/new).

**:warning: Do not install the gateway, but copy the token.**

Export your token:

```shell
export TRAEFIK_HUB_TOKEN=SET_YOUR_TOKEN_HERE
```

With this token, we can add a [static configuration file](linux/traefik-hub.toml) for Traefik Hub and a [systemd service](linux/traefik-hub.service):

```shell
sudo cp api-gateway/1-getting-started/linux/traefik-hub.toml /etc/traefik-hub/traefik-hub.toml
sudo sed -i -e "s/PASTE_YOUR_TOKEN_HERE/$TRAEFIK_HUB_TOKEN/g" /etc/traefik-hub/traefik-hub.toml
sudo cp api-gateway/1-getting-started/linux/traefik-hub.service /etc/systemd/system/traefik-hub.service
sudo chown root:root /etc/systemd/system/traefik-hub.service
sudo chmod 644 /etc/systemd/system/traefik-hub.service
sudo systemctl daemon-reload
sudo systemctl enable --now traefik-hub.service
```

We can check it is running with the following command:

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

On Linux, we can use all the providers supported by Traefik Proxy and all the providers supported by Traefik Hub.

Let's begin with a simple file provider.

We will deploy a simple _whoami_ app on systemd and try to reach it from Traefik Proxy.

```shell
# Install whoami
curl -L https://github.com/traefik/whoami/releases/download/v1.10.2/whoami_v1.10.2_linux_amd64.tar.gz -o /tmp/whoami.tar.gz
tar xvzf /tmp/whoami.tar.gz -C /tmp whoami
rm -f /tmp/whoami.tar.gz
sudo mv whoami /usr/local/bin/whoami
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

We will enable this app with a [systemd unit file](linux/whoami.service):

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

Now, we can add a [simple dynamic configuration file](linux/whoami.yaml) to expose it with Traefik Hub.

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

## Secure authentication using JWTs with Traefik Hub

Now, let's try to secure its access with a JWT token.

```diff
diff -Nau api-gateway/1-getting-started/linux/whoami.yaml api-gateway/1-getting-started/linux/whoami-jwt.yaml
--- api-gateway/1-getting-started/linux/whoami.yaml
+++ api-gateway/1-getting-started/linux/whoami-jwt.yaml
@@ -3,6 +3,14 @@
     whoami:
       rule: Host(`whoami.localhost`)
       service: local
+      middlewares:
+      - jwtAuth
+
+  middlewares:
+    jwtAuth:
+      plugin:
+        jwt:
+          signingSecret: "JWT on Traefik Hub!"

   services:
     local:

```

Let's apply it:

```shell
sudo cp api-gateway/1-getting-started/linux/whoami-jwt.yaml /etc/traefik-hub/dynamic/whoami.yaml
sleep 5
```

Get the token from https://jwt.io using the same signing secret or get one with command line:

```bash
jwt_header=$(echo -n '{"alg":"HS256","typ":"JWT"}' | base64 | sed s/\+/-/g | sed 's/\//_/g' | sed -E s/=+$//)
payload=$(echo -n '{"sub": "123456789","name":"John Doe","iat":'$(date +%s)'}' | base64 | sed s/\+/-/g |sed 's/\//_/g' |  sed -E s/=+$//)
secret='JWT on Traefik Hub!'
hexsecret=$(echo -n "$secret" | od -A n -t x1  | sed 's/ *//g' | tr -d '\n')
hmac_signature=$(echo -n "${jwt_header}.${payload}" |  openssl dgst -sha256 -mac HMAC -macopt hexkey:$hexsecret -binary | base64  | sed s/\+/-/g | sed 's/\//_/g' | sed -E s/=+$//)
export JWT_TOKEN="${jwt_header}.${payload}.${hmac_signature}"
```

![JWT Token](../../src/images/jwt-token.png)

With this token, we can test it:

```shell
# This call is not authorized => 401
curl -I http://whoami.localhost
# Let's set the token
export JWT_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.AuyxLr6YEAIdMxXujJ2icNvMCamR1SizrunWlyfLlJw"
# This call with the token is allowed => 200
curl -I -H "Authorization: Bearer $JWT_TOKEN" http://whoami.localhost
```

## Docker providers

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

Now we can test the service with a simple [docker compose](linux/docker-compose.yaml) file:

```shell
sudo docker-compose -f $(pwd)/api-gateway/1-getting-started/linux/docker-compose.yaml up -d
```

Since we already enabled the docker provider in Traefik Hub configuration, we should now be able to curl it:

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
