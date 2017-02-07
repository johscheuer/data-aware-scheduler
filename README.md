# Data Aware Scheduler (PoC)

## Dependency Managment

For the dependency managment was (dep)[https://github.com/golang/dep] used.

```bash
$ dep ensure -update
```

## Build

To build the binary (for linux):

```
$ GOOS=linux go build -o scheduler -a --ldflags '-extldflags "-static"' -tags netgo -installsuffix netgo .
```

to build a Docker container:

```
$ GOOS=linux bash build
$ docker build -t johscheuer/data-aware-scheduler:0.0.1 .
$ docker push johscheuer/data-aware-scheduler:0.0.1
```

## Run it 

### Prerequisites

Create a Quobyte Volume `TestVolume`:

```bash
kubectl exec -n quobyte -it qmgmt-pod -- qmgmt -u api:7860 volume create TestVolume root root BASE 0777
```

Create a "big" file:

```bash
# Adjust the path if needed
PATH_TO_VOL="/var/lib/kubelet/plugins/kubernetes.io~quobyte/TestVolume"
dd if=/dev/zero of=$PATH_TO_VOL/testFile.bin bs=1G count=10
```

### Master Node (not containerized)

```bash
$ ln -s /var/lib/kubelet/kubeconfig ./config

$ kubectl -n quobyte port-forward $(kubectl -n quobyte get po -l role=webconsole --no-headers | awk '{print $1}') 7860 > /dev/null &

$ grep quobyte /proc/mounts
quobyte@10.0.28.3:7866|10.0.66.3:7866|10.0.67.2:7866/ /var/lib/kubelet/plugins/kubernetes.io~quobyte fuse rw,nosuid,nodev,noatime,user_id=0,group_id=0,allow_other 0 0

$ cp config.yaml.example config.yaml

$ kubectl create -f deployments/nginx.yaml

$ ./scheduler
```

# TODO

- [ ] Docs
- [ ] Memory in scheduling
- [X] Localisation (mountpoint + xattr)
- [X] Annotate used file ?
- [ ] Support DiskType
- [ ] Support Multiple Files
- [ ] Containerized Scheduler