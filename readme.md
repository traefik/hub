# Alpha 3

Two Kubernetes cluster have been created with [env-on-demand](https://github.com/traefik/env-on-demand)
- [blue](https://github.com/traefik/env-on-demand/issues/375)
- [green](https://github.com/traefik/env-on-demand/issues/376)


On each cluster we install an ingress controller and whoami

## Install ingress controller

```bash
kubectl apply -f traefik
```

## Install whoami application

```bash
kubectl apply -f whoami
```

## Connect to hub

[ui](https://hub.traefik.io) michael+alpha3@traefik.io/Gerald42

- Install the agent

```bash
# blue
helm upgrade --install hub hub/hub --set token="a9a7b210-a52f-41ba-be13-b049976debca" --namespace hub-agent

# green
helm upgrade --install hub hub/hub --set token="7985948d-f9a1-4157-b5c7-7ac2d98a0a63" --namespace hub-agent
```

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
