# API LifeCycle Management

In this tutorial, we will see how to publish a new version of an API.

## Deploy an API

First, we'll deploy the API in _getting started_:

```shell
kubectl apply -f src/manifests/apps-namespace.yaml
kubectl apply -f src/manifests/weather-app.yaml
kubectl apply -f api-management/3-api-lifecycle-management/manifests/api.yaml
```

```shell
namespace/apps unchanged
configmap/weather-data unchanged
middleware.traefik.io/stripprefix-weather unchanged
deployment.apps/weather-app unchanged
service/weather-app unchanged
configmap/weather-app-openapispec unchanged
api.hub.traefik.io/api-lifecycle-apimanagement-weather-api created
apiaccess.hub.traefik.io/api-lifecycle-apimanagement-weather-api created
ingressroute.traefik.io/api-lifecycle-apimanagement-weather-api created
```

And confirms it works as expected:

```shell
export ADMIN_TOKEN=
```

```shell
# This call is not allowed
curl -i http://api.lifecycle.apimanagement.docker.localhost/weather
# This call is allowed
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.lifecycle.apimanagement.docker.localhost/weather
```

## Publish the first API Version

To use API Version features, we'll need to:

1. Declare an `APIVersion`
2. Reference it into the `API`
3. Use it in the routing

```diff :../../hack/diff.sh -r -a "manifests/api.yaml manifests/api-v1.yaml"
--- manifests/api.yaml
+++ manifests/api-v1.yaml
@@ -1,41 +1,54 @@
 ---
 apiVersion: hub.traefik.io/v1alpha1
-kind: API
+kind: APIVersion
 metadata:
-  name: api-lifecycle-apimanagement-weather-api
+  name: api-lifecycle-apimanagement-weather-api-v1
   namespace: apps
 spec:
+  release: v1.0.0
   openApiSpec:
     path: /openapi.yaml
     override:
       servers:
-        - url: http://api.lifecycle.apimanagement.docker.localhost
+        - url: http://api.lifecycle.apimanagement.docker.localhost/weather-v1
+
+---
+apiVersion: hub.traefik.io/v1alpha1
+kind: API
+metadata:
+  name: api-lifecycle-apimanagement-weather-api-v1
+  namespace: apps
+spec:
+  versions:
+    - name: api-lifecycle-apimanagement-weather-api-v1
 
 ---
 apiVersion: hub.traefik.io/v1alpha1
 kind: APIAccess
 metadata:
-  name: api-lifecycle-apimanagement-weather-api
+  name: api-lifecycle-apimanagement-weather-api-v1
   namespace: apps
 spec:
   apis:
-  - name: api-lifecycle-apimanagement-weather-api
+  - name: api-lifecycle-apimanagement-weather-api-v1
   everyone: true
 
 ---
 apiVersion: traefik.io/v1alpha1
 kind: IngressRoute
 metadata:
-  name: api-lifecycle-apimanagement-weather-api
+  name: api-lifecycle-apimanagement-weather-api-v1
   namespace: apps
   annotations:
-    hub.traefik.io/api: api-lifecycle-apimanagement-weather-api # <=== Link to the API using its name
+    hub.traefik.io/api-version: api-lifecycle-apimanagement-weather-api-v1
 spec:
   entryPoints:
   - web
   routes:
-  - match: Host(`api.lifecycle.apimanagement.docker.localhost`) && PathPrefix(`/weather`)
+  - match: Host(`api.lifecycle.apimanagement.docker.localhost`) && PathPrefix(`/weather-v1`)
     kind: Rule
     services:
     - name: weather-app
       port: 3000
+    middlewares:
+      - name: stripprefix-weather
```

We can apply it:

```shell
kubectl apply -f api-management/3-api-lifecycle-management/manifests/api-v1.yaml
```

```shell
apiversion.hub.traefik.io/api-lifecycle-apimanagement-weather-api-v1 created
api.hub.traefik.io/api-lifecycle-apimanagement-weather-api-v1 created
apiaccess.hub.traefik.io/api-lifecycle-apimanagement-weather-api-v1 created
ingressroute.traefik.io/api-lifecycle-apimanagement-weather-api-v1 created
```

And confirm it's still working:

```shell
# This call is not allowed
curl -i http://api.lifecycle.apimanagement.docker.localhost/weather-v1/weather
# This call is allowed
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.lifecycle.apimanagement.docker.localhost/weather-v1/weather
```

## Publish a second API Version

Now, let's say a new version is available. We'll need to test whether everything is OK before making it go to production.

So, for this second API Version, we'll need to:

1. Deploy this new version
2. Declare an `APIVersion`
3. Reference it into the `API`
4. Create a new `IngressRoute` requiring a special header

