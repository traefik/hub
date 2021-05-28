# Alpha 1

The Kubernetes cluster has been created with [env-on-demand](https://github.com/traefik/env-on-demand/issues/356)

The kubeconfig is available in this [comment](https://github.com/traefik/env-on-demand/issues/356#issuecomment-849664409)

## Install ingress controller

```bash
kubectl apply -f ingress-haproxy
kubectl apply -f ingress-nginx
kubectl apply -f traefik
```

## Install whoami application

```bash
kubectl apply -f whoami
```

## Create account on neo and create a cluster

[ui](https://hub.traefiklabs.tech/)

- Create an account
- Create a new cluster
- Install the neo agent with the ui instructions

## Run query to simulate traffic on applications

```
hey https://whoami.haproxy.hub.demo.traefiklabs.tech
hey https://whoami.traefik.hub.demo.traefiklabs.tech
hey https://whoami.nginx.hub.demo.traefiklabs.tech
hey https://whoami.nginx.hub.demo.traefiklabs.tech/httpbin/status/409
hey https://whoami.nginx.hub.demo.traefiklabs.tech/httpbin/status/500
hey https://whoami.nginx.hub.demo.traefiklabs.tech/httpbin/status/200
```

## Clean up

```
helm uninstall --namespace neo-agent neo
kubectl delete ns neo-agent
kubectl delete mutatingwebhookconfigurations.admissionregistration.k8s.io neo
```
