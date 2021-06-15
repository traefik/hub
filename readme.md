# UI team dev env

```bash
k3d cluster create --k3s-server-arg "--no-deploy=traefik" \
--agents="2" \
--port 80:80@loadbalancer \
--port 443:443@loadbalancer \
--port 8000:8000@loadbalancer \
--port 8443:8443@loadbalancer \
--port 9000:9000@loadbalancer \
--port 9443:9443@loadbalancer
```

## Install ingress controller

```bash
kubectl apply -f traefik
kubectl apply -f ingress-haproxy
kubectl apply -f ingress-nginx
```

## Install whoami application

```bash
kubectl apply -f whoami
```

## Connect to hub

[ui](https://hub.traefik.io) michael+alpha3@traefik.io/Gerald42

- Install the agent

```bash
kubectl create namespace hub-agent
helm repo add hub https://helm.traefik.io/hub
helm repo update
helm upgrade --install hub hub/hub --set token="2baf55b8-9655-4f4d-89e4-692c7bc4d7fc" --namespace hub-agent --set controllerDeployment.args="{--log-level=debug,--platform-url=https://platform-preview.hub.traefik.io/agent}"
```

## Run query to simulate traffic on applications

```
hey -host whoami.traefik.docker.localhost http://127.0.0.1:80
hey -host app.traefik.docker.localhost http://127.0.0.1:80
hey -host whoami.nginx.docker.localhost http://127.0.0.1:8000
hey -host whoami.haproxy.docker.localhost http://127.0.0.1:9000
hey -host whoami.nginx.docker.localhost http://127.0.0.1:8000/httpbin/status/409
hey -host whoami.nginx.docker.localhost http://127.0.0.1:8000/httpbin/status/500
hey -host whoami.nginx.docker.localhost http://127.0.0.1:8000/httpbin/status/200
```

## Clean up

```
helm uninstall --namespace hub-agent hub
kubectl delete ns hub-agent
kubectl delete mutatingwebhookconfigurations.admissionregistration.k8s.io hub
```
