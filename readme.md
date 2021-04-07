# Neo

Create the kubernetes cluster with k3d

```bash
k3d cluster create --k3s-server-arg "--no-deploy=traefik" \
--agents="2" \
--image="rancher/k3s:v1.20.2-k3s1" \
--port 80:80@loadbalancer \
--port 443:443@loadbalancer \
--port 8000:8000@loadbalancer \
--port 8443:8443@loadbalancer \
--port 9000:9000@loadbalancer \
--port 9443:9443@loadbalancer

k3d image import gcr.io/traefiklabs/neo-agent:latest
```

Available docker images:
- rancher/k3s:v1.20.2-k3s1
- rancher/k3s:v1.19.7-k3s1
- rancher/k3s:v1.18.15-k3s1
- rancher/k3s:v1.17.17-k3s1
- rancher/k3s:v1.16.15-k3s1

## Install ingress controllers

### Ingress Nginx

```bash
kubectl apply -f ingress-nginx/
```
cf: https://kubernetes.github.io/ingress-nginx/deploy/#installation-guide

### Ingress Nginx Inc

```bash
kubectl apply -f ingress-nginx-inc/crds
kubectl apply -f ingress-nginx-inc/
```

cf: https://docs.nginx.com/nginx-ingress-controller/installation/installation-with-manifests/

### Traefik

```bash
kubectl apply -f traefik/
```
cf: https://doc.traefik.io/traefik/user-guides/crd-acme/

### Install Neo

We recommand to overwrite the values file with this values file if you don't need to run neo-services locally:

```yaml
# Default values for neo-helm-chart.
image:
  name: gcr.io/traefiklabs/neo-agent
  pullPolicy: IfNotPresent
  # Overrides the image tag whose default is the chart appVersion.
  tag: "2e63cbf"

# User token to access to neo
token: "4a585aab-f00e-4548-8528-222ef086bebb"

deployment:
  args:
    - --log-level=debug
    - --platform-url=https://platform.neo.traefiklabs.tech
    - --scrape-ip=http://10.42.0.36:8080/metrics
    - --scrape-name=traefik
    - --scrape-kind=traefik
```

Add Neo's chart repository to Helm:

```bash
helm repo add neo https://helm.traefik.io/neo
```

You can update the chart repository by running:

```bash
helm repo update
```

#### Deploying Neo

```bash
helm install neo neo/neo
```

#### Deploying Neo by overwriting values.yaml

```bash
helm install neo neo/neo --values=./values.yaml
```

#### Deploying Neo in a specific namespace

```bash
helm install neo neo/neo --namespace neo
```

#### Deploying Neo with a full-yaml

```bash
kubectl apply -f https://traefik.github.io/neo-helm-chart/yaml/0.1.1.yaml
```

#### Launch unit tests

You need to install the helm-plugin [unittest](https://github.com/rancher/unittest)

Then:

```bash
helm unittest neo/
```

#### Uninstall

We consider in this example the version install being <neo>:

```bash
helm uninstall neo
```

If neo-agent was install in a specific namespace

```bash
helm uninstall neo --namespace neo-namespace
```

## Install demo application

```bash
kubectl apply -f whoami/
```

## Test application

- Nginx inc
```bash
$ curl -H "Host: nginx-inc.docker.localhost" http://127.0.0.1:8000
Hostname: app-v1-9bb4bd54d-64gkk
IP: 127.0.0.1
IP: ::1
IP: 10.42.0.13
IP: fe80::e441:17ff:fe21:e48
RemoteAddr: 10.42.1.9:52586
GET / HTTP/1.1
Host: nginx-inc.docker.localhost
User-Agent: curl/7.64.1
Accept: */*
Connection: close
X-Forwarded-For: 10.42.1.11
X-Forwarded-Host: nginx-inc.docker.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Real-Ip: 10.42.1.11
```

- Nginx k8s
```bash
$ curl -H "Host: nginx-k8s.docker.localhost" http://127.0.0.1:9000
Hostname: app-v1-9bb4bd54d-p6zxb
IP: 127.0.0.1
IP: ::1
IP: 10.42.0.14
IP: fe80::3074:a9ff:fe5f:8ea4
RemoteAddr: 10.42.1.13:44084
GET / HTTP/1.1
Host: nginx-k8s.docker.localhost
User-Agent: curl/7.64.1
Accept: */*
X-Forwarded-For: 192.168.32.3
X-Forwarded-Host: nginx-k8s.docker.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Real-Ip: 192.168.32.3
X-Request-Id: 257daa1a28c2eba470dc5f9f8a5f61dd
X-Scheme: http
```

- Traefik
```bash
$ curl -H "Host: traefik.docker.localhost" http://127.0.0.1/
Hostname: app-v1-9bb4bd54d-p6zxb
IP: 127.0.0.1
IP: ::1
IP: 10.42.0.14
IP: fe80::3074:a9ff:fe5f:8ea4
RemoteAddr: 10.42.1.14:41604
GET / HTTP/1.1
Host: traefik.docker.localhost
User-Agent: curl/7.64.1
Accept: */*
Accept-Encoding: gzip
X-Forwarded-For: 10.42.1.12
X-Forwarded-Host: traefik.docker.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Forwarded-Server: traefik-78b84dc55f-8f25x
X-Real-Ip: 10.42.1.12
```
