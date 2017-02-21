# Benchmark Test

## Dockerfile
```bash
docker run --name mysql -d -v /var/lib/kubelet/plugins/kubernetes.io~quobyte/TestVolume/database:/var/lib/mysql -e MYSQL_ROOT_PASSWORD="password" mysql:8 
docker exec -ti mysql bash

apt-get update
apt-get install -y sysbench


mysql -u root 

create database dbtest;

```