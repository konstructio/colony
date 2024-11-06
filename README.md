# Colony Scout 

## Report

```bash
go build -o colony-scout

./colony-scout report \
  --validate=k8s,cloud-init \
  --type=server \ # server, node, agent
  # --colony-api="http://localhost:8080" \
  --token=123456 \
  --cluster-id=00000000-0000-0000-0000-000000000000 \
  --workflow-id=00000000-0000-0000-0000-000000000000 \
  --hardware-id=00000000-0000-0000-0000-000000000000 \
  --host-ip-port=192.168.0.43:6443 \
  --kubeconfig=~/.kube/config \
  --k3s-token=00000000-0000-0000-0000-000000000000

``` 

## Discovery

```bash
go build -o colony-scout
  
./colony-scout discovery \
  # --colony-api="http://localhost:8080" \
  --token=123456 \
  --hardware-id=00000000-0000-0000-0000-000000000000
        
```

## Running test

Validate and install [kwok](https://kwok.sigs.k8s.io/)

### Run tests in CI

```bash
make test
```

### Run tests locally

```bash
make start_kwok && make test
```

## Development with Docker

### Build

```bash
docker compose build
```

### Run

```bash
docker compose up
```
