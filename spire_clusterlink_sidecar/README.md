# sidecar for clusterlink + spire

Steps:
1. deploy spire via helm charts into cluster
2. deploy spiffe-helper with cat command

## Step 1: deploy spire via helm

If you haven't done so create a cluster:

```
KIND_EXPERIMENTAL_PROVIDER=podman kind create cluster
```

Now clone the helm-charts repo. 

```
git clone --depth=1 --branch=spire-0.17.0 git@github.com:spiffe/helm-charts-hardened.git helm-charts
cd helm-charts
```

Install the CRDs:

```
helm upgrade --install --create-namespace -n spire-mgmt spire-crds charts/spire-crds
```

Install the helm charts. For this demo, we set 

```
helm upgrade --install --create-namespace -n spire-mgmt \
--set global.spire.namespaces.create=true \
--set spire-server.defaultX509SvidTTL=1m \
--values examples/production/values.yaml \
--values examples/production/example-your-values.yaml \
--values examples/tornjak/values.yaml \
--render-subchart-notes spire charts/spire

cd ..
```

Wait for spire-agent to come:

```
kubectl get po -n spire-system -w
```


## Step 2: Deploy the demo workload

First let's build the proper image and load into kind:

```
docker buildx build -f Dockerfile.alpine -t spiffe-helper:secret-build --load .
kind load docker-image spiffe-helper:secret-build
```

And then we can deploy:

```
kubectl create namespace demo
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
```

## Step 3: Check that it's working

First, let's see that the pod is up successfully: 

```
kubectl get po -n demo
```

```
NAME                      READY   STATUS    RESTARTS   AGE
server-6f9b66855b-5p8d9   1/1     Running   0          4s
```

Now let's see the logs:

```
kubectl logs -n demo $(kubectl get pods -n demo -o name)
```

And you can view the secrets in the demo namespace:

```
kubectl get secret -n demo
```

## Step 4: Inspect the SVID

To view and to inspect x509 fields:

```
kubectl get secret -n demo svid.pem -o jsonpath='{.data.*}' | base64 -d | openssl x509 -text
```

## Step n+1: Cleanup

To cleanup the demo deployment, remove the namespace:

```
kubectl delete namespace demo
```

You may also delete the cluster:

```
kind delete cluster
```