```diff :../../hack/diff.sh -r -a "manifests/api-v1.yaml manifests/api-v1.1.yaml"
--- manifests/api-v1.yaml
+++ manifests/api-v1.1.yaml
@@ -2,42 +2,43 @@
 apiVersion: hub.traefik.io/v1alpha1
 kind: APIVersion
 metadata:
-  name: api-lifecycle-apimanagement-weather-api-v1
+  name: api-lifecycle-apimanagement-weather-api-v1-1
   namespace: apps
 spec:
-  release: v1.0.0
+  release: v1.1.0
   openApiSpec:
     path: /openapi.yaml
     override:
       servers:
-        - url: http://api.lifecycle.apimanagement.docker.localhost/weather-v1
+        - url: http://api.lifecycle.apimanagement.docker.localhost/weather-multi-versions
 
 ---
 apiVersion: hub.traefik.io/v1alpha1
 kind: API
 metadata:
-  name: api-lifecycle-apimanagement-weather-api-v1
+  name: api-lifecycle-apimanagement-weather-api-v1-1
   namespace: apps
 spec:
   versions:
     - name: api-lifecycle-apimanagement-weather-api-v1
+    - name: api-lifecycle-apimanagement-weather-api-v1-1
 
 ---
 apiVersion: hub.traefik.io/v1alpha1
 kind: APIAccess
 metadata:
-  name: api-lifecycle-apimanagement-weather-api-v1
+  name: api-lifecycle-apimanagement-weather-api-v1-1
   namespace: apps
 spec:
   apis:
-  - name: api-lifecycle-apimanagement-weather-api-v1
+  - name: api-lifecycle-apimanagement-weather-api-v1-1
   everyone: true
 
 ---
 apiVersion: traefik.io/v1alpha1
 kind: IngressRoute
 metadata:
-  name: api-lifecycle-apimanagement-weather-api-v1
+  name: api-lifecycle-apimanagement-weather-api-v1-1
   namespace: apps
   annotations:
     hub.traefik.io/api-version: api-lifecycle-apimanagement-weather-api-v1
@@ -45,10 +46,30 @@
   entryPoints:
   - web
   routes:
-  - match: Host(`api.lifecycle.apimanagement.docker.localhost`) && PathPrefix(`/weather-v1`)
+  - match: Host(`api.lifecycle.apimanagement.docker.localhost`) && PathPrefix(`/weather-multi-versions`)
     kind: Rule
     services:
     - name: weather-app
       port: 3000
     middlewares:
       - name: stripprefix-weather
+
+---
+apiVersion: traefik.io/v1alpha1
+kind: IngressRoute
+metadata:
+  name: api-lifecycle-apimanagement-weather-api-v1-1-preview
+  namespace: apps
+  annotations:
+    hub.traefik.io/api-version: api-lifecycle-apimanagement-weather-api-v1-1
+spec:
+  entryPoints:
+  - web
+  routes:
+  - match: Host(`api.lifecycle.apimanagement.docker.localhost`) && PathPrefix(`/weather-multi-versions`) && Header(`X-Version`, `preview`)
+    kind: Rule
+    services:
+    - name: weather-app-forecast
+      port: 3000
+    middlewares:
+      - name: stripprefix-weather
```

So let's do it:

```shell
kubectl apply -f src/manifests/weather-app-forecast.yaml
kubectl apply -f api-management/3-api-lifecycle-management/manifests/api-v1.1.yaml
```

```shell
configmap/weather-app-forecast-data unchanged
deployment.apps/weather-app-forecast unchanged
service/weather-app-forecast unchanged
apiversion.hub.traefik.io/api-lifecycle-apimanagement-weather-api-v1-1 created
api.hub.traefik.io/api-lifecycle-apimanagement-weather-api-v1-1 created
apiaccess.hub.traefik.io/api-lifecycle-apimanagement-weather-api-v1-1 created
ingressroute.traefik.io/api-lifecycle-apimanagement-weather-api-v1-1 created
ingressroute.traefik.io/api-lifecycle-apimanagement-weather-api-v1-1-preview created
```

Now, we can test if it works:

```shell
# Even with preview X-Version header, it should return 401 without token
curl -i  -H "X-Version: preview" http://api.lifecycle.apimanagement.docker.localhost/weather-multi-versions/weather
# Regular access => returns weather data
curl  -H "Authorization: Bearer $ADMIN_TOKEN" http://api.lifecycle.apimanagement.docker.localhost/weather-multi-versions/weather
# Preview access, with special header => returns forecast data
curl -H "X-Version: preview"  -H "Authorization: Bearer $ADMIN_TOKEN" http://api.lifecycle.apimanagement.docker.localhost/weather-multi-versions/weather
```

