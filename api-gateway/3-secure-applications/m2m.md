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
kubectl apply -f src/manifests/apps-namespace.yaml
kubectl apply -f src/manifests/hydra.yaml
kubectl wait -n hydra --for=condition=ready pod --selector=app=hydra --timeout=90s
kubectl wait -n hydra --for=condition=ready pod --selector=app=consent --timeout=90s
kubectl wait -n hydra --for=condition=complete  job/create-hydra-clients --timeout=90s
```

TraefikLabs has open-sourced a simple _whoami_ application displaying technical information about the request.

First, let's deploy and expose it:

```shell
kubectl apply -f src/manifests/whoami-app.yaml
kubectl apply -f api-gateway/3-secure-applications/manifests/whoami-app-ingressroute.yaml
sleep 5
```

```shell
deployment.apps/whoami created
service/whoami created
ingressroute.traefik.io/secure-applications-apigateway-no-auth created
```

It should be accessible with curl on http://secure-applications.apigateway.docker.localhost/no-auth

```shell
curl http://secure-applications.apigateway.docker.localhost/no-auth
```

```shell
Hostname: whoami-6f57d5d6b5-bgmfl
IP: 127.0.0.1
IP: ::1
IP: 10.42.0.10
IP: fe80::c8f6:84ff:fe66:3158
RemoteAddr: 10.42.0.6:38110
GET /no-auth HTTP/1.1
Host: secure-applications.apigateway.docker.localhost
User-Agent: curl/8.5.0
Accept: */*
Accept-Encoding: gzip
X-Forwarded-For: 10.42.0.1
X-Forwarded-Host: secure-applications.apigateway.docker.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Forwarded-Server: traefik-hub-6f5bbd6568-rp882
X-Real-Ip: 10.42.0.1
```

To secure it with OAuth2, we can use the OAuth2 Client Credentials middleware:

```diff :../../hack/diff.sh -r -a "manifests/whoami-app-ingressroute.yaml manifests/whoami-app-oauth2-client-creds.yaml"
--- manifests/whoami-app-ingressroute.yaml
+++ manifests/whoami-app-oauth2-client-creds.yaml
@@ -1,15 +1,32 @@
 ---
 apiVersion: traefik.io/v1alpha1
+kind: Middleware
+metadata:
+  name: oauth2-client-creds
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
-  name: secure-applications-apigateway-no-auth
+  name: secure-applications-apigateway-oauth2-client-credentials
   namespace: apps
 spec:
   entryPoints:
     - web
   routes:
-  - match: Host(`secure-applications.apigateway.docker.localhost`) && Path(`/no-auth`)
+  - match: Host(`secure-applications.apigateway.docker.localhost`) && Path(`/oauth2-client-credentials`)
     kind: Rule
     services:
     - name: whoami
       port: 80
+    middlewares:
+    - name: oauth2-client-creds
```

We can deploy the secured `IngressRoute`:

```shell
kubectl apply -f api-gateway/3-secure-applications/manifests/whoami-app-oauth2-client-creds.yaml
```

```shell
middleware.traefik.io/oauth2-client-creds created
ingressroute.traefik.io/secure-applications-apigateway-oauth2-client-credentials created
```

Once it's ready, we can create a Hydra OAuth2 client with _client_credentials_ grant type.
This step is automated for you in this tutorial but here is how it's created:

```shell :../../src/manifests/hydra.yaml -s 359 -e 366 -i s1
hydra create oauth2-client \
  --endpoint http://hydra.hydra.svc:4445 \
  --name oauth-client \
  --secret traefiklabs \
  --grant-type client_credentials \
  --audience https://traefik.io \
  --token-endpoint-auth-method client_secret_post \
  --format json > /data/oauth-client.json
```

If needed, it can be run manually with `kubectl exec -it -n hydra deploy/hydra -- hydra create ...`.

```shell
client_id=$(kubectl get secrets -n apps oauth-client -o json | jq -r '.data.client_id' | base64 -d -w 0)
client_secret=$(kubectl get secrets -n apps oauth-client -o json | jq -r '.data.client_secret' | base64 -d -w 0)
auth=$(echo -n "$client_id:$client_secret" | base64 -w 0)
curl -H "Authorization: Basic $auth" http://secure-applications.apigateway.docker.localhost/oauth2-client-credentials
```

It should output something like this:

```shell
Hostname: whoami-6f57d5d6b5-bgmfl
IP: 127.0.0.1
IP: ::1
IP: 10.42.0.10
IP: fe80::c8f6:84ff:fe66:3158
RemoteAddr: 10.42.0.6:37870
GET /oauth2-client-credentials HTTP/1.1
Host: secure-applications.apigateway.docker.localhost
User-Agent: curl/8.5.0
Accept: */*
Accept-Encoding: gzip
Authorization: Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6IjU2ZDdkMzliLTdhNTUtNDFkZi1iNjZkLTRjYzU0YmQ1YmZmOCIsInR5cCI6IkpXVCJ9.eyJhdWQiOlsiaHR0cHM6Ly90cmFlZmlrLmlvIl0sImNsaWVudF9pZCI6ImZlZTU2Nzc1LWJhYjMtNDE1Mi1hZDM3LWUyMTE0YWRlNjQ0OSIsImV4cCI6MTcxODgxNzQ5MiwiZXh0Ijp7fSwiaWF0IjoxNzE4ODEzODkyLCJpc3MiOiJodHRwOi8vaHlkcmEuaHlkcmEuc3ZjOjQ0NDQiLCJqdGkiOiI3Y2NmNzAxNC04OWM0LTQ1OTYtYWFjNS0wNTdlZjIzYjVjNjkiLCJuYmYiOjE3MTg4MTM4OTIsInNjcCI6W10sInN1YiI6ImZlZTU2Nzc1LWJhYjMtNDE1Mi1hZDM3LWUyMTE0YWRlNjQ0OSJ9.AwT-3_XQvScKzcfK-HumGZn9AfD9BofzfMxraT4Nmvb7OPamkPwhn6i_hwYtvcxth0TUx6W4gziMX867rw3jPS_KZPeq33GYWkIlmVJbmE90jWcST7MOm5_Pl-KfmV9YKioWD1RFGGM3rkIrobmtH1JM3Oxbxi5bbcPOrdFGlpIiAst5V6LC8e93vwga9mvh86TCT7ZnaxVHNN5Rrz_KdkCnidpUcc5Vev1GlTGOyhK4uolqu7fyQiyckeSNGB_BLB-bk1JBPEApgPWRjKIXLwhmR-xg4WXFl3kWY4nBI7ECmbClMMCfpXa1zYWF_kjDHodWxL7n7dEsrsZuykXRTKT10iT2VxH7QrPS-lHOu_sg6svCCGObB_lkv0rHBP6P9K-nD3tkJi_uPbVEIFkzjxDe6CEIZ2Xrn8H1GVmig-NoNGpflMYVu41wb_6eRv-PPACD_GI-YQOQvpMJXPFIjMIUMmvIWg-vD0bzrd_YUipa0HfruP_ENnHeIXwhJBMCWeVNwGCslsSj8uO7KaTF1NrTDuIHZNBnAp2WxZFQ4RPC5O1T3TJOpPn7dj5PKN-XgUGQlmUCUGPazfvFFBFQymoDSL88ijXDTkbzYGD2TnrbcqyCWV6uJdyEoV1Q8OA_lGtN39XcUuyiMGavSGdQY5yyBoULXo8oAwprOIdAy68
Sub: fee56775-bab3-4152-ad37-e2114ade6449
X-Forwarded-For: 10.42.0.1
X-Forwarded-Host: secure-applications.apigateway.docker.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Forwarded-Server: traefik-hub-6f5bbd6568-rp882
X-Real-Ip: 10.42.0.1
```

As we can see:

1. A JWT Access Token is in the bearer HTTP header
2. The `Sub` header of the authentication has been added to the request.

We can decode this JWT token easily on https://jwt.io

![Decode JWT Access Token](./images/oauth2-decode.png)

There is another mode allowed with this middleware, where we set _ClientId_ and _ClientSecret_ directly on Traefik Hub. Let's try it out.

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

We will remove the forward headers block and set the client's credentials directly in the middleware.
Another client has been automatically created for that purpose. Let's see how:

```shell :../../src/manifests/hydra.yaml -s 368 -e 374 -i s1
hydra create oauth2-client \
  --endpoint http://hydra.hydra.svc:4445 \
  --name oauth-client-nologin \
  --secret traefiklabs \
  --grant-type client_credentials \
  --audience https://traefik.io \
  --format json > /data/oauth-client-nologin.json
