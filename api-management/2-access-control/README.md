# Access Control

In this tutorial, we will see how to control access to APIs.

## Simple access control

Let's try a simple example start. Let's say we want to give separate access between an (internal) admin user and an external user with a subscription.

```mermaid
---
title: Simple access control
---
graph LR
    admin-user[Admin User] --> admin-app(admin API)
    external-user[External User] --> weather-app(weather API)

 %% CSS
 classDef apis fill:#326ce5,stroke:#fff,stroke-width:4px,color:#fff;
 classDef users fill:#fff,stroke:#bbb,stroke-width:2px,color:#326ce5;
 class admin-user,external-user users;
 class admin-app,private-app apis;
```

First, we will deploy the _weather_ app and the _admin_ app:

```shell
kubectl apply -f src/manifests/weather-app.yaml
kubectl apply -f src/manifests/admin-app.yaml
```

To ensure isolation between access, there is a versatile `APIAccess` CRD, allowing the linking of user groups and APIs. So, let's declare the _admin_ API with its `APIAccess`:

```yaml
---
apiVersion: hub.traefik.io/v1alpha1
kind: API
metadata:
  name: admin-api
  namespace: admin
spec: {}

---
apiVersion: hub.traefik.io/v1alpha1
kind: APIAccess
metadata:
  name: admin
  namespace: admin
spec:
  apis: # <===== Select only this API
    - name: admin-api
  groups: # <=== Allow access only for this group
    - admin

---
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: admin-api
  namespace: admin
  annotations:
    hub.traefik.io/api: admin-api
spec:
  entryPoints:
    - web
  routes:
  - match: Host(`api.docker.localhost`) && PathPrefix(`/admin`)
    kind: Rule
    services:
    - name: admin-app
      port: 3000
```

```shell
kubectl apply -f api-management/2-access-control/simple/admin-api.yaml
```

```shell
api.hub.traefik.io/admin-api created
apiaccess.hub.traefik.io/admin created
ingressroute.traefik.io/admin-api created
```

For the _external_ `API` with its `APIAccess`, we'll see how to use a label selector:

```yaml
---
apiVersion: hub.traefik.io/v1alpha1
kind: API
metadata:
  name: weather-api
  namespace: apps
  labels:
    subscription: external
spec: {}

---
apiVersion: hub.traefik.io/v1alpha1
kind: APIAccess
metadata:
  name: weather-api-external
  namespace: apps
spec:
  groups:
    - external
  apiSelector: # <======== Select all APIs with label subscription=external
    matchLabels:
      subscription: external

---
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: weather-api
  namespace: apps
  annotations:
    hub.traefik.io/api: weather-api
spec:
  entryPoints:
    - web
  routes:
  - match: Host(`api.docker.localhost`) && PathPrefix(`/weather`)
    kind: Rule
    services:
    - name: weather-app
      port: 3000
```

```shell
kubectl apply -f api-management/2-access-control/simple/weather-api.yaml
```

```shell
api.hub.traefik.io/weather-api created
apiaccess.hub.traefik.io/weather-api created
ingressroute.traefik.io/weather-api created
```

### Test it

First, we'll need to create the _admin_ user in the _admin_ group and the _external_ user in the _external_ group, following instructions in [getting started](../1-getting-started/README.md)

Now, we can test it with the users' API tokens.

```shell
export ADMIN_TOKEN=
export EXTERNAL_TOKEN=
```

```shell
# This call is allowed => 200
curl -i -H "Authorization: Bearer $ADMIN_TOKEN" "http://api.docker.localhost/admin"
# This call is forbidden => 403
curl -i -H "Authorization: Bearer $ADMIN_TOKEN" "http://api.docker.localhost/weather"
```

```shell
# This call is allowed => 200
curl -i -H "Authorization: Bearer $EXTERNAL_TOKEN" "http://api.docker.localhost/weather"
# This call is forbidden => 403
curl -i -H "Authorization: Bearer $EXTERNAL_TOKEN" "http://api.docker.localhost/admin"
```

:information_source: If it fails, just wait one minute and try again. The token needs to be sync before it can be accepted by Traefik Hub.

## Advanced access control

This second example is more complex, but it's also more secure, using Operation Filters.

* _admin_ can get and update weather data on **private-app** and access without restriction on **admin-app**
* _external_ user can only get data on **weather-app**

