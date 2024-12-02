# Federate the on-prem OpenShift cluster with SPIRE-enabled AWS ROSA cluster

In this tutorial, we will be creating two kind clusters, deploying SPIRE on them, deploy simple applications, and federating the clusters through Tornjak. 

The following structure is the end goal:

<TODO insert image>

We will build this with the following steps:

1. Setup the clusters
   a. Create the Kind Clusters
   b. Deploy SPIRE on both clusters
   c. Enable the experimental feature on Tornjak
   d. Expose the relevant endpoints
2. Deploy the workloads
   a. Deploy the server workload on Cluster A
   b. Deploy the client workload on Cluster A
   c. Deploy the client workload on Cluster B
3. Federate SPIRE Server B with SPIRE Server A
   a. Federate using the Tornjak API
   b. Configure workloads in Cluster B to `federateWith` Cluster A
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

This tutorial uses Kind on rootless Podman. We will be creating two Kind clusters and deploying on Helm:

- kubectl 
- Helm
- kind
- podman
- git

## Step 1: Setup the Clusters

We will create the clusters and deploy SPIRE on both. 

Let's obtain the necessary deployment files for this tutorial:

```
git clone git@github.com:maia-iyer/spire-demos.git -b tornjak_crd_federation
cd tornjak_crd_federation
```

### Step 1a: Create the Kind Clusters

If a Podman machine is up and running skip the following step. Else run this command to start the podman machine:

```
podman machine init -m 4096
```

Now we can create the Kind clusters. 

```
export KIND_EXPERIMENTAL_PROVIDER=podman
kind create cluster --name=server
export SERVER_CONTEXT=$(kubectl config current-context)
kind create cluster --name=client
export CLIENT_CONTEXT=$(kubectl config current-context)
```

### Step 1b: Deploy SPIRE on each Kind cluster

Now we can deploy SPIRE on each Kind cluster. The following deploys on Cluster A

```
helm upgrade --install -n spire-mgmt spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace --kube-context=$SERVER_CONTEXT
envsubst < resources/helm_values_server.yaml | helm upgrade --install -n spire-mgmt spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f - --kube-context=$SERVER_CONTEXT
```

And the same for Cluster B

```
helm upgrade --install -n spire-mgmt spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace --kube-context=$CLIENT_CONTEXT
envsubst < resources/helm_values_server.yaml | helm upgrade --install -n spire-mgmt spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f - --kube-context=$CLIENT_CONTEXT
```

#### Note: on the Helm installs

Notably, the helm installs are nearly identical, except for two things:

1. They have different trust domain names. It is not possible to federated two SPIRE servers with the same trust domain names. 
2. Only Cluster A has federation enabled. This is because in this demo we only need to federate in one direction.

### Step 1c: Configure

----------

### Step 1: Deploy SPIRE + Tornjak via Helm on the AWS ROSA Cluster

We will deploy SPIRE and Tornjak via the Helm charts. 

#### The Custom Helm Values

There are two things to note of the configurations of the SPIRE server:

1. **The trust domains are configured to be different.** If this is not true, then the SPIRE servers will not be able to federate. 
2. **controllerManager identities is set with a federatesWith field.** The SPIRE controller manager automatically creates workload entries when pods are created in the cluster. Setting this field causes all workload entries to automatically receive the trust bundle of the other trust domain. 

Deploy in OpenShift with the following commands:

```
export APP_DOMAIN=$(oc get cm -n openshift-config-managed console-public -o go-template="{{ .data.consoleURL }}" | sed 's@https://@@; s/^[^.]*\.//')
echo $APP_DOMAIN
```

```
helm upgrade --install -n spire-mgmt spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace
envsubst < helm_values_server.yaml | helm upgrade --install -n spire-mgmt spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f -
```

### Step 1.5 Enable CRD management with Tornjak locally

Run the following:

```
kubectl apply -f client_tornjak_cm.yaml
kubectl delete po -n spire-server spire-server-0
```

### Step 2: Deploy the Server Workload on ROSA

Now we will deploy a SPIFFE-enabled workload on ROSA. The following creates the demo namespace and relevant OpenShift routs. Let's apply the server deployment. 

```
envsubst < workload_server.yaml | kubectl apply -f -
```

### Step 3: Deploy a Client Workload on ROSA

Now we deploy a client workload. This client workload is simply the spiffe-helper that auto-refreshes the keys and certificates in local files. 

```
kubectl apply -f workload_client.yaml
```

Let's exec into this pod and curl:

```
kubectl exec -n demo -it $(kubectl get po -n demo -o name -l app=client) -- curl --cacert /opt/svid_bundle.pem https://demo-server.$APP_DOMAIN
```

You should get a message `Success!!!`.

### Step 3: Deploy SPIRE + Tornjak via Helm on the Prem Cluster

Deploy in OpenShift with the following commands:

```
export APP_DOMAIN=$(oc get cm -n openshift-config-managed console-public -o go-template="{{ .data.consoleURL }}" | sed 's@https://@@; s/^[^.]*\.//')
echo $APP_DOMAIN
```

```
helm upgrade --install -n spire-mgmt spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace
envsubst < helm_values_client.yaml | helm upgrade --install -n spire-mgmt spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f -
```

### Step 3.5: Enable CRD management with Tornjak locally

Run the following:

```
kubectl apply -f client_tornjak_cm.yaml
kubectl delete po -n spire-server spire-server-0
```

### Step 4: Deploy a client workload (and see failure)

Set `ROSA_APP_DOMAIN`:

```
echo "export ROSA_APP_DOMAIN=$APP_DOMAIN"
```

Now we deploy a client workload. This client workload is simply the spiffe-helper that auto-refreshes the keys and certificates in local files. 

```
kubectl apply -f workload_client.yaml
```

Let's exec into this pod and curl:

```
kubectl exec -n demo -it $(kubectl get po -n demo -o name -l app=client) -- curl --cacert /opt/svid_bundle.pem https://demo-server.$ROSA_APP_DOMAIN
```

You should get an error. 


### Step 5: Federate the Clusters

Let's federate the client-side SPIRE server with the server-side SPIRE server. 

#### Obtain initial trust bundles

First, we must obtain the initial trust bundle of the server-side SPIRE server. We can do this by performing a Tornjak API call:

```
curl https://tornjak-backend.$ROSA_APP_DOMAIN/api/v1/spire/bundle
```

#### Exchange the trust bundles

Now we can create a federation relationship on the client-side SPIRE server. Here's the following curl command:

```
curl --request POST \
  --data "$(
    jq -n --argjson bundle "$(curl -s https://tornjak-backend.$ROSA_APP_DOMAIN/api/v1/spire/bundle)" --arg bundle_endpoint_url https://spire-server-federation.$ROSA_APP_DOMAIN --arg trust_domain $ROSA_APP_DOMAIN --arg endpoint_spiffe_id spiffe://$ROSA_APP_DOMAIN/spire/server '{
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
  https://tornjak-backend.$APP_DOMAIN/api/v1/spire-controller-manager/clusterfederatedtrustdomains

```

The above makes a call to the client-side Tornjak server that creates a federation relationship based on the bundle obtained from the server-side Tornjak server. 

To see that the federation relationship was configured, run the following:

```
curl https://tornjak-backend.$APP_DOMAIN/api/v1/spire/federations
```

And you should see one entry. You may also view the logs to verify the bundle has successfully been refreshed:

```
kubectl logs -n spire-server spire-server-0 | grep "Bundle refreshed"
```

#### Configure the workloads

Finally, the workloads must be explicitly set to receive federated bundles. We need only do this in the client cluster:

```
envsubst < clusterspiffeid_client.yaml | kubectl apply -f - 
```

### Deploy Apps

Now let's perform the curl again:

```
kubectl exec -n demo -it $(kubectl get po -n demo -o name -l app=client) -- curl --cacert /opt/svid_bundle.pem https://demo-server.$ROSA_APP_DOMAIN
```

### Cleanup

In each cluster, run:

```
kubectl delete namespace demo
helm uninstall -n spire-mgmt spire
helm uninstall -n spire-mgmt spire-crds
kubectl delete ns spire-mgmt
kubectl delete crd clusterfederatedtrustdomains.spire.spiffe.io
kubectl delete crd clusterspiffeids.spire.spiffe.io
kubectl delete crd clusterstaticentries.spire.spiffe.io
```


----------

### References

- Docker-compose quickstart for federation [here](https://github.com/spiffe/spire-tutorials/tree/main/docker-compose/federation)
- Kubernetes tutorial for federation [here](https://github.com/flobuehr/spire-federation)
- Helm charts for deployment [here](https://github.com/spiffe/helm-charts-hardened)
- Docs for SPIRE controller manager CRDs [here](https://github.com/spiffe/spire-controller-manager/tree/main/docs)
