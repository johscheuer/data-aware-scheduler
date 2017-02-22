# Benchmark Test

## Dockerfile

## Requirements

- gnuplot
- kubernetes python client
- running k8s cluster
- running data-aware scheduler

## Preparation

```
# Move this into bechmarker.py
# Manual
kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api:7860 volume create mysql-store root root BASE 0777
kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api:7860 volume create result-store root root BASE 0777

kubectl create -f jobs/config.yaml
kubectl create -f jobs/setup_job.yaml
kubectl create -f jobs/non_data_local.yaml
```
## Execute Benchmark

-> allow to run multiple rounds eq. 25 times -> avg
-> Python script ->  wait till comp.

## Run benchmark -> gnuplot


## Create Graph

We use gnuplot to draw a Graph

```bash
gnuplot benchmark_graph
```

#docker pull prom/mysqld-exporter

#docker run -d -p 9104:9104 --link=my_mysql_container:bdd  \
#        -e DATA_SOURCE_NAME="user:password@(bdd:3306)/database" prom/mysqld-exporter
