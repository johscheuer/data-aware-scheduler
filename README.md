# Data Aware Scheduler (PoC)

## Dependency Managment

For the dependency managment was (dep)[https://github.com/golang/dep] used.

```bash
dep ensure -update
```

## Build

To build the binary:

```
go build -o scheduler -a --ldflags '-extldflags "-static"' -tags netgo -installsuffix netgo .
```

to build a Docker container:

```
GOOS=linux bash build
docker build -t johscheuer/data-aware-scheduler:0.0.1 .
docker push johscheuer/data-aware-scheduler:0.0.1
```

# TODO

- [ ] Docs
- [ ] Memory in scheduling
- [ ] Localisation (mountpoint + xattr)
- [ ] Annotate used files ?

