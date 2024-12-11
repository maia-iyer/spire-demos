# Federate two SPIRE-enabled Kind clusters

In this tutorial, we will be creating two kind clusters, deploying SPIRE on them, deploy simple applications, and federating the clusters through Tornjak. 

The following structure is the end goal:

![arch-diagram](./rsrc/diagram.png)

We will build this with the following steps:

1. Setup the clusters
   1. Create the Kind Clusters
   2. Deploy SPIRE on both clusters
   3. Enable the experimental feature on Tornjak
   4. Expose the relevant endpoints
2. Deploy the workloads
   1. Deploy the server workload on Cluster A
   2. Deploy the client workload on Cluster A
   3. Deploy the client workload on Cluster B
3. Federate SPIRE Server B with SPIRE Server A
   1. Federate using the Tornjak API
   2. Configure workloads in Cluster B to `federateWith` Cluster A
4. Test workload connection
5. Cleanup

### Required Steps for Federation

Federation is the process of establishing trust between SPIRE servers. In this case we will be establishing a federation relationship on Cluster B with Cluster A. This requires that Cluster A has an exposed bundle endpoint. 

If this is true, we can establish the relationship in two steps:
1. Obtain the initial trust bundle of Cluster A
2. Call the Tornjak API endpoint to establish a federation relationship with the following information:
  - Bundle Endpoint URL
  - Trust Domain
  - Initial bundle

Once this is done, federation is established. 

#### Note on how foreign trust bundles get to workloads

Workloads obtain trust bundles through the workload API, even for foreign trust bundles. However, it is required that for each workload that needs a foreign trust bundle that the workload's entry is configured to `federateWith` the foreign trust domain. 

----------

## Step 0: Requirements

This tutorial has been tested on Kind with rootful Podman on OSX. Any container runtime that Kind supports should work as well. 

The following tools are required for the commands we use: 

- kubectl 
- Helm
- kind
- podman
- git
- envsubst
- jq

## Step 1: Setup the Clusters

We will create the clusters and deploy SPIRE on both. Cluster A will use a nips address. Cluster B will use localhost and port-forwarding.

Let's obtain the necessary deployment files for this tutorial:

```
git clone https://github.com/maia-iyer/spire-demos.git
cd spire-demos/tornjak_crd_federation
```

### Step 1.1: Create the Kind Clusters

If a Podman machine is up and running skip the following step. Else on OSX or Windows, run this command to start the podman machine:

```
podman machine init -m 4096 --rootful=true
podman machine start
```

If you have multiple container runtimes, specify the proper runtime:

```
export KIND_EXPERIMENTAL_PROVIDER=podman
```

Now we can create the Kind clusters. We will add extra port mappings to cluster A because we will set up ingress on that cluster. 

```
kind create cluster --name=cluster-a --config=resources/kind_cluster_a_config.yaml
export CONTEXT_A=$(kubectl config current-context)
kind create cluster --name=cluster-b
export CONTEXT_B=$(kubectl config current-context)
```

### Step 1.2: Set up Ingress on Cluster A

On Kind we can deploy an Nginx Ingress controller to access application services running within the environment.

Set the `APP_DOMAIN` environment variable to containe the subdomain for which all applications can be accessed:

```
export APP_DOMAIN=$(ipconfig getifaddr en0).nip.io
```

Confirm the variable has been populated:

```
echo $APP_DOMAIN
```

A value similar to `x.xxx.xxx.xxx.nip.io` indicates the variable has been set properly.

We will also use a local self-signed certificate to secure the TLS connections of these applications and deploy the ingress controller:

```
kubectl apply -f resources/kind_ingress_deployment_a.yaml --context=$CONTEXT_A
kubectl wait --namespace ingress-nginx --context=$CONTEXT_A \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s
```

### Step 1.3: Deploy SPIRE on each Kind cluster

Now we can deploy SPIRE on each Kind cluster. The following deploys on Cluster A

```
helm upgrade --install -n spire-mgmt spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace --kube-context=$CONTEXT_A
envsubst < resources/helm_values_a.yaml | helm upgrade --install -n spire-mgmt spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f - --kube-context=$CONTEXT_A
```

And the same for Cluster B

