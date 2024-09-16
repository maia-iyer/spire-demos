# Multiple Kind Cluster Federation with the Tornjak API

1. Create Kind clusters
2. Deploy SPIRE on each cluster which specifies its trust domain
3. Federate the clusters

We will mimic connectivity via port-forwarding. 

### Required Steps for Federation

We do the federation after creating the clusters and deploying SPIRE. Therefore, we will be doing dynamic federation. 

To federate the client cluster with the server cluster takes a couple steps:
1. Upon deploying SPIRE, configure such that:
    - enable federation endpoint
    - trust domain must be specified
2. After deployment, 
    - Obtain the server-side SPIRE bundle
    - Pass the bundle to the client-side SPIRE server via a Tornjak API call

----------

## A step-by-step tutorial for locally demonstrating federation between Kind clusters

### Step 0: Requirements

- kubectl 
- kind (this tutorial was tested with podman and kind)
- helm

If you are using Podman, you will need to set the `KIND_EXPERIMENTAL_PROVIDER`:

```
export KIND_EXPERIMENTAL_PROVIDER=podman
```

### Step 0.5: Create the Kind clusters

For the purposes of this, we can name the clusters `server` and `client`:

```
kind create cluster --name=server
export SERVER_CONTEXT=$(kubectl config current-context)
kind create cluster --name=client
export CLIENT_CONTEXT=$(kubectl config current-context)
```

### Step 1: Deploy SPIRE + Tornjak via Helm

We will deploy SPIRE and Tornjak via the Helm charts. 

#### The Custom Helm Values

There are two things to note of the configurations of the SPIRE server:

1. **The trust domains are configured to be different.** If this is not true, then the SPIRE servers will not be able to federate. 
2. **controllerManager identities is set with a federatesWith field.** The SPIRE controller manager automatically creates workload entries when pods are created in the cluster. Setting this field causes all workload entries to automatically receive the trust bundle of the other trust domain. 

Deploy with the following commands:

```
helm upgrade --install -n spire-mgmt spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace --kube-context=$SERVER_CONTEXT
helm upgrade --install -n spire-mgmt spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f helm_values_server.yaml --kube-context=$SERVER_CONTEXT

helm upgrade --install -n spire-mgmt spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace --kube-context=$CLIENT_CONTEXT
helm upgrade --install -n spire-mgmt spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f helm_values_client.yaml --kube-context=$CLIENT_CONTEXT
```

### Step 1.5: Expose the Bundle Endpoints

First, let's expose the bundle endpoints for the server cluster on `localhost:8440`. 

Open a new terminal window for each of the following commands. Note you will likely
need to substitute the `$SERVER_CONTEXT` variable in the new
terminal sessions. The following commands will hang. 

```
kubectl port-forward --context $SERVER_CONTEXT -n spire-server svc/spire-server 8440:8443
```

OPTIONALLY, you can see the bundle endpoints by going to `https://localhost:8440`
in your browser. 

Now expose the Tornjak backend endpoints:

```
kubectl port-forward --context $SERVER_CONTEXT -n spire-server svc/spire-tornjak-backend 10000:10000
```

```
kubectl port-forward --context $CLIENT_CONTEXT -n spire-server svc/spire-tornjak-backend 10001:10000
```

### Step 2: Federate the clusters

Let's federate the client-side SPIRE server with the server-side SPIRE server. 

#### Obtain initial trust bundles

First, we must obtain the initial trust bundle of the server-side SPIRE server. We can do this by performing a Tornjak API call:

```
curl localhost:10000/api/v1/spire/bundle
```

#### Exchange the trust bundles

Now we can create a federation relationship on the client-side SPIRE server. Here's the following curl command:

```
curl --request POST \
  --data "$(
    jq -n --argjson bundle "$(curl -s localhost:10000/api/v1/spire/bundle)" '{
      "federation_relationships": [
        {
          "trust_domain": "server.org",
          "bundle_endpoint_url": "https://host.docker.internal:8440",
          "https_spiffe": {
            "endpoint_spiffe_id": "spiffe://server.org/spire/server"
          },
          "trust_domain_bundle": $bundle
        }
      ]
    }'
  )" \
  http://localhost:10001/api/v1/spire/federations
```

The above makes a call to the client-side Tornjak server that creates a federation relationship based on the bundle obtained from the server-side Tornjak server. 

To see that the federation relationship was configured, run the following:

```
curl localhost:10001/api/v1/spire/federations
```

And you should see one entry. You may also view the logs to verify the bundle has successfully been refreshed:

```
kubectl --context=kind-client logs -n spire-server spire-server-0 | grep "Bundle refreshed"
```

### Cleanup

```
kind delete cluster --name=server
kind delete cluster --name=client
```


----------

### References

- Docker-compose quickstart for federation [here](https://github.com/spiffe/spire-tutorials/tree/main/docker-compose/federation)
- Kubernetes tutorial for federation [here](https://github.com/flobuehr/spire-federation)
- Helm charts for deployment [here](https://github.com/spiffe/helm-charts-hardened)
- Docs for SPIRE controller manager CRDs [here](https://github.com/spiffe/spire-controller-manager/tree/main/docs)

