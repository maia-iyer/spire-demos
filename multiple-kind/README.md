# Multiple Kind Cluster Federation

This tutorial creates the following structure:

```
SERVER-CLUSTER                                     CLIENT-CLUSTER
     |                                                  |
     |                                                  |
     ----------------------                             ------------------------------
     |                     |                            |                             |
 -SPIRE namespaces-       - DEMO namespace -            --DEMO namespace--            --SPIRE namespaces--
|                  |     |                  |   TLS    |                 |           |                    |
| SPIRE deployment |     | Server Deployment|   <-->   |Client deployment|           |   SPIRE deployment |
|                  |     |                  |          |                 |           |                    |
 ------------------       ------------------           -------------------            ---------------------
     ^                                                                                    ^
     |                                                                                    |
     ---------------------------- SPIRE FEDERATION via bundle endpoints -------------------

```

And the steps follow this loose structure:

1. Create Kind clusters
2. Deploy SPIRE on each cluster which specifies its trust domain
3. Federate the clusters
4. Edit the ClusterSPIFFEID that automatically creates entries to include federation for demo workspace
5. Create `demo` namespace and deploy Server and Client workloads

We will mimic connectivity via port-forwarding. 

### A look at the applications

This tutorial demonstrates a simple Client - Server communication. The code included is copied
nearly directly from the [go-spiffe jwt example](https://github.com/spiffe/go-spiffe/tree/main/v2/examples/spiffe-jwt). 
More information on what each application exactly does can be found here. 

Roughly, the server and client make a standard TLS connection, where the server presents an x509-SVID and the client authenticates with a JWT. The server will either accept and log the request upon succesful validation of the JWT, or reject and log the error. The client is a one-time job that will print `Success!!!` upon successful request. In this case, both the server and client will require the SVID and trust bundle. 

We include a Dockerfile used for building the images in this tutorial, as well as deployment files.  Find the server code in the [server folder](./server) and the client code in the [client folder](./client). 

To build the images yourself, run:

```
docker buildx build -t gospiffe-server:v0 server
docker buildx build -t gospiffe-client:v0 client
```

For this tutorial, we will be using the publicly available images at `docker.io/maiariyer/gospiffe-server:v0` and `docker.io/maiariyer/gospiffe-client:v0`. 

### Required Steps for Federation

We do the federation after creating the clusters and deploying SPIRE. Therefore, we will be doing dynamic federation. 

To federate the clusters takes a couple steps:
1. Upon deploying SPIRE, configure to enable federation and for entries to federate with the other trust domain. 
2. Obtain initial trust bundles and create a Federation object in each of the clusters

----------

## A step-by-step tutorial for locally demonstrating federation between Kind clusters

### Step 0: Requirements

- kubectl 
- kind (this tutorial was tested with podman and kind)
- helm

### Step 0.5: Create the Kind clusters

For the purposes of this, we can name the clusters `server` and `client`:

```
kind create cluster --name=server
export SERVER_CONTEXT=$(kubectl config current-context)
kind create cluster --name=client
export CLIENT_CONTEXT=$(kubectl config current-context)
```

### Step 1: Deploy SPIRE via Helm

We will use hardened Helm charts to deploy. First we get a local copy of the charts. 

```
git clone --depth=1 --branch=spire-0.17.0 git@github.com:spiffe/helm-charts-hardened.git helm-charts
cd helm-charts
```

Now, we create the CRDs in both clusters:

```
helm upgrade --install --create-namespace -n spire-mgmt spire-crds charts/spire-crds --kube-context=$SERVER_CONTEXT

helm upgrade --install --create-namespace -n spire-mgmt spire-crds charts/spire-crds --kube-context=$CLIENT_CONTEXT
```

And finally, let's install the helm charts for both clusters. The only difference between the 
cluster installs is that they need to have a different trust domain name. If they do not, 
they will not be able to federate with each other. We have provided such values in the files 
`helm_values_server.yaml` and `helm_values_client.yaml`. Note also that both files also enable
federation. 

```
helm upgrade --install --create-namespace -n spire-mgmt \
--set global.spire.namespaces.create=true \
--values examples/production/values.yaml \
--values examples/production/example-your-values.yaml \
--values examples/tornjak/values.yaml \
--values ../helm_values_server.yaml \
--kube-context=$SERVER_CONTEXT --render-subchart-notes spire charts/spire

helm upgrade --install --create-namespace -n spire-mgmt \
--set global.spire.namespaces.create=true \
--values examples/production/values.yaml \
--values examples/production/example-your-values.yaml \
--values examples/tornjak/values.yaml \
--values ../helm_values_client.yaml \
--kube-context=$CLIENT_CONTEXT --render-subchart-notes spire charts/spire

cd ..
```

```
kubectl label namespace "spire-server" pod-security.kubernetes.io/enforce=restricted --overwrite --context=$SERVER_CONTEXT
kubectl label namespace "spire-server" pod-security.kubernetes.io/enforce=restricted --overwrite --context=$CLIENT_CONTEXT
```

#### The Custom Helm Values

There are two things to note of the configurations of the SPIRE server:

1. **The trust domains are configured to be different.** If this is not true, then the SPIRE servers will not be able to federate. 
2. **controllerManager identities is set with a federatesWith field.** The SPIRE controller manager automatically creates workload entries when pods are created in the cluster. Setting this field causes all workload entries to automatically receive the trust bundle of the other trust domain. 

### Step 1.5: Expose the Bundle Endpoints

First, let's expose the bundle endpoints for each of the clusters. The server and
client cluster will have bundle endpoint exposed on `localhost:8440` and 
`localhost:8441` respectively. 

Open a new terminal window for each of the following commands. Note you will likely
need to substitute the `$SERVER_CONTEXT` and `$CLIENT_CONTEXT` variables in the new
terminal sessions. The following commands will hang. 

```
kubectl port-forward --context $SERVER_CONTEXT -n spire-server svc/spire-server 8440:8443
```

```
kubectl port-forward --context $CLIENT_CONTEXT -n spire-server svc/spire-server 8441:8443
```

OPTIONALLY, you can see the bundle endpoints by going to `https://localhost:8440` and `https://localhost:8441` 
in your browser respectively. 

### Step 2: Federate the clusters

Now that SPIRE runs in both clusters, we must federate the two servers. We will 
use the `https_spiffe` federation method and create a 
`ClusterFederatedTrustDomain` object. 


#### Obtain initial trust bundles

To configure the federation relationships, we need to obtain initial trust bundles for each
cluster. We can do the following:

```
kubectl exec -n spire-server spire-server-0 --context $SERVER_CONTEXT -- /opt/spire/bin/spire-server bundle show -format spiffe > server.bundle
kubectl exec -n spire-server spire-server-0 --context $CLIENT_CONTEXT -- /opt/spire/bin/spire-server bundle show -format spiffe > client.bundle
```

#### Exchange the trust bundles

Use the following script to create the YAMLs to configure a dynamic federation relationship. 

```
./add_bundle_to_yaml.sh fed_object_server.yaml client.bundle apply_to_server.yaml
./add_bundle_to_yaml.sh fed_object_client.yaml server.bundle apply_to_client.yaml
kubectl apply -f apply_to_server.yaml --context $SERVER_CONTEXT
kubectl apply -f apply_to_client.yaml --context $CLIENT_CONTEXT
```

To see that the federation relationships were configured, run the following:

```
kubectl exec -n spire-server spire-server-0 --context $SERVER_CONTEXT -- /opt/spire/bin/spire-server federation list
kubectl exec -n spire-server spire-server-0 --context $CLIENT_CONTEXT -- /opt/spire/bin/spire-server federation list
```

And you should see one entry each. 

### Step 5: Deploy workloads

First, let's create a `demo` namespace in each of the clusters. 

```
kubectl create namespace demo --context $SERVER_CONTEXT
kubectl create namespace demo --context $CLIENT_CONTEXT
```

Now, we can deploy the server workload and service. Wait for the pod to show `Running` state and `Ctl+C` out.

```
kubectl apply -f workload_server.yaml --context $SERVER_CONTEXT
kubectl get po -n demo --context $SERVER_CONTEXT -w
```

Now port forward the service. You may want to do this in a separate terminal

```
kubectl port-forward -n demo --context $SERVER_CONTEXT svc/server-service 10001:8443
```

Notice that if you go to `https://localhost:10001` it should give an error `Invalid or unsupported authorization header`. This comes from the fact that we are not providing the proper authorization header bearer token. 

And now, let's deploy the client job: 

```
kubectl apply -f workload_client.yaml --context $CLIENT_CONTEXT
```

From this, eventually, the job should show completion:

```
% kubectl get job -n demo
NAMESPACE   NAME     COMPLETIONS   DURATION   AGE
demo        client   1/1           6s         13h
```

And we can look at the logs which will show that the TLS call completed successfully:

```
kubectl logs -n demo --context $SERVER_CONTEXT deployment/server
kubectl logs -n demo --context $CLIENT_CONTEXT job/client
```

#### Demonstrating failure

To demonstrate a failure, an alternate audience value can be used:

```
kubectl apply -f workload_client_fail.yaml --context $CLIENT_CONTEXT
```

You will see the failure in the client logs:

```
% kubectl logs -n demo client-fail-d94tn
Starting...
Created Newx509Source
created HTTP client
created JWTSource
fetched JWT SVID
2024/03/26 02:16:17 unable to issue request to "https://host.docker.internal:10001": Get "https://host.docker.internal:10001": unexpected ID "spiffe://server.example/ns/demo/sa/default"
```

And in the server, it would show up as a TLS handshake error:

```
% kubectl logs -n demo client-fail-d94tn
...
2024/03/26 02:17:47 http: TLS handshake error from 127.0.0.1:55164: remote error: tls: bad certificate
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
