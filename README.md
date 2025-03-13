Video Demo: https://asciinema.org/a/8oRlcN3eumfXZcgEkbKClyqFw

# Dynamic ConfigMap Propagator

## Create Operator

### Init Operator
```
operator-sdk init --domain custom --repo DynamicConfigMapPropagator
```

### Update Makefile 
Update makefile for accessing the controller-gen, set the following to Makefile
```
CONTROLLER_GEN ?= $(GOBIN)/controller-gen
```
### Create API
Create `ConfigMapSyncer` Api.
```
operator-sdk create api --group syncer --version v1alpha1 --kind ConfigMapSyncer --resource --controller --namespaced=true
```

## Modify Operator

### Update Api

## Deploy

### Make Manifests and Generator

```
make manifests

make generate
```

### Docker Build and Push

```
make docker-build docker-push
```

### Deploy to Kubernetes

```
kubectl apply -f kube/crds/
```

```
kubectl apply -f kube/manifests/namespace.yaml

kubectl apply -f kube/manifests/rbac.yaml

kubectl apply -f kube/manifests/controller.yaml
```

### Apply Custom Resource

```
kubectl apply -f kube/custom-resource/configmapsyncer.yaml
```

### Test Operator

```
kubectl apply -f kube/configmaps/master-cm-one.yaml

kubectl apply -f kube/configmaps/random-cm.yaml
```
