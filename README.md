# sfc-k8s-operator

## How to run controllers?
```
make manifests
make install
make run
make uninstall
make undeploy
```

## Custom resource
```
kubectl create ns sfc

# deploy service function chain, service functions CR and services
kubectl apply -k config/samples/sfc/ -n sfc
kubectl apply -k config/samples/service-function/ -n sfc

# delete service functions
kubectl delete pod --all -n sfc
```
