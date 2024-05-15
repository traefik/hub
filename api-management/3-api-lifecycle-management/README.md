# API LifeCycle Management

In this tutorial, we will see how to publish a new version of an API.

## Deploy an API

First, we'll deploy the API in _getting started_:

```shell
kubectl apply -f src/manifests/weather-app.yaml
kubectl apply -f api-management/1-getting-started/manifests/api.yaml
```

```shell
namespace/apps created
configmap/weather-data created
deployment.apps/weather-app created
service/weather-app created
api.hub.traefik.io/weather-api created
apiaccess.hub.traefik.io/weather-api created
ingressroute.traefik.io/weather-app created
```

And confirms it works as expected:

```shell
export ADMIN_TOKEN=
```

```shell
# This call is not allowed
curl -i http://api.docker.localhost/weather
# This call is allowed
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.docker.localhost/weather
```

## Publish the first API Version

To use API Version features, we'll need to:

1. Declare an `APIVersion`
2. Reference it into the `API`
3. Use it in the routing


```diff
$ diff -Nau api-management/1-getting-started/manifests/api.yaml api-management/3-api-lifecycle-management/manifests/api-v1.yaml
--- api-management/1-getting-started/manifests/api.yaml
+++ api-management/3-api-lifecycle-management/manifests/api-v1.yaml
@@ -1,10 +1,21 @@
 ---
 apiVersion: hub.traefik.io/v1alpha1
+kind: APIVersion
+metadata:
+  name: weather-api-v1
+  namespace: apps
+spec:
+  release: v1.0.0
+
+---
+apiVersion: hub.traefik.io/v1alpha1
 kind: API
 metadata:
   name: weather-api
   namespace: apps
-spec: {}
+spec:
+  versions:
+    - name: weather-api-v1

 ---
 apiVersion: hub.traefik.io/v1alpha1
@@ -24,7 +35,7 @@
   name: weather-app
   namespace: apps
   annotations:
-    hub.traefik.io/api: weather-api
+    hub.traefik.io/api-version: weather-api-v1
 spec:
   entryPoints:
   - web
```

We can apply it:

```shell
kubectl apply -f api-management/3-api-lifecycle-management/manifests/api-v1.yaml
```

```shell
apiversion.hub.traefik.io/weather-api-v1 created
api.hub.traefik.io/weather-api configured
apiaccess.hub.traefik.io/weather-api unchanged
ingressroute.traefik.io/weather-app configured
```

And confirm it's still working:

```shell
# This call is not allowed
curl -i http://api.docker.localhost/weather
# This call is allowed
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.docker.localhost/weather
```

## Publish a second API Version

Now, let's say a new version is available. We'll need to test whether everything is OK before making it go to production.

So, for this second API Version, we'll need to:

1. Deploy this new version
2. Declare an `APIVersion`
3. Reference it into the `API`
4. Create a new `IngressRoute` requiring a special header

```diff
$ diff -Nau api-management/3-api-lifecycle-management/manifests/api-v1.yaml api-management/3-api-lifecycle-management/manifests/api-v1.1.yaml
--- api-management/3-api-lifecycle-management/manifests/api-v1.yaml
+++ api-management/3-api-lifecycle-management/manifests/api-v1.1.yaml
@@ -9,6 +9,15 @@

 ---
 apiVersion: hub.traefik.io/v1alpha1
+kind: APIVersion
+metadata:
+  name: weather-api-v1-1
+  namespace: apps
+spec:
+  release: v1.1.0
+
+---
+apiVersion: hub.traefik.io/v1alpha1
 kind: API
 metadata:
   name: weather-api
@@ -16,6 +25,7 @@
 spec:
   versions:
     - name: weather-api-v1
+    - name: weather-api-v1-1

 ---
 apiVersion: hub.traefik.io/v1alpha1
@@ -45,3 +55,21 @@
     services:
     - name: weather-app
       port: 3000
+
+---
+apiVersion: traefik.io/v1alpha1
+kind: IngressRoute
+metadata:
+  name: weather-api-v1-1
+  namespace: apps
+  annotations:
+    hub.traefik.io/api-version: weather-api-v1-1
+spec:
+  entryPoints:
+  - web
+  routes:
+  - match: Host(`api.docker.localhost`) && PathPrefix(`/weather`) && Header(`X-Version`, `preview`)
+    kind: Rule
+    services:
+    - name: weather-app-forecast
+      port: 3000
```

