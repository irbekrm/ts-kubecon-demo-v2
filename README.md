# hello

## Deploying on Kubernetes

1. Create `hello` namespace

```
kubectl create namespace hello
```

2. Create a [Tailscale API key](https://tailscale.com/kb/1085/auth-keys/)

3. Create a Kubernetes `Secret` with the key value

```
kubectl create secret generic ts-auth --namespace hello --from-literal=TS_AUTHKEY=<API KEY>
```

4. Apply manifests
```
kubectl apply -f ./yamls
```