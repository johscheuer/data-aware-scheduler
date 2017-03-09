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
$ VERSION=0.0.1
$ docker build -t johscheuer/data-aware-scheduler:$VERSION .
$ docker push johscheuer/data-aware-scheduler:$VERSION
```

## Run it 

### Prerequisites

- Running Quobyte Cluster
- Running Kubernetes Cluster

```

```


# TODO

- [X] Docs
- [ ] Memory in scheduling
- [X] Localisation (mountpoint + xattr)
- [X] Annotate used file ?
- [ ] Support DiskType
- [X] Support Multiple Files/directory
- [X] Containerized Scheduler
- [ ] look at taint
