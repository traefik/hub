# API

## Hub agent traefik

### HTTP call done by the agent

- Link the agent
```console
Request: POST /agent/link
Request Header:
   Authorization: Bearer XXXX
Request body
{
    "platform": "other"
}


Response: 200 POST /agent/link
Response Header:
   Content-Type: application/json
   Content-Length: 53
Response body
{
    "clusterId": "6a9c9ae4-2c28-4d96-b641-58102ddf2d79"
}
```

- Retrieve the config
```console
Request: GET /agent/config
Request Header:
   Authorization: Bearer XXXX

Response: 200 GET /agent/config
Response Header:
   Content-Length: 319
   Content-Type: application/json
Response body
{
    "topology": {
        "gitProxyHost": "platform-preview.hub.traefik.io",
        "gitOrgName": "traefiklabs",
        "gitRepoName": "62863d398cd4ff5cfe9986b9"
    },
    "metrics": {
        "interval": 60000000000,
        "tables": [
            "1m",
            "10m",
            "1h"
        ]
    },
    "accessControl": {
        "maxSecuredRoutes": 3
    },
    "gslb": {
        "httpHealthcheckConfig": {
            "minIntervalSeconds": 300,
            "thresholdEditable": false
        }
    }
}
```

- Retrieve tunnel endpoints
```console
Request: GET /agent/tunnel-endpoints
Request Header:
   Authorization: Bearer XXXX

Response: 200 GET /agent/tunnel-endpoints
Response Header:
   Content-Type: application/json
Response body
[
    {
        "tunnelId": "b6a75fbd-309c-4f13-8810-3add4f2c3a0c",
        "brokerEndpoint": "ws://35.179.77.20:8080",
        "domain": "*-eiru4h.rryaci28.traefikhub.dev"
    },
    {
        "tunnelId": "c429b924-25d7-4907-909f-0e3c3fcfd19e",
        "brokerEndpoint": "ws://18.130.249.102:8080",
        "domain": "*-eiru4h.rryaci28.traefikhub.dev"
    }
]
```

- Retrieve edge ingresses
```console
Request: GET /agent/edge-ingresses
Request Header:
   Authorization: Bearer XXXX

Response: 200 GET /agent/edge-ingresses
Response Header:
   Content-Length: 416
   Content-Type: application/json
Response body
[
    {
        "id": "62976bb4021f69d5416d9b4c",
        "workspaceId": "62863d398cd4ff5cfe9986b9",
        "clusterId": "6a9c9ae4-2c28-4d96-b641-58102ddf2d79",
        "name": "whoami-1654090674655",
        "domain": "testy-ermine-eiru4h.rryaci28.traefikhub.dev",
        "service": {
            "name": "whoami",
            "network": "traefik-hub",
            "port": 80
        },
        "acp": {
            "name": "test"
        },
        "version": "p0dI2PCxsQfpBWFpz4V8Oel2u3Q=",
        "createdAt": "2022-06-01T13:37:56.851Z",
        "updatedAt": "2022-06-01T13:37:56.851Z"
    }
]
```

- Retrieve ACPs
```console
Request: GET /agent/acps
Request Header:
   Authorization: Bearer XXXX

Response: 200 GET /agent/acps
Response Header:
   Content-Length: 397
   Content-Type: application/json
Response body
[
    {
        "id": "62976bb1da692dedf0c3e5b1",
        "workspaceId": "62863d398cd4ff5cfe9986b9",
        "clusterId": "6a9c9ae4-2c28-4d96-b641-58102ddf2d79",
        "version": "n6TsvCSed7VsB+XxmGumbgZDRGA=",
        "name": "test",
        "jwt": null,
        "basicAuth": {
            "users": [
                "test:test"
            ],
            "realm": "",
            "stripAuthorizationHeader": false,
            "forwardUsernameHeader": ""
        },
        "digestAuth": null,
        "createdAt": "2022-06-01T13:37:53.911Z",
        "updatedAt": "2022-06-01T13:37:53.911Z"
    }
]
```

- Retrieve wildcard certificates
```console
Request: GET /agent/wildcard-certificate
Request Header:
   Authorization: Bearer XXXX
   
Response: 200 GET /agent/wildcard-certificate
Response Header:
   Content-Type: application/json
Response body
{
    "id": "628757bb8930a0d40e098286",
    "createdAt": "2022-05-20T08:56:27.669Z",
    "updatedAt": "2022-05-20T08:57:49.762Z",
    "workspaceId": "62863d398cd4ff5cfe9986b9",
    "certificate": "LS0tLS1CRUdJTiBDRVJ...",
    "privateKey": "LS0tLS1CRUdJTiBSU0Eg...",
    "domains": [
        "*.rryaci28.traefikhub.dev"
    ],
    "notAfter": "2022-08-18T07:57:47Z",
    "notBefore": "2022-05-20T07:57:48Z",
    "status": "active"
}
```

- Retrieve data
```console
Request: GET /agent/data
Request Header:
   Authorization: Bearer XXXX
   Accept: avro/binary;v2
   Content-Type: avro/binary;v2
   
Response: 200 GET /agent/data
Response Header:
   Content-Type: avro/binary
   Content-Length: 1638
Response body
AVRO body
```

- Push metrics to the platform
```console
Request: POST /agent/metrics
Request Header:
   Authorization: Bearer XXXX
   Accept: avro/binary;v2
   Content-Type: avro/binary;v2
Request body
AVRO body

Response: 200 POST /agent/metrics
Response Header:
   Content-Length: 0
```

- Preflight
```console
Request: POST /agent/preflight
Request Header:
   Authorization: Bearer XXXX
Request body
[]

Response: 200 POST /agent/preflight
Response Header:
   Content-Type: application/json
   Content-Length: 5
Response body
null
```
