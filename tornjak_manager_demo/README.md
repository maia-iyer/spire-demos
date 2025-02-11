# Demoing the Tornjak Manager

1. Create Kind cluster
2. Deploy SPIRE + Tornjak on the cluster
3. Deploy the Tornjak manager
4. Access the Tornjak manager UI

We will mimic connectivity via port-forwarding. 

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

### Step 1: Create the Kind clusters

For the purposes of this, we can name the clusters `server` and `client`:

```
kind create cluster
```

### Step 2: Deploy SPIRE + Tornjak via Helm

We will deploy SPIRE and Tornjak via the Helm charts. 

#### The Custom Helm Values

There are two things to note of the configurations of the SPIRE server:

1. **The trust domains are configured to be different.** If this is not true, then the SPIRE servers will not be able to federate. 
2. **controllerManager identities is set with a federatesWith field.** The SPIRE controller manager automatically creates workload entries when pods are created in the cluster. Setting this field causes all workload entries to automatically receive the trust bundle of the other trust domain. 

Deploy with the following commands:

```
helm upgrade --install -n spire-mgmt spire-crds spire-crds --repo https://spiffe.github.io/helm-charts-hardened/ --create-namespace
helm upgrade --install -n spire-mgmt spire spire --repo https://spiffe.github.io/helm-charts-hardened/ -f helm_values.yaml
```

### Step 2.5: Expose the Tornjak backend Endpoints

Open a new terminal window for the following command to expose the Tornjak backend endpoints:

```
kubectl port-forward  -n spire-server svc/spire-tornjak-backend 10000:10000
```

### Step 3: Run the Tornjak Manager

Go to the Tornjak repo and run the following command:

```
go run cmd/manager/main.go 
```

This opens a port on localhost:50000. Open the browser and go to `http://localhost:50000/manager-api/server/list` to verify. 

### Step 3.5: Make a Tornjak Manager API Call

The manager acts as a registry. You can view the registered Tornjak servers with this call:

```
curl http://localhost:50000/manager-api/server-list
```

To register the server we deployed, use the following call:

```
curl http://localhost:50000/manager-api/server/register --header "Content-Type: application/json" --data '{"name": "backend", "address": "http://localhost:10000"}'
```

Finally, to run a backend call, here's an example:

```
curl http://localhost:50000/manager-api/tornjak/serverinfo/backend
```

### Step 4: Run the UI

Finally, we can run the UI. From the root Tornjak directory:

```
cd frontend
REACT_APP_API_SERVER_URI=http://localhost:50000/ REACT_APP_TORNJAK_MANAGER=true npm start
```

### Cleanup

Running the manager in local mode creates a local DB file which we can remove:

```
rm -r serverlocaldb
```

```
kind delete cluster
```


