#!/bin/bash

function wait_for_completion() {
    max_waits=180
    waits=0
    until $(kubectl get job -l job-name=$1 --no-headers -o json | jq '.items[0].status.succeeded == 1'); do
    if [[ waits -ge max_waits ]]; then
        echo "Max wait... break up"
        break
    fi

    echo "Wait for $1 to finish"
    sleep 60
    let waits++
    done;
}

function cleanup_volume() {
    if kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api:7860 volume show $1 &> /dev/null;
    then
        kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api:7860 volume delete $1 root root BENCHMARK 0777
    fi

    kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api:7860 volume create $1 root root BENCHMARK 0777
}

if [ $(kubectl exec -n quobyte -it qmgmt-pod -- qmgmt -u api:7860 volume config list | grep BENCHMARK | wc -l) -eq 0 ];
then
    cat ./quobyte_config.dump | kubectl exec -n quobyte -it qmgmt-pod -- qmgmt -u api:7860 volume config import BENCHMARK
fi

kubectl delete -f simple_dd/ &> /dev/null
#kubectl delete -f mysql/

cleanup_volume mysql-store
cleanup_volume dd-test

if ! kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api:7860 volume show result-store &> /dev/null;
then
    kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api:7860 volume create result-store root root BENCHMARK 0777
fi

echo "Run Benchmark with dd"
echo "Setup Benchmark with dd"
kubectl create -f simple_dd/setup_job.yaml
wait_for_completion benchmark-init-dd

echo "Run non data local Benchmark with dd"
kubectl create -f simple_dd/non_data_local.yaml
wait_for_completion benchmark-non-local-dd

echo "Run data local Benchmark with dd"
kubectl create -f simple_dd/data_local.yaml
wait_for_completion benchmark-local-dd
# TODO how to get results?

#TODO mysql
echo "Run Benchmark with mysql"