```
helm upgrade --install -n spire-mgmt spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace --kube-context=$CONTEXT_B
envsubst < resources/helm_values_b.yaml | helm upgrade --install -n spire-mgmt spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f - --kube-context=$CONTEXT_B
```

#### Note: on the Helm installs

Notably, the helm installs are nearly identical, except for two things:

1. They have different trust domain names. It is not possible to federated two SPIRE servers with the same trust domain names. 
2. Only Cluster A has federation enabled. This is because in this demo we only need to federate in one direction.

### Step 1.4: Configure Tornjak

Run the following to configure Tornjak to enable CRD management:

```
kubectl apply -f resources/tornjak_cm.yaml --context=$CONTEXT_A
kubectl delete po -n spire-server spire-server-0 --context=$CONTEXT_A
kubectl apply -f resources/tornjak_cm.yaml --context=$CONTEXT_B
kubectl delete po -n spire-server spire-server-0 --context=$CONTEXT_B
```

### Step 1.5: Port-forward from Cluster B

We need to expose the Tornjak backend endpoint from Cluster B. In your current terminal, run:

```
echo export CONTEXT_B=$CONTEXT_B
```

Open a new terminal and run the resulting line and the port-forward: 

```
echo export CONTEXT_B=<$CONTEXT_B value from other terminal>
kubectl port-forward -n spire-server --context=$CONTEXT_B svc/spire-tornjak-backend 10000:10000
```

## Step 2: Deploy the workloads

In this tutorial, we will deploy a TLS server on the Cluster A, and a TLS client on both clusters. 

For reference, the TLS server is SPIFFE-enabled and uses the go-spiffe library to communicate with the SPIRE agent's workload API. 

The TLS client is an Alpine image that uses the SPIFFE Helper to locally populate files with SPIRE-issued certificates. We will manually exec and curl into the container to demonstrate TLS connection. 

#### Note: on TLS connections

We are using one-direction TLS connection where clients verify the authenticity of the server. Therefore, proper communication requires the server presents a certificate that matches clients' trust bundles. Therefore, for this tutorial, we only need allow the trust bundle from Cluster A to be given to the workload in Cluster B, and no trust bundle from Cluster B need be given to workloads in Cluster A in this simple setup. 

### Step 2.1: Deploy the server

Let's deploy the SPIFFE-enabled TLS server on Cluster A:

```
envsubst < resources/workload_server.yaml | kubectl apply --context=$CONTEXT_A -f -
kubectl wait -n demo --context=$CONTEXT_A --for=condition=ready pod --selector=app=demo-server
```

### Step 2.2: Deploy the client in Cluster A

Let's deploy the client into cluster A:

```
kubectl apply -f resources/workload_client.yaml --context=$CONTEXT_A
kubectl wait -n demo --context=$CONTEXT_A --for=condition=ready pod --selector=app=client --timeout=90s
```

Once it's running, let's exec into the pod and curl the TLS server:

```
kubectl exec -n demo -it $(kubectl get po -n demo -o name -l app=client --context=$CONTEXT_A) --context=$CONTEXT_A -- curl --cacert /opt/svid_bundle.pem https://demo-server.$APP_DOMAIN
```

You should get a `Success!!!` message in response

### Step 2.3: Deploy the client into Cluster B

Let's deploy the client into cluster B:

```
kubectl apply -f resources/workload_client.yaml --context=$CONTEXT_B
kubectl wait -n demo --context=$CONTEXT_B --for=condition=ready pod --selector=app=client --timeout=90s
```

Once it's running, let's exec into the pod and curl the TLS server:

```
kubectl exec -n demo -it $(kubectl get po -n demo -o name -l app=client --context=$CONTEXT_B) --context=$CONTEXT_B -- curl --cacert /opt/svid_bundle.pem https://demo-server.$APP_DOMAIN
```

You should get an error message in response:

```
curl: (60) SSL certificate problem: unable to get local issuer certificate
More details here: https://curl.se/docs/sslcerts.html

curl failed to verify the legitimacy of the server and therefore could not
establish a secure connection to it. To learn more about this situation and
how to fix it, please visit the webpage mentioned above.
command terminated with exit code 60
```

### Step 2.4: [OPTIONAL] ErrImagePull

