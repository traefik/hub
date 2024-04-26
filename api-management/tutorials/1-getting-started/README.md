# Getting Started

In this tutorial, we will publish a simple, public API.

First, we will deploy the public app:

```shell
kubectl apply -f src/manifests/public-app.yaml
```

Second, we can declare the API:

```yaml
---
apiVersion: hub.traefik.io/v1alpha1
kind: API
metadata:
  name: public-api
  namespace: apps
spec:
  pathPrefix: "/weather/public"
  service:
    name: public-app
    port:
      number: 3000
```

```shell
kubectl apply -f tutorials/1-getting-started/manifests/public-api.yaml
```

And the Gateway where we want to publish it:

```yaml
apiVersion: hub.traefik.io/v1alpha1
kind: APIGateway
metadata:
  name: api-gateway
spec:
  customDomains: # Custom domains to reach the API Gateway
    - api.docker.localhost
```

```shell
kubectl apply -f tutorials/1-getting-started/manifests/api-gateway.yaml
```
