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

## Run it locally (OSX)

```bash
$ ln -s /var/lib/kubelet/kubeconfig config

$ kubectl -n quobyte port-forward webconsole-2030557253-bq8xk 7860 > /dev/null &

$ grep quobyte /proc/mounts
quobyte@10.0.28.3:7866|10.0.66.3:7866|10.0.67.2:7866/ /var/lib/kubelet/plugins/kubernetes.io~quobyte fuse rw,nosuid,nodev,noatime,user_id=0,group_id=0,allow_other 0 0
```

# TODO

- [ ] Docs
- [ ] Memory in scheduling
- [X] Localisation (mountpoint + xattr)
- [X] Annotate used file ?
- [ ] Support DiskType
- [ ] Support Multiple Files
