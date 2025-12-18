#!/bin/bash
# Bootstrap script - Run this ONCE to set up the local development cluster
# Usage: ./bootstrap.sh

set -euo pipefail

echo "=========================================="
echo "DEV-PROJECT Bootstrap Setup"
echo "=========================================="

# 1. Install Kind if not present
if ! command -v kind &> /dev/null; then
    echo "Installing Kind..."
    curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
    chmod +x ./kind
    sudo mv ./kind /usr/local/bin/
fi

# 2. Install kubectl if not present
if ! command -v kubectl &> /dev/null; then
    echo "Installing kubectl..."
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod +x kubectl
    sudo mv kubectl /usr/local/bin/
fi

# 3. Install Helm if not present
if ! command -v helm &> /dev/null; then
    echo "Installing Helm..."
    curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
fi

# 4. Create Kind cluster
echo "Creating Kind cluster..."
kind delete cluster --name dev 2>/dev/null || true
kind create cluster --name dev --config kind-config.yaml

# 5. Wait for cluster to be ready
echo "Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready node --all --timeout=300s

# 6. Install StorageClass
echo "Installing StorageClass..."
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml
kubectl patch storageclass local-path -p '{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'

# 7. Install Ingress NGINX
echo "Installing Ingress NGINX..."
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=120s

# 8. Add Helm repos
echo "Adding Helm repositories..."
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# 9. Install PostgreSQL
echo "Installing PostgreSQL..."
helm upgrade --install postgres bitnami/postgresql -f gitops/infra/postgres/values-dev.yaml

# 10. Wait for PostgreSQL
echo "Waiting for PostgreSQL to be ready..."
kubectl wait --for=condition=ready pod --selector=app.kubernetes.io/name=postgresql --timeout=300s

# 11. Wait for PostgreSQL to accept connections
echo "Waiting for PostgreSQL to accept connections..."
for i in {1..60}; do
  if kubectl exec -it deployment/postgres-postgresql -- pg_isready -U appuser -d appdb &>/dev/null; then
    echo "PostgreSQL is ready!"
    break
  fi
  echo "Attempt $i: PostgreSQL not ready yet, waiting..."
  sleep 2
done

# 12. Run migrations with retry
echo "Running database migrations..."
MIGRATION_SQL=$(cat user-service/migrations/001_init.sql)

for i in {1..5}; do
  echo "Migration attempt $i of 5..."
  if kubectl run psql-migrate-$i --rm -i --restart=Never --image=bitnami/postgresql:latest -- \
    psql "postgresql://appuser:apppass@postgres-postgresql:5432/appdb?sslmode=disable" \
    -c "$MIGRATION_SQL" 2>/dev/null; then
    echo "✅ Migration successful!"
    break
  else
    echo "Migration failed, retrying in 5 seconds..."
    sleep 5
  fi
done

# 13. Add api.local to hosts
echo "Adding api.local to /etc/hosts..."
if ! grep -q "127.0.0.1 api.local" /etc/hosts; then
    echo "127.0.0.1 api.local" | sudo tee -a /etc/hosts
fi

# 14. Build and load Docker images
echo "Building Docker images..."
docker build -t user-service:dev ./user-service
docker build -t auth-service:dev ./auth-service

echo "Loading images into Kind..."
kind load docker-image user-service:dev --name dev
kind load docker-image auth-service:dev --name dev

# 15. Apply Kubernetes manifests
echo "Applying Kubernetes manifests..."
kubectl apply -k gitops/apps/user-service/overlays/dev
kubectl apply -k gitops/apps/auth-service/overlays/dev
kubectl apply -k gitops/apps/api-ingress/overlays/dev

# 16. Wait for services to be ready
echo "Waiting for services to be ready..."
kubectl wait --for=condition=ready pod --selector=app=user-service --timeout=120s 2>/dev/null || true
kubectl wait --for=condition=ready pod --selector=app=auth-service --timeout=120s 2>/dev/null || true

echo "=========================================="
echo "✅ Bootstrap complete!"
echo "=========================================="
echo ""
echo "Test the services:"
echo "  curl -i http://api.local/api/v1/users -H 'Content-Type: application/json' -d '{\"username\":\"ahmed\",\"password\":\"supersecret1\"}'"
echo "  curl -i http://api.local/api/v1/login -H 'Content-Type: application/json' -d '{\"username\":\"ahmed\",\"password\":\"supersecret1\"}'"