So let's do it:

```shell
kubectl apply -f src/manifests/weather-app-forecast.yaml
kubectl apply -f api-management/3-api-lifecycle-management/manifests/api-v1.1.yaml
```

```shell
namespace/apps unchanged
configmap/weather-app-forecast-data created
deployment.apps/weather-app-forecast created
service/weather-app-forecast created
configmap/weather-app-forecast-openapispec created
apiversion.hub.traefik.io/weather-api-v1 unchanged
apiversion.hub.traefik.io/weather-api-v1-1 created
api.hub.traefik.io/weather-api configured
apiaccess.hub.traefik.io/weather-api unchanged
ingressroute.traefik.io/weather-api configured
ingressroute.traefik.io/weather-api-v1-1 created
```

Now, we can test if it works:

```shell
# Even with preview X-Version header, it should return 401 without token
curl -i  -H "X-Version: preview" http://api.docker.localhost/weather
# Regular access => returns weather data
curl  -H "Authorization: Bearer $ADMIN_TOKEN" http://api.docker.localhost/weather
# Preview access, with special header => returns forecast data
curl -H "X-Version: preview"  -H "Authorization: Bearer $ADMIN_TOKEN" http://api.docker.localhost/weather
```

To go further, one can use this pattern with other Traefik Middlewares to route versions based on many parameters: path, query, content type, clientIP, basicAuth, forwardAuth, and many others!

## Try the new version with a part of the traffic

Once this new version is adequately tested, we'll want to put it in production. We'll distribute the traffic among the two versions to see if it can handle the load.

To achieve this goal, we'll need to:

1. Remove test `IngressRoute` weather-api-v1-1
2. Declare a Weighted Round Robin TraefikService for load balancing
3. Use it in the `IngressRoute`

Since the last step, the diff is looking like this:

```diff
$ diff -Nau --color api-management/3-api-lifecycle-management/manifests/api-v1.1.yaml api-management/3-api-lifecycle-management/manifests/api-v1.1-weighted.yaml
--- api-management/3-api-lifecycle-management/manifests/api-v1.1.yaml
+++ api-management/3-api-lifecycle-management/manifests/api-v1.1-weighted.yaml
@@ -1,4 +1,20 @@
 ---
+apiVersion: traefik.io/v1alpha1
+kind: TraefikService
+metadata:
+  name: weather-api-wrr
+  namespace: apps
+spec:
+  weighted:
+    services:
+      - name: weather-app
+        port: 3000
+        weight: 1
+      - name: weather-app-forecast
+        port: 3000
+        weight: 1
+
+---
 apiVersion: hub.traefik.io/v1alpha1
 kind: APIVersion
 metadata:
@@ -53,23 +69,6 @@
   - match: Host(`api.docker.localhost`) && PathPrefix(`/weather`)
     kind: Rule
     services:
-    - name: weather-app
-      port: 3000
-
----
-apiVersion: traefik.io/v1alpha1
-kind: IngressRoute
-metadata:
-  name: weather-api-v1-1
-  namespace: apps
-  annotations:
-    hub.traefik.io/api-version: weather-api-v1-1
-spec:
-  entryPoints:
-  - web
-  routes:
-  - match: Host(`api.docker.localhost`) && PathPrefix(`/weather`) && Header(`X-Version`, `preview`)
-    kind: Rule
-    services:
-    - name: weather-app-forecast
+    - name: weather-api-wrr
       port: 3000
+      kind: TraefikService
```

Let's apply it:

```shell
kubectl delete ingressroute -n apps weather-api-v1-1
kubectl apply -f api-management/3-api-lifecycle-management/manifests/api-v1.1-weighted.yaml
```

```shell
ingressroute.traefik.io "weather-api-v1-1" deleted
traefikservice.traefik.io/weather-api-wrr created
apiversion.hub.traefik.io/weather-api-v1 unchanged
apiversion.hub.traefik.io/weather-api-v1-1 unchanged
api.hub.traefik.io/weather-api unchanged
apiaccess.hub.traefik.io/weather-api unchanged
ingressroute.traefik.io/weather-api configured
```

A simple test should confirm that it works:

```shell
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.docker.localhost/weather
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.docker.localhost/weather
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.docker.localhost/weather
curl -H "Authorization: Bearer $ADMIN_TOKEN" http://api.docker.localhost/weather
```

To go further, it's also possible to mirror production traffic to a new version and/or to use a sticky session.