```mermaid
---
title: Advanced access control
---
graph LR
    admin-user[Admin User] -->|ALL| admin-app(admin API)
    admin-user -->|GET,PUT| weather-app
    external-user[External User] --> |GET| weather-app(weather API)

 %% CSS
 classDef apis fill:#326ce5,stroke:#fff,stroke-width:4px,color:#fff;
 classDef users fill:#fff,stroke:#bbb,stroke-width:2px,color:#326ce5;
 class admin-user,external-user users;
 class admin-app,weather-app apis;
```

One needs to define operationSets to configure operationFilters. Here, we'll differentiate **GET** and **PATCH** HTTP methods.

```diff
$ diff -Nau --color api-management/2-access-control/simple/weather-api.yaml api-management/2-access-control/complex/weather-api.yaml
--- api-management/2-access-control/simple/weather-api.yaml
+++ api-management/2-access-control/complex/weather-api.yaml
@@ -9,6 +9,15 @@
 spec:
   openApiSpec:
     path: /openapi.yaml
+    operationSets:
+      - name: get-forecast
+        matchers:
+          - pathPrefix: "/weather"
+            methods: [ "GET" ]
+      - name: patch-forecast
+        matchers:
+          - pathPrefix: "/weather"
+            methods: [ "PATCH" ]

 ---
 apiVersion: hub.traefik.io/v1alpha1
 ---
 apiVersion: hub.traefik.io/v1alpha1
 kind: APIAccess
 metadata:
-  name: weather-api
+  name: weather-api-external
   namespace: apps
 spec:
   groups:
@@ -22,6 +31,26 @@
   apiSelector:
     matchLabels:
       subscription: external
+  operationFilter:
+    include:
+      - get-forecast
+
+---
+apiVersion: hub.traefik.io/v1alpha1
+kind: APIAccess
+metadata:
+  name: weather-api-admin
+  namespace: apps
+spec:
+  groups:
+    - admin
+  apiSelector:
+    matchLabels:
+      subscription: external
+  operationFilter:
+    include:
+      - get-forecast
+      - patch-forecast
```

### Deploy and test it

After deploying it:

```shell
kubectl apply -f api-management/2-access-control/complex/weather-api.yaml
kubectl apply -f api-management/2-access-control/complex/admin-api.yaml
```

It can be tested with the API token of the admin:

```shell
# This call is allowed.
curl -i -H "Authorization: Bearer $ADMIN_TOKEN" "http://api.docker.localhost/admin/"
# This call is now allowed
curl -i -H "Authorization: Bearer $ADMIN_TOKEN" "http://api.docker.localhost/weather"
# And even PATCH is allowed
curl -i -XPATCH -H "Authorization: Bearer $ADMIN_TOKEN" "http://api.docker.localhost/weather"
```

And test it with the external user's token:

```shell
# This one is allowed
curl -i -H "Authorization: Bearer $EXTERNAL_TOKEN" "http://api.docker.localhost/weather"
# And PATCH should be not allowed
curl -i -XPATCH -H "Authorization: Bearer $EXTERNAL_TOKEN" "http://api.docker.localhost/weather"
```

It can be explained quite easily if **PATCH** is still allowed. There is still an `APIAccess` created with the simple tutorial:

```yaml
kubectl get apiaccess -n apps
NAME                    AGE
weather-api            16m
weather-api-admin      16m
weather-api-external   16m
```

It means that for the `external` user group, there are two `APIAccess` applying:

This is the first one:

```yaml
---
apiVersion: hub.traefik.io/v1alpha1
kind: APIAccess
metadata:
  name: weather-api
  namespace: apps
spec:
  groups:
    - external
  apiSelector:
    matchLabels:
      subscription: external
```

And this is the second one:

```yaml
---
apiVersion: hub.traefik.io/v1alpha1
kind: APIAccess
metadata:
  name: weather-api-external
  namespace: apps
spec:
  groups:
    - external
  apiSelector:
    matchLabels:
      subscription: external
  operationFilter:
    include:
      - get-forecast
```

The first one allows all kinds of HTTP requests. If we delete it, the _external_ user can no longer call the API with the **PATCH** HTTP verb.

```shell
kubectl delete apiaccess -n apps weather-api
# This time, PATCH is not allowed
curl -i -XPATCH -H "Authorization: Bearer $EXTERNAL_TOKEN" "http://api.docker.localhost/weather"
```
