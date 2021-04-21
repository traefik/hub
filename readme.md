# Alpha 1

The Kubernetes cluster has been created with [env-on-demand](https://github.com/traefik/env-on-demand/issues/327)

The kubeconfig is available in this [comment](https://github.com/traefik/env-on-demand/issues/327#issuecomment-823890725)

## Install ingress controller

```bash
kubectl apply -f ingress-haproxy
kubectl apply -f ingress-nginx
kubectl apply -f traefik
```

## Install whoami application

```bash
kubectl apply -f neo/manifests/whoami
```

## Create account on neo and create a cluster

https://traefiklabs-neo-ui.netlify.app/

- Create an account
- Create a new cluster
- Install the neo agent with the ui instructions

## Run query to simulate traffic on applications

```
hey https://whoami.haproxy.neo.demo.traefiklabs.tech
hey https://whoami.traefik.neo.demo.traefiklabs.tech
hey https://whoami.nginx.neo.demo.traefiklabs.tech
hey https://whoami.nginx.neo.demo.traefiklabs.tech/httpbin/status/409
hey https://whoami.nginx.neo.demo.traefiklabs.tech/httpbin/status/500
hey https://whoami.nginx.neo.demo.traefiklabs.tech/httpbin/status/200
```
