## Machine to Machine (M2M) authentication with OAuth2 Client Credentials

The OAuth2 Client Credentials (defined in [OAuth 2.0 RFC 6749, section 4.4](https://tools.ietf.org/html/rfc6749#section-4.4)) protocol provides a way to secure delegated access between applications with a JWT access token.

With Traefik Hub, the authentication can be done between applications on the app level or on the gateway level.

:information_source: To follow this tutorial, you'll need to install Traefik Hub following [getting started](../1-getting-started/README.md) instructions.

Now that Traefik Hub is deployed, let's see how it works on the app level, using OAuth 2 Client Credentials flow:

```mermaid
---
title: OAuth2 Client Credentials flow
---
sequenceDiagram
    actor app as M2M Application
    participant apigw as Traefik Hub<br>API Gateway
    participant idp as Authorization<br>Server
    participant api as Application
    autonumber

    app->>apigw: Authenticate with Application Credentials
    apigw->>idp: Validate Application Credentials. Request access token
    idp->>apigw: Issue access token
    apigw->>api: Send request with access token in HTTP Header
```

In this tutorial, we will use [Ory Hydra](https://www.ory.sh/hydra/), an OAuth 2 and OpenID Connect server. We will also use a simple [login/consent app](https://github.com/jlevesy/hail-hydra) that always says yes.

We can deploy it:

```shell
kubectl apply -f api-gateway/2-secure-applications/manifests/hydra.yaml
kubectl wait -n hydra --for=condition=ready pod --selector=app=hydra --timeout=90s
kubectl wait -n hydra --for=condition=ready pod --selector=app=consent --timeout=90s
```

TraefikLabs has open-sourced a simple _whoami_ application displaying technical information about the request.

First, let's deploy and expose it:

```shell
kubectl apply -f src/manifests/whoami-app.yaml
kubectl apply -f src/manifests/whoami-app-ingressroute.yaml
sleep 5
```

```shell
namespace/apps created
deployment.apps/whoami created
service/whoami created
ingressroute.traefik.io/whoami
```

It should be accessible with curl on http://whoami.docker.localhost/

```shell
curl http://whoami.docker.localhost/
```

```shell
Hostname: whoami-697f8c6cbc-8wqq8
IP: 127.0.0.1
IP: ::1
IP: 10.244.0.9
IP: fe80::7c41:ceff:fe38:23e7
RemoteAddr: 10.244.0.10:41222
GET / HTTP/1.1
Host: whoami.docker.localhost
User-Agent: curl/7.88.1
Accept: */*
Accept-Encoding: gzip
X-Forwarded-For: 10.244.0.1
X-Forwarded-Host: whoami.docker.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Forwarded-Server: traefik-hub-54fc878695-8r696
X-Real-Ip: 10.244.0.1
```

To secure it with OAuth2, we can use the OAuth2 Client Credentials middleware:

```diff
--- src/manifests/whoami-app-ingressroute.yaml
+++ api-gateway/2-secure-applications/manifests/whoami-app-oauth2.yaml
@@ -1,5 +1,20 @@
 ---
 apiVersion: traefik.io/v1alpha1
+kind: Middleware
+metadata:
+  name: oauth2-creds
+  namespace: apps
+spec:
+  plugin:
+    oAuthClientCredentials:
+      url: http://hydra.hydra.svc:4444/oauth2/token
+      audience: https://traefik.io
+      usernameClaim: sub
+      forwardHeaders:
+        Sub: sub
+
+---
+apiVersion: traefik.io/v1alpha1
 kind: IngressRoute
 metadata:
   name: whoami
@@ -13,3 +28,5 @@
     services:
     - name: whoami
       port: 80
+    middlewares:
+    - name: oauth2-creds
```

We can deploy the secured `IngressRoute`:

```shell
kubectl apply -f api-gateway/2-secure-applications/manifests/whoami-app-oauth2.yaml
```

```shell
middleware.traefik.io/oauth2-creds created
ingressroute.traefik.io/whoami configured
```

Once it's ready, we can create a Hydra OAuth2 client with _client_credentials_ grant type and test the authentication with it:

```shell
client=$(kubectl exec -it -n hydra deploy/hydra -- \
  hydra create oauth2-client --name oauth-client --secret traefiklabs \
    --grant-type client_credentials --endpoint http://127.0.0.1:4445/ \
    --audience https://traefik.io --format json \
    --token-endpoint-auth-method client_secret_post)
echo $client | jq -r '{ "client_id": .client_id, "client_secret": .client_secret }'
client_id=$(echo $client | jq -r '.client_id')
client_secret=$(echo $client | jq -r '.client_secret')
auth=$(echo -n "$client_id:$client_secret" | base64 -w 0)
curl -H "Authorization: Basic $auth" whoami.docker.localhost
```

It should output something like this:

```shell
{
  "client_id": "4a3265b7-ac47-4f84-b7ad-dffa4f907ffb",
  "client_secret": "traefiklabs"
}
Hostname: whoami-697f8c6cbc-kbtls
IP: 127.0.0.1
IP: ::1
IP: 10.244.0.19
IP: fe80::a8a4:1aff:fe79:dd55
RemoteAddr: 10.244.0.20:37702
GET / HTTP/1.1
Host: whoami.docker.localhost
User-Agent: curl/7.88.1
Accept: */*
Accept-Encoding: gzip
Authorization: Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6ImQ4ZDVkNTNhLWNiNmEtNDhlMi1iMzUxLTA0ZTc3N2U0Njk2YiIsInR5cCI6IkpXVCJ9.eyJhdWQiOlsiaHR0cHM6Ly90cmFlZmlrLmlvIl0sImNsaWVudF9pZCI6IjRhMzI2NWI3LWFjNDctNGY4NC1iN2FkLWRmZmE0ZjkwN2ZmYiIsImV4cCI6MTcxNTg3NDg0NSwiZXh0Ijp7fSwiaWF0IjoxNzE1ODcxMjQ1LCJpc3MiOiJodHRwOi8vaHlkcmE6NDQ0NCIsImp0aSI6ImIxYzI5ODFiLWViZmUtNDZlMS04ZDQ2LTY1NjE0N2RlMzUzNCIsIm5iZiI6MTcxNTg3MTI0NSwic2NwIjpbXSwic3ViIjoiNGEzMjY1YjctYWM0Ny00Zjg0LWI3YWQtZGZmYTRmOTA3ZmZiIn0.fXt8SslskPIjjgh9gYfC2sjkfjDpjZ_n3m2oQ5EG1PqPHdVkV2kcc00R3bJXXi5kabNRoIvluJX4K_kx93w6Q8OoZ-MCO0TnkitzRduVVw6QqfkCDCWh7v2bLv3Kev6e_U3jmMcXXWkS7CA2tJycSkficK_wIUvMyO2RCDoChwvk_xDFPHf9zmnOGPEJsppMK16wMfE1MYz67sPiLGnU3x3pKIlyTLd7FsPw-GqpXzC1hbHEvoL3cmjCpHm4AQ1ksxJct0fI70Gd2UDSA6j4j0W5ma2fa8ugp6pCHVOnuvNjlfFWod0lX7Izff9RatWpakFlkzNH3JgmdnYwSbnYUuukIGVYaggZNWsu8jaX5jT0nXK_F-sIpuO5shRVNfT3hQUmf2pw_lOZC10p8ZKxzS91rG-8haWGfwJvbtft7LMSs47300832y1PNcri8PS5n9oCh4yNF94WgEwrqKU-iRUkd73SKAYVcFDtU7HeQ9yN0e_Itg6GpbnzgWMxKpLfT-iYV3TuX42S-wimcvxSmB22_IfVbXlwtJOi7IijkJpGcetcShz6R8CHu9upK-78IVQa_eRhw9VAwx2iTLrkjr8Q6xdxCGd1ajSvjQOahlDIO7Va1vxUsZXRNjHRUCSP1vKGJVcXI6xGG5r2wt_87BYsH6d3pLFmd89Q_MfAWn0
Sub: 4a3265b7-ac47-4f84-b7ad-dffa4f907ffb
X-Forwarded-For: 10.244.0.1
X-Forwarded-Host: whoami.docker.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Forwarded-Server: traefik-hub-54458bcb46-bwwvc
X-Real-Ip: 10.244.0.1
```

As we can see:

1. A JWT Access Token is in the bearer HTTP header
2. The `Sub` header of the authentication has been added to the request.

We can decode this JWT token easily on https://jwt.io

![Decode JWT Access Token](./images/oauth2-decode.png)

There is an other mode allowed with this middleware, where we set _ClientId_ and _ClientSecret_ directly on Traefik Hub. Let's try it out.

```mermaid
---
title: OAuth2 Client Credentials flow with authentication on Traefik Hub
---
sequenceDiagram
    actor app as M2M Application
    participant apigw as Traefik Hub<br>API Gateway
    participant idp as Authorization<br>Server
    participant api as Application
    autonumber

    app->>apigw: Send request
    apigw->>idp: Validate request. Request access token using Traefik Hub credentials
    idp->>apigw: Issue access token
    apigw->>api: Send request with access token in HTTP Header
```

We will remove the forward headers block and set the client's credentials directly in the middleware. We are using environment variables to substitute them according to your credentials to generate the applied configuration.

```diff
diff -Nau api-gateway/2-secure-applications/manifests/whoami-app-oauth2.yaml api-gateway/2-secure-applications/manifests/whoami-app-oauth2-client-creds.yaml
--- api-gateway/2-secure-applications/manifests/whoami-app-oauth2.yaml
+++ api-gateway/2-secure-applications/manifests/whoami-app-oauth2-client-creds.yaml
@@ -9,9 +9,8 @@
     oAuthClientCredentials:
       url: http://hydra.hydra.svc:4444/oauth2/token
       audience: https://traefik.io
-      usernameClaim: sub
-      forwardHeaders:
-        Sub: sub
+      clientId: $CLIENT_ID
+      clientSecret: $CLIENT_SECRET
```

Let's try it with a new user:

```shell
client=$(kubectl exec -it -n hydra deploy/hydra -- \
  hydra create oauth2-client --name oauth-client --secret traefiklabs \
    --grant-type client_credentials --endpoint http://127.0.0.1:4445/ \
    --audience https://traefik.io --format json)
echo $client | jq -r '{ "client_id": .client_id, "client_secret": .client_secret }'
export CLIENT_ID=$(echo $client | jq -r '.client_id')
export CLIENT_SECRET=$(echo $client | jq -r '.client_secret')
cat api-gateway/2-secure-applications/manifests/whoami-app-oauth2-client-creds.yaml | envsubst | kubectl apply -f -
sleep 2
curl whoami.docker.localhost
```

As we can see, there is no authentication required now *and* there is a JWT access token transmitted to the application:

```shell
{
  "client_id": "bcd19bc4-240d-4a1c-93fa-64f2818590f8",
  "client_secret": "traefiklabs"
}
middleware.traefik.io/oauth2-creds configured
ingressroute.traefik.io/whoami unchanged
Hostname: whoami-697f8c6cbc-kbtls
IP: 127.0.0.1
IP: ::1
IP: 10.244.0.19
IP: fe80::a8a4:1aff:fe79:dd55
RemoteAddr: 10.244.0.20:55802
GET / HTTP/1.1
Host: whoami.docker.localhost
User-Agent: curl/7.88.1
Accept: */*
Accept-Encoding: gzip
Authorization: Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6ImQ4ZDVkNTNhLWNiNmEtNDhlMi1iMzUxLTA0ZTc3N2U0Njk2YiIsInR5cCI6IkpXVCJ9.eyJhdWQiOlsiaHR0cHM6Ly90cmFlZmlrLmlvIl0sImNsaWVudF9pZCI6ImJjZDE5YmM0LTI0MGQtNGExYy05M2ZhLTY0ZjI4MTg1OTBmOCIsImV4cCI6MTcxNTg3NjE2NywiZXh0Ijp7fSwiaWF0IjoxNzE1ODcyNTY3LCJpc3MiOiJodHRwOi8vaHlkcmE6NDQ0NCIsImp0aSI6IjVjZmUyMWQ3LWZkMGQtNDE5Ny05YzBlLWNhZDMwYTk2ZmYzNiIsIm5iZiI6MTcxNTg3MjU2Nywic2NwIjpbXSwic3ViIjoiYmNkMTliYzQtMjQwZC00YTFjLTkzZmEtNjRmMjgxODU5MGY4In0.JCMmt_RQiJuzdxJS0DLFsMt8jX3zDiHsD0iQnm3k4T-nCvUAev3ij4D0zeuhnXSmCs7Xr69gJkmbbGLemc19huioklPja4mLgKyxbU_lr3ua7n66nMWt5miq4VXk10xsUAJzB4tqbZMfX_GWL66u1XD8G3kgFN5JZ5zVVUmYFfF6sYT-bsOOfJcHWORlMNJUniIat-GaobkapW_knH38AU7JMiXxoek4GjB3rthXGVtyKQpsY1azmJ6LBx5_pWEiOjvq5pne3qdogTTeCsl9u9KMZuOVIPCYS3NH34ILgx4NbMZgD_PvU1D_encRuFMYlu2rNsWFdW6aAIh_RbfxlQZm1T-MjghTr0QJT8e4d6SLGWB_AUUnR7ANxUmZjN-lLrv_G0tiLKYJfx6HTRhejGxR13xrUKYr8HknxysUMjexXqVgwlkML8l13xPHBJQddIeDUcqCrVTPWq4BzYOFVTGFOrJ24vqpGeCQPWGuv59DcanYYsOYpxrFbXlrjgjX12q6ONo5Oi94_hgo-wKWfS0yjuirsnO8Rpcl47J03lvwjhqbNKUFBfPqWJaSYukVhfgZc4aTD0xVc4GiHVgy6x6MkiuI8FL62EF1MQEhQrWD-1KF_siYDL0mAW0NvcxgA4aAxtNwV71brIjfHQejtwZ0Sk-OJnFvsJ0iHoMq7Rg
X-Forwarded-For: 10.244.0.1
X-Forwarded-Host: whoami.docker.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Forwarded-Server: traefik-hub-54458bcb46-bwwvc
X-Real-Ip: 10.244.0.1
```

This mode is clearly less secure. Nevertheless, it's possible to secure requests between applications and Traefik Hub using other techniques (not exhaustive): IP restriction with `IPAllowList`, other authentication mechanisms like `BasicAuth`, or using mTLS.