To go further, one can use this pattern with other Traefik Middlewares to route versions based on many parameters: path, query, content type, clientIP, basicAuth, forwardAuth, and many others!

## Try the new version with a part of the traffic

Once this new version is adequately tested, we'll want to put it in production. We'll distribute the traffic among the two versions to see if it can handle the load.

To achieve this goal, we'll need to:

1. Remove test `IngressRoute` weather-api-v1-1
2. Declare a Weighted Round Robin TraefikService for load balancing
3. Use it in the `IngressRoute`

Since the last step, the diff is looking like this:

```diff :../../hack/diff.sh -r -a "manifests/api-v1.1.yaml manifests/api-v1.1-weighted.yaml"
--- manifests/api-v1.1.yaml
+++ manifests/api-v1.1-weighted.yaml
@@ -1,64 +1,24 @@
 ---
-apiVersion: hub.traefik.io/v1alpha1
-kind: APIVersion
-metadata:
-  name: api-lifecycle-apimanagement-weather-api-v1-1
-  namespace: apps
-spec:
-  release: v1.1.0
-  openApiSpec:
-    path: /openapi.yaml
-    override:
-      servers:
-        - url: http://api.lifecycle.apimanagement.docker.localhost/weather-multi-versions
-
----
-apiVersion: hub.traefik.io/v1alpha1
-kind: API
-metadata:
-  name: api-lifecycle-apimanagement-weather-api-v1-1
-  namespace: apps
-spec:
-  versions:
-    - name: api-lifecycle-apimanagement-weather-api-v1
-    - name: api-lifecycle-apimanagement-weather-api-v1-1
-
----
-apiVersion: hub.traefik.io/v1alpha1
-kind: APIAccess
-metadata:
-  name: api-lifecycle-apimanagement-weather-api-v1-1
-  namespace: apps
-spec:
-  apis:
-  - name: api-lifecycle-apimanagement-weather-api-v1-1
-  everyone: true
-
----
 apiVersion: traefik.io/v1alpha1
-kind: IngressRoute
+kind: TraefikService
 metadata:
-  name: api-lifecycle-apimanagement-weather-api-v1-1
+  name: api-lifecycle-apimanagement-weather-api-wrr
   namespace: apps
-  annotations:
-    hub.traefik.io/api-version: api-lifecycle-apimanagement-weather-api-v1
 spec:
-  entryPoints:
-  - web
-  routes:
-  - match: Host(`api.lifecycle.apimanagement.docker.localhost`) && PathPrefix(`/weather-multi-versions`)
-    kind: Rule
+  weighted:
     services:
-    - name: weather-app
-      port: 3000
-    middlewares:
-      - name: stripprefix-weather
+      - name: weather-app
+        port: 3000
+        weight: 1
+      - name: weather-app-forecast
+        port: 3000
+        weight: 1
 
 ---
 apiVersion: traefik.io/v1alpha1
 kind: IngressRoute
 metadata:
-  name: api-lifecycle-apimanagement-weather-api-v1-1-preview
+  name: weather-api
   namespace: apps
   annotations:
     hub.traefik.io/api-version: api-lifecycle-apimanagement-weather-api-v1-1
@@ -66,10 +26,11 @@
   entryPoints:
   - web
   routes:
-  - match: Host(`api.lifecycle.apimanagement.docker.localhost`) && PathPrefix(`/weather-multi-versions`) && Header(`X-Version`, `preview`)
+  - match: Host(`api.lifecycle.apimanagement.docker.localhost`) && PathPrefix(`/weather-v1-wrr`)
     kind: Rule
     services:
-    - name: weather-app-forecast
+    - name: api-lifecycle-apimanagement-weather-api-wrr
       port: 3000
+      kind: TraefikService
     middlewares:
       - name: stripprefix-weather
```

Let's apply it:

```shell
kubectl apply -f api-management/3-api-lifecycle-management/manifests/api-v1.1-weighted.yaml
```

```shell
traefikservice.traefik.io/api-lifecycle-apimanagement-weather-api-wrr created
ingressroute.traefik.io/weather-api created
```

A simple test should confirm that it works:

```shell
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.lifecycle.apimanagement.docker.localhost/weather-v1-wrr/weather
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.lifecycle.apimanagement.docker.localhost/weather-v1-wrr/weather
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.lifecycle.apimanagement.docker.localhost/weather-v1-wrr/weather
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.lifecycle.apimanagement.docker.localhost/weather-v1-wrr/weather
```

To go further, it's also possible to mirror production traffic to a new version and/or to use a sticky session.
