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
    if kubectl -n quobyte exec -i qmgmt-pod -- qmgmt -u api:7860 volume show $1 &> /dev/null;
    then
        kubectl -n quobyte exec -i qmgmt-pod -- qmgmt -u api:7860 volume delete -f $1
    fi

    kubectl -n quobyte exec -i qmgmt-pod -- qmgmt -u api:7860 volume create $1 root root $2 0777
}

function import_config() {
    if [ $(kubectl exec -n quobyte -i qmgmt-pod -- qmgmt -u api:7860 volume config list | grep $1 | wc -l) -eq 0 ];
    then
        cat ./volume_configs/$1.conf | kubectl exec -n quobyte -i qmgmt-pod -- qmgmt -u api:7860 volume config import $1
    fi
}

import_config simple
import_config mysql

kubectl delete -f simple/ &> /dev/null
kubectl delete -f mysql/ &> /dev/null

cleanup_volume mysql-store mysql
cleanup_volume simple-test simple

if ! kubectl -n quobyte exec -i qmgmt-pod -- qmgmt -u api:7860 volume show result-store &> /dev/null;
then
    kubectl -n quobyte exec -i qmgmt-pod -- qmgmt -u api:7860 volume create result-store root root BASE 0777
fi
exit
echo "Run simple Benchmark with fio"
echo "Setup simple Benchmark with fio"
kubectl create -f simple/setup_job.yaml
wait_for_completion benchmark-simple-init

echo "Run non data local Benchmark with fio"
kubectl create -f simple/non_data_local.yaml
wait_for_completion benchmark-simple-non-local

echo "Run data local Benchmark with fio"
kubectl create -f simple/data_local.yaml
wait_for_completion benchmark-simple-local
# TODO how to get results?

#TODO mysql
echo "Run Benchmark with mysql"
