# Development

This doc explains how to set up a development environment, so you can get started
contributing to `etcd-operator` or build a PoC (Proof of Concept). 

## Prerequisites

1. Golang version 1.13+
2. Kubernetes version v1.15+ with `~/.kube/config` configured.
3. Kustomize version 3.5+
4. Kubebuilder version 2.3+

## Build

* Clone this project

```shell script
git clone git@github.com:wenhuwang/etcd-operator.git
```

* Install Etcd CRD into your cluster

```shell script
make install
```

## Develop & Debug

If you change Etcd CRD, remember to rerun `make && make install`.

Use the following command to develop and debug.

```shell script
$ make run
```

For example, use the following command to create an etcd cluster.

```shell script
$ cd ./config/samples

$ kubectl apply -f etcd_v1alpha1_etcdcluster.yaml
etcd.mars.io/etcdcluster-sample created

$ kubectl get pods -n pg
etcdcluster-sample-0                   3/3     Running   0          3m
etcdcluster-sample-1                   3/3     Running   0          3m
etcdcluster-sample-2                   3/3     Running   0          3m
```

## Make a pull request

Remember to write unit-test and e2e test before making a pull request.