# Full Microservices + GitOps Workspace (Ready)

## Fast local run (no Docker Hub / no GitHub needed)

### 1) Create kind cluster
```bash
kind delete cluster --name dev
kind create cluster --name dev --config kind-config.yaml
```

### 2) StorageClass (only if your cluster has none/default missing)
```bash
kubectl get storageclass
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml
kubectl patch storageclass local-path -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
```

### 3) Ingress NGINX for kind
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
```

### 4) Postgres (Helm)
```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
helm upgrade --install postgres bitnami/postgresql -f gitops/infra/postgres/values-dev.yaml
```

### 5) Run migration (one-time)
```bash
kubectl run psql --rm -i --tty --restart=Never --image=bitnami/postgresql:latest --   psql "postgresql://appuser:apppass@postgres-postgresql:5432/appdb?sslmode=disable"   -c "$(cat user-service/migrations/001_init.sql)"
```

### 6) Build + load images into kind
```bash
docker build -t user-service:dev ./user-service
docker build -t auth-service:dev ./auth-service
kind load docker-image user-service:dev --name dev
kind load docker-image auth-service:dev --name dev
```

### 7) Point deployments to local images
Edit these files and set `image:` to `user-service:dev` and `auth-service:dev`:
- `gitops/apps/user-service/base/deployment.yaml`
- `gitops/apps/auth-service/base/deployment.yaml`

### 8) Apply manifests
```bash
kubectl apply -k gitops/apps/user-service/overlays/dev
kubectl apply -k gitops/apps/auth-service/overlays/dev
kubectl apply -k gitops/apps/api-ingress/overlays/dev
```

### 9) Add host entry
Add to hosts file: `127.0.0.1 api.local`

### 10) Test
```bash
curl -i http://api.local/api/v1/users -H "Content-Type: application/json" -d '{"username":"ahmed","password":"supersecret1"}'
curl -i http://api.local/api/v1/login -H "Content-Type: application/json" -d '{"username":"ahmed","password":"supersecret1"}'
```

## GitOps mode (Argo CD)
The `gitops/argocd/*.yaml` files are filled, but they include placeholders (`YOUR_GH_USER`, `YOUR_DOCKERHUB`).
To use them, push `gitops/` to GitHub and replace placeholders.
