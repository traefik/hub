# Neo

  - [K3D](#k3d)
    - [Prerequisites](#prerequisites)
    - [Installation](#installation)
  - [Postman](#postman)
  - [Manual installation](#manual-installation)
    - [Create the cluster](#create-the-cluster)
    - [Install Ingress Controllers](#install-ingress-controllers)
    - [Install Neo Agent](#install-neo-agent)
    - [Install demo application](#install-demo-application)
    - [Test application](#test-application)
    
**You can run the full stack of Neo with K3D or by manually install every piece of the puzzle.
We hardly recommend the K3D way of course.**

## K3D

### Prerequisites

You will need these binaries to use the script:
- jq
- gcloud
- k3d
- kubectl
- helm

You need to be logged in gcloud before running any script.

```bash
gcloud auth login
gcloud auth configure-docker
```

Before running the script, you need a `.env` file in the `neo` folder.
`Just copy the `.env.example` and fill it with your own credentials.

- `GCLOUD_EMAIL` => Your email address to connect to gcr.
- `GITHUB_ORG` => The organization where the repository will be created by the topology service.
- `GITHUB_TOKEN` => A github token with `repo:*` permissions.
- `AWS_CLIENT_ID` => A client ID for connection to AWS
- `AWS_CLIENT_SECRET` => A client secret ID for AWS
- `NEO_USERNAME` => Your username on Neo
- `NEO_PASSWORD` => Your password on Neo

The AWS secrets can be found in `keybase://team/containous.dev/neo/k3d.md`.
The Neo account can be found in `keybase://team/containous.dev/neo/auth0.md` (`JWT_PASSWORD` and `JWT_USER`).
The `GITHUB_TOKEN` need the following `repo` scope.

#### Mac User

On MacOs, you need to install `coreutils` for the script to work.

To resolve `*.docker.localhost`, you also need to add these hosts in your `/etc/hosts`:
```bash
127.0.0.1 platform.docker.localhost
127.0.0.1 webapp.docker.localhost
127.0.0.1 jaeger-ui.docker.localhost
127.0.0.1 prometheus.docker.localhost
127.0.0.1 grafana.docker.localhost
```

### Installation

The local installation can be done with `make run`. The script will create a k3d cluster and deploy the following objects:
- IngressControllers
  - Nginx
  - Haproxy
  - Traefik
- Whoami with one ingress per ingress controller
- Neo platform
  - MongoDB
  - Neo services:
    - metrics
    - organization
    - topology
    - alert
    - clusters
    - certificates
    - invitation
    - ui
    - token
    - notification
    - (+ an ingress to access to all the services)
- Neo-agent
- Jaeger
- Monitoring
  - Grafana
  - Prometheus 
- Pebble

There are several commands to renew secrets, clean, or speed up the deployment :

#### jwt

If you need to renew your jwt. You can just run this command :

```
make jwt
```

#### renew-gcr-token

If your gcr credentials expire, you need to renew them. You can just run this command :

```
make renew-gcr-token
```

#### renew-auth0-admin-token

If the organization service doesn't work as expected, and you get some auth0 errors logs, your token is probably expired.
You can renew it with this command:

```
make renew-auth0-admin-token
```

#### apply-coredns-conf

If you have some errors like this:
```
getaddrinfo ENOTFOUND platform.docker.localhost
```

You have to reapply the coredns configuration with `make apply-coredns-conf`.

#### clean

`make clean` won't delete the k3d cluster but will delete every component created with the `make run` command.

#### delete

`make delete` will delete the k3d cluster.

#### --adsl

`make run-adsl` allows docker to pull the images before starting the cluster.
We recommend running it instead of `make run` if your internet connection is a bit slow.

### Exposed Endpoints

- UI: https://webapp.docker.localhost/
- Jaeger: https://jaeger-ui.docker.localhost/
- Neo-APIs: http://platform.docker.localhost/
    - /agent 
    - /organization 
    - /topology
    - /cluster
    - /token
    - /notification
    - /alert
    - /certificates
    - /invitation
    
- Grafana: https://grafana.docker.localhost
- Prometheus: https://prometheus.docker.localhost/
    
## Postman

A Postman collection with multiple environments is available in this repo. Check out the dedicated [readme](/postman/readme.md).

## Manual installation

### Create the cluster

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

### Install ingress controllers

#### Ingress Nginx

```bash
kubectl apply -f neo/manifests/ingress-nginx/
```
cf: https://kubernetes.github.io/ingress-nginx/deploy/#installation-guide

#### HaProxy

```bash
kubectl apply -f neo/manifests/ingress-haproxy
```

cf: https://haproxy-ingress.github.io/docs/getting-started/

#### Traefik

```bash
kubectl apply -f neo/manifests/traefik/
```
cf: https://doc.traefik.io/traefik/user-guides/crd-acme/

### Install Neo Agent

First you need to create a secret:
```bash
kubectl create secret -n $namespace docker-registry gcr-access-token \
                --docker-server=gcr.io \
                --docker-username=oauth2accesstoken \
                --docker-password="$(gcloud auth print-access-token)" \
                --docker-email=${GCLOUD_EMAIL}
```

We recommend overwriting the values file with this values file if you don't need to run neo-services locally:


```yaml
# Default values for neo-helm-chart.
image:
  name: gcr.io/traefiklabs/neo-agent
  pullPolicy: IfNotPresent
  pullSecrets:
    - name: gcr-access-token
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
    - --topology-info=traefik=whoami/whoami
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

##### Deploying Neo by overwriting values.yaml

```bash
helm install neo neo/neo --values=./values.yaml
```

##### Deploying Neo in a specific namespace

```bash
helm install neo neo/neo --namespace neo
```

##### Deploying Neo with a full-yaml

```bash
kubectl apply -f https://traefik.github.io/neo-helm-chart/yaml/0.1.1.yaml
```

##### Launch unit tests

You need to install the helm-plugin [unittest](https://github.com/rancher/unittest)

Then:

```bash
helm unittest neo/
```

##### Uninstall

We consider in this example the version install being <neo>:

```bash
helm uninstall neo
```

If neo-agent was installed in a specific namespace

```bash
helm uninstall neo --namespace neo-namespace
```

### Install demo application

```bash
kubectl apply -f neo/manifests/whoami/
```

### Test application

- HaProxy
```console
$ curl -H "Host: haproxy.docker.localhost" http://127.0.0.1:8000
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
```console
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
```console
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
