# POC for [Authentik](https://goauthentik.io/) [Go client library](https://github.com/goauthentik/client-go)

- K8s deployment
- [gotest](/gotest), POC project utilizing [goauthentik/client-go](https://github.com/goauthentik/client-go)
  - create Group
  - create User
  - create User's password
  - create User's OAuth token
  - get User's Groups (using User's OAuth token)

## Requirements

- [gvm](https://github.com/moovweb/gvm) Go 1.19
    ```bash
    gvm install go1.19 --prefer-binary --with-build-tools --with-protobuf
    gvm use go1.19 --default
    ```
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
- [docker](https://docs.docker.com/get-docker/)
- [docker-compose](https://docs.docker.com/compose/install/)
  ```bash
  sudo apt-get install -y docker-compose
  ```

## K8s

### Deploy on K8s (PostgreSQL)

Authentik manifests already generated with Authentik Helm chart and configures with `AUTHENTIK_BOOTSTRAP_PASSWORD` and `AUTHENTIK_BOOTSTRAP_TOKEN` if you need 
to change them see next chapter first.

Execute script to deploy manifests and open browser window, login: `akadmin`, pwd: `Authentik01234567890!`

```bash
./scripts/deploy-authentik-k8s.sh
```

## Create Authentik k8s manifests using Helm

```bash
helm repo add authentik https://charts.goauthentik.io
helm repo update

helm template authentik authentik/authentik -f ./k8s/postgresql/values.yml > ./k8s/postgresql/authentik-postgresql.yml
```

If you want to set predefined `password` and `token` for the default admin user `akadmin`:

edit `deploy.yml` ->  Deployment `authentik-server`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: authentik-server
  ...
spec:
  ...
  template:
    ...
    spec:
      containers:
        - name: authentik
          ...
          env:            
            ...
            - name: AUTHENTIK_BOOTSTRAP_PASSWORD
              value: "Authentik01234567890!"
            - name: AUTHENTIK_BOOTSTRAP_TOKEN
              value: "NoMlxBQuYgfu3j19ygGqhjXenAjrJgOfN5naqmSDBUhdLjYqHKze7yyzY07H"
```

edit `deploy.yml` ->  Deployment `authentik-worker`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: authentik-worker
  ...
spec:
  ...
  template:
    ...
    spec:
      ...
      containers:
        - name: authentik
          ...
          env:            
            ...
            - name: AUTHENTIK_BOOTSTRAP_PASSWORD
              value: "Authentik01234567890!"
            - name: AUTHENTIK_BOOTSTRAP_TOKEN
              value: "NoMlxBQuYgfu3j19ygGqhjXenAjrJgOfN5naqmSDBUhdLjYqHKze7yyzY07H"
```

### ### Deploy on K8s (CockroachDB, experimental, not working yet)

```bash
# create manifests if needed
helm template crdb cockroachdb/cockroachdb --namespace default \
--set fullnameOverride=crdb \
--set single-node=true \
--set statefulset.replicas=1 > ./k8s/cockroachdb/cockroachdb.yml

# create namespace
kubectl create ns threeport-api

# deploy cockroachdb
kubectl apply -f ./k8s/cockroachdb/cockroachdb.yml
echo "waiting forcockroachdb to get ready"
kubectl wait pod -n threeport-api crdb-0 --for condition=Ready --timeout=180s

# deploy authentik
kubectl apply -f ./k8s/cockroachdb/authentik-cockroachdb.yml
kubectl apply -f ./k8s/cockroachdb/crdb-test-pod.yml

# undeploy authentik
kubectl delete -f ./k8s/cockroachdb/authentik-cockroachdb.yml

# undeploy cockroachdb
kubectl delete -f ./k8s/cockroachdb/cockroachdb.yml
kubectl delete -f ./k8s/cockroachdb/crdb-test-pod.yml
```

## Docker Compose

### Run using docker-compose

```bash
./scripts/start-docker-compose-authentik.sh
```

## Run POC 

```bash
cd gotest
make run
```