If you are receiving this error upon any workload deployment, it's likely due to rate limits with DockerHub. Wait a couple minutes, delete the pod that is erroring, and it should come up. 

Otherwise, you may build and load the images yourself: 

```
podman build -t docker.io/maiariyer/tls-server:v1 server --load
kind load docker-image docker.io/maiariyer/tls-server:v1
podman buildx build -t docker.io/maiariyer/tls-client:v1 client --load
kind load docker-image docker.io/maiariyer/tls-client:v1
```

The source files for the TLS server and client are in the `resources/server` and `resources/client` directories respectively. 

## Step 3: Federate the clusters

As we saw, workloads from the same trust domain have the proper trust bundle to properly establish TLS connection with the TLS server. However, workloads from a separate trust domain do not have the proper trust bundle. We will now federate SPIRE Server B with SPIRE server a using the Tornjak API. 

### Step 3.1: Federate the SPIRE Servers

We can do this with two calls: (1) obtains the trust bundle from Trust Domain A, and (2) creates a federation relationship using that bundle on SPIRE Server B. 

The first step can be done via curl command. We will use the Tornjak API for this: 

```
curl -k https://tornjak-backend.$APP_DOMAIN/api/v1/spire/bundle
```

We can pass this result as an argument using jq to format the Tornjak API call to create the bundle endpoint:

```
curl --request POST \
  --data "$(
    jq -n --argjson bundle "$(curl -sk https://tornjak-backend.$APP_DOMAIN/api/v1/spire/bundle)" --arg bundle_endpoint_url https://spire-server-federation.$APP_DOMAIN --arg trust_domain $APP_DOMAIN --arg endpoint_spiffe_id spiffe://$APP_DOMAIN/spire/server '{
      "federation_relationships": [
        {
          "trust_domain": $trust_domain,
          "bundle_endpoint_url": $bundle_endpoint_url,
          "https_spiffe": {
            "endpoint_spiffe_id": $endpoint_spiffe_id
          },
          "trust_domain_bundle": $bundle
        }
      ]
    }'
  )" \
  http://localhost:10000/api/v1/spire-controller-manager/clusterfederatedtrustdomains
```

### Step 3.2: Verify Federation Establishment

We can verify that the federation relationship is configured by making the following API call:

```
curl http://localhost:10000/api/v1/spire/federations
```

Ensure the response is non-empty.

We can also check the SPIRE server logs to ensure the connection with the foreign bundle endpoint is successfully established:

```
kubectl logs -n spire-server spire-server-0 --context=$CONTEXT_B | grep "Bundle refreshed"
```

If this result comes back non-empty the SPIRE server has been federated!

### Step 3.3: Configure workloads to `federateWith` the foreign trust domain

In order for the workload to finally obtain the foreign trust bundle, we need to configure the workload entries. We can make the following call:

```
envsubst < resources/clusterspiffeid_b.yaml | kubectl apply --context=$CONTEXT_B -f - 
```

#### Note on configuring workload entries

We are using the SPIRE controller manager in this demo to automatically register workloads. Above we are adjusting the default template for entries that workloads receive. 

It is important to note that this example allows all workload entries to receive the foreign trust domain bundle, but in practice we should be more restrictive about which workloads obtain the foreign trust bundle. 

## Step 4: Verify successful TLS connection from Cluster B

Finally, let's perform the CURL again: 

```
kubectl exec -n demo -it $(kubectl get po -n demo -o name -l app=client --context=$CONTEXT_B) --context=$CONTEXT_B -- curl --cacert /opt/svid_bundle.pem https://demo-server.$APP_DOMAIN
```

We see success!

## Step 5: Cleanup

Run the following:

```
kind delete cluster --name=cluster-a
kind delete cluster --name=cluster-b
```

Then to delete podman machine if you are running on Windows or OSX:

```
podman machine stop
podman machine rm
```

----------

### References

- Docker-compose quickstart for federation [here](https://github.com/spiffe/spire-tutorials/tree/main/docker-compose/federation)
- Kubernetes tutorial for federation [here](https://github.com/flobuehr/spire-federation)
- Helm charts for deployment [here](https://github.com/spiffe/helm-charts-hardened)
- Docs for SPIRE controller manager CRDs [here](https://github.com/spiffe/spire-controller-manager/tree/main/docs)