```

```diff :../../hack/diff.sh -r -a "manifests/whoami-app-oauth2.yaml manifests/whoami-app-oauth2-client-creds-nologin.yaml"
--- manifests/whoami-app-oauth2.yaml
+++ manifests/whoami-app-oauth2-client-creds-nologin.yaml
@@ -0,0 +1,31 @@
+---
+apiVersion: traefik.io/v1alpha1
+kind: Middleware
+metadata:
+  name: oauth2-client-creds-nologin
+  namespace: apps
+spec:
+  plugin:
+    oAuthClientCredentials:
+      url: http://hydra.hydra.svc:4444/oauth2/token
+      audience: https://traefik.io
+      clientId: "urn:k8s:secret:oauth-client-nologin:client_id"
+      clientSecret: "urn:k8s:secret:oauth-client-nologin:client_secret"
+
+---
+apiVersion: traefik.io/v1alpha1
+kind: IngressRoute
+metadata:
+  name: secure-applications-apigateway-oauth2-client-credentials-nologin
+  namespace: apps
+spec:
+  entryPoints:
+    - web
+  routes:
+  - match: Host(`secure-applications.apigateway.docker.localhost`) && Path(`/oauth2-client-credentials-nologin`)
+    kind: Rule
+    services:
+    - name: whoami
+      port: 80
+    middlewares:
+    - name: oauth2-client-creds-nologin
```

Let's try it:

```shell
kubectl apply -f api-gateway/3-secure-applications/manifests/whoami-app-oauth2-client-creds-nologin.yaml
sleep 3
curl http://secure-applications.apigateway.docker.localhost/oauth2-client-credentials-nologin
```

As we can see, there is no authentication required now _and_ there is a JWT access token transmitted to the application:

```shell
Hostname: whoami-6f57d5d6b5-bgmfl
IP: 127.0.0.1
IP: ::1
IP: 10.42.0.10
IP: fe80::c8f6:84ff:fe66:3158
RemoteAddr: 10.42.0.6:50004
GET /oauth2-client-credentials-nologin HTTP/1.1
Host: secure-applications.apigateway.docker.localhost
User-Agent: curl/8.5.0
Accept: */*
Accept-Encoding: gzip
Authorization: Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6IjU2ZDdkMzliLTdhNTUtNDFkZi1iNjZkLTRjYzU0YmQ1YmZmOCIsInR5cCI6IkpXVCJ9.eyJhdWQiOlsiaHR0cHM6Ly90cmFlZmlrLmlvIl0sImNsaWVudF9pZCI6IjNkMzYwNGViLWNkOTItNDAxMi05ODlhLTAxMTY3Mjc3YzRmMSIsImV4cCI6MTcxODgxNzYyMiwiZXh0Ijp7fSwiaWF0IjoxNzE4ODE0MDIyLCJpc3MiOiJodHRwOi8vaHlkcmEuaHlkcmEuc3ZjOjQ0NDQiLCJqdGkiOiJmYTcwZDBlMy1hM2E4LTQ4ZDYtYmNmNi1mMGYyZDEwY2ZkYTUiLCJuYmYiOjE3MTg4MTQwMjIsInNjcCI6W10sInN1YiI6IjNkMzYwNGViLWNkOTItNDAxMi05ODlhLTAxMTY3Mjc3YzRmMSJ9.fVM8ba7cSLBC-2J5t72sp8bzbZF5hGZRvaiWsKeJQxqcslQrB05nzZHhKvkg5Bqsw-0wycFVvYuc_1DWRai0jE7mG73vgiTuuBe38NaAh4hVt9vTORzvKtm_V1Wx4ZAtniGB-6o5ta6TyLc68tq78KHUeWTzZ0f5ugteZVmrflvBgWQWWjM612op2nbA9m6ArDurWRst6FAvJ_lSja60XHjmHSan78Q9ps7PzH1PB2zkmrsaI6Y-c81CtMR6KOdBmO0iD4eRoHGh2GP0iCiozv9r_8pLc7xkdFBDeFoswGIVRQhqEvOLchE5Ca7DnI6PQupX8NtXRrPY-blS8d-WT4UwHVUOc_nEQHhZuIZk3IG7iE6JMmtc_0dOWdBlu5m-XVHe1mC5XSb55McuY0ckp2mb2pbPgQJCcra67prcqQqpXZc0syOCTlvjk6mHBXYMmiISIsunGttKmNuZKFteUiaPsqgRJf_B2JQvG4RknEk1Nl5VKr8ouneP0xunSCRCZyGScZ_qt5XPbhBqLOA4dedATWqtQ7UT8hp5TWOmE0_1bZM3CKSOQeX3aPQWA6NFsJHKxY2IZsohes_QXACCI2qfHa5CVueRzgBYPkECtWC1pXhBWy4E5p4Jp-2mMBvEpTjltcprndfaGbFHxOw1nj1p24ESES7P9IUK73DQca8
X-Forwarded-For: 10.42.0.1
X-Forwarded-Host: secure-applications.apigateway.docker.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Forwarded-Server: traefik-hub-6f5bbd6568-rp882
X-Real-Ip: 10.42.0.1
```
