## Configure the Identity provider & generate tokens

Connect to the identity provider pod.

```shell
  kubectl exec -it -n oauthidp deployments/hydra  -- sh
```

Create a user.

```shell
hydra clients create --endpoint http://127.0.0.1:4445/ --id my-client --secret secret -g client_credentials
```

Generate the token (1h of validity).

```shell
hydra token client --endpoint http://127.0.0.1:4444/ --client-id my-client --client-secret secret
```

Then, the generated token can be used to access the protected service.

```shell
curl -v http://service.docker.localhost -H "Authorization: Bearer {TOKEN}"
```