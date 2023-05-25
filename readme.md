# Hub

  - [K3D](#k3d)
    - [Prerequisites](#prerequisites)
    - [Installation](#installation)
    - [Using Tunneling](#using-tunneling)
  - [Postman](#postman)
  - [API](./api/api.md)
    - [Hub agent traefik](./api/api.md#hub-agent-traefik)
  - [Nats](#nats)

**You can run the full stack of Hub with K3D or by manually install every piece of the puzzle.
We hardly recommend the K3D way of course.**

## K3D

### Prerequisites

You will need these binaries to use the script:
- jq
- gcloud
- k3d
- kubectl
- helm

You need to be logged in gcloud before running any script.

```bash
gcloud auth login
gcloud auth configure-docker
```

Before running the script, you need a `.env` file in the `hub` folder.
Just copy the `.env.example` and fill it with your own credentials.

- `GCLOUD_EMAIL` => Your email address to connect to gcr.
- `AWS_CLIENT_ID` => A client ID for connection to AWS
- `AWS_CLIENT_SECRET` => A client secret ID for AWS
- `HUB_USERNAME` => Your username on Hub
- `HUB_PASSWORD` => Your password on Hub

The AWS secrets can be found in `keybase://team/containous.dev/hub/k3d.md`.
The Hub account can be found in `keybase://team/containous.dev/hub/auth0.md` (`JWT_PASSWORD` and `JWT_USER`).

#### Mac User

On macOS, you need to install `coreutils` for the script to work.

To resolve `*.docker.localhost`, you also need to add these hosts in your `/etc/hosts`:
```bash
127.0.0.1 platform.docker.localhost
127.0.0.1 webapp.docker.localhost
127.0.0.1 jaeger-ui.docker.localhost
127.0.0.1 prometheus.docker.localhost
127.0.0.1 grafana.docker.localhost
127.0.0.1 sso.portal.docker.localhost
```

### Installation

The local installation can be done with `make run`. The script will create a k3d cluster and deploy the following objects:
- IngressControllers
  - Traefik
- Hub platform
  - MongoDB
  - Hydra
  - Nats
  - Hub services:
    - metrics
    - workspace
    - topology
    - alert
    - clusters
    - certificates
    - invitation
    - ui
    - token
    - notification
    - gslb
    - offer
    - acp
    - tunnel
    - api-management
    - (+ an ingress to access to all the services)
- Traefik Hub sidecar
- Jaeger
- Monitoring
  - Grafana
  - Prometheus
- Pebble

There are several commands to renew secrets, clean, or speed up the deployment :

#### jwt

If you need to renew your jwt. You can just run this command :

```
make jwt
```

#### renew-gcr-token

If your gcr credentials expire, you need to renew them. You can just run this command :

```
make renew-gcr-token
```

#### renew-auth0-admin-token

If the workspace service doesn't work as expected, and you get some auth0 errors logs, your token is probably expired.
You can renew it with this command:

```
make renew-auth0-admin-token
```

#### clean

`make clean` won't delete the k3d cluster but will delete every component created with the `make run` command.

#### delete

`make delete` will delete the k3d cluster.

#### run-adsl

`make run-adsl` allows docker to pull the images before starting the cluster.
We recommend running it instead of `make run` if your internet connection is a bit slow.


### Mongodb

The mongodb is provisioned with username `root` and password `admin`.

```bash
# forward port
kubectl port-forward -n mongo services/mongodb 27017:27017

# using mongosh cli
mongosh mongodb://root:admin@localhost
```

### Using Tunneling

To have a complete view at the tunneling functionality, you can read this
[doc](https://notion.so/containous/10-01-22-Tunneling-8bc7a7451abe4679afa8c24a4456ee36).

To be used with the k3d cluster, we have deployed the broker to a new namespace (like we used to do with the pop).
The broker opens port on the fly so exposing it is quite difficult. For now, we choose to use port forward.

Once your agent is running and you GSLB configured with a private connection, a tunnel group should be available in the
database.

Example:
```
replicaset:PRIMARY> use tunnels
switched to db tunnels
replicaset:PRIMARY> db.tunnelgroups.find().pretty()
{
	"_id" : ObjectId("620bb99d8616ee3e267c596d"),
	"workspaceId" : "6311c90bfce04bd29e473a20",
	"clusterId" : "d992ed12-e160-472e-ad14-20e6ec7150c9",
	"clusterEndpoint" : ":11002",
	"tunnels" : [
		{
			"id" : "ba4d0618-65b0-487f-b362-a0cc082efe47",
			"brokerId" : "1b285347-f06e-4985-817e-a1db0a5e8886",
			"inboundPort" : 36717
		}
	],
	"tunnelCount" : 1,
	"tunnelsUpdatedAt" : ISODate("2022-02-15T15:14:04.252Z"),
	"subscriptionCount" : 1,
	"subscriptionCountUpdatedAt" : ISODate("2022-02-15T14:32:04.228Z")
}
```
You can also find the port in the broker logs if you want.

The inboundPort is the port you need to expose with the port-forward:
```
kubectl port-forward -n broker hub-tunnel-broker-5bb8446c58-2qmfq 36717:36717
```

Then, you can call the broker and access the private endpoint:
```
curl http://127.0.0.1:36717 -H 'Host: whoami.localhost'
Hostname: 70a8e07bcaa9
IP: 127.0.0.1
IP: 172.17.0.2
RemoteAddr: 172.17.0.1:63978
GET / HTTP/1.1
Host: whoami.localhost
User-Agent: curl/7.77.0
Accept: */*
Accept-Encoding: gzip
X-Real-Ip: 127.0.0.1
```

### Exposed Endpoints

- UI: https://webapp.docker.localhost/
- Jaeger: https://jaeger-ui.docker.localhost/
- Hub-APIs: http://platform.docker.localhost/
    - /agent
    - /workspace
    - /topology
    - /cluster
    - /token
    - /notification
    - /alert
    - /certificates
    - /invitation

- Grafana: https://grafana.docker.localhost
- Prometheus: https://prometheus.docker.localhost/

## Postman

A Postman collection with multiple environments is available in this repo. Check out the dedicated [readme](/postman/readme.md).

## Nats

### Install and configuring nats-cli

Install nats-cli from: https://docs.nats.io/using-nats/nats-tools/nats_cli

```bash
# Setting up nats-cli context
nats context save traefik-hub-local \
--server nats://nats.docker.localhost:4222 \
--user traefik-hub \
--password traefik-hub \
--description 'Traefik Hub local' \
--select
```

### Wiretap all nats messages

```bash
nats subscribe '>'
```
