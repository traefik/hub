# Update
```bash
helm template \
  --repo https://kubernetes.github.io/ingress-nginx --create-namespace \
  --set fullnameOverride="ingress-nginx" \
  --set service.ports.http=9000 --set service.ports.https=9443 \
  --release-name \
  --namespace ingress-nginx ingress-nginx > hub/manifests/ingress-nginx/deploy.yaml
```