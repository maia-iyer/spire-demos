# Federate the on-prem OpenShift cluster with SPIRE-enabled AWS ROSA cluster

1. Deploy SPIRE on the AWS ROSA cluster
2. Deploy the server workload
3. Deploy a client workload (and see success)
3. Deploy SPIRE on the on-prem cluster
4. Deploy a client workload (and see failure)
5. Federate the clusters
6. Deploy another client workload (and see success)

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
- helm

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

