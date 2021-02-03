# Neo

Create the kubernetes cluster with k3d

```bash
k3d cluster create --k3s-server-arg "--no-deploy=traefik" \
--agents="2" \
--port 80:80@loadbalancer \
--port 443:443@loadbalancer \
--port 8000:8000@loadbalancer \
--port 8443:8443@loadbalancer \
--port 9000:9000@loadbalancer \
--port 9443:9443@loadbalancer

k3d image import gcr.io/traefiklabs/neo-agent:latest
```

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
$ curl -H "Host: traefik.docker.localhost" http://127.0.0.1
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
