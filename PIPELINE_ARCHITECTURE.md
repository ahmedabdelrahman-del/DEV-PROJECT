# Pipeline Architecture Guide

## Overview
Your project follows the **GitOps pattern** with Jenkins CI and ArgoCD-ready infrastructure.

---

## Pipeline Responsibilities

### **Jenkins Pipeline (One-time setup PER SERVICE)**
**When:** On every git push
**What it does:**
1. Lint & test code (Go fmt, Go vet, Unit tests)
2. Build Docker image
3. Push to Docker Hub
4. Update GitOps manifests with new image tag
5. E2E smoke (kind): spin up disposable kind, deploy dev overlays, run create-user + login flow, then delete cluster

**Files:** `auth-service/Jenkinsfile` and `user-service/Jenkinsfile`

**Why separate pipelines?**
- Each service is independent
- Each service can be deployed/scaled separately
- Easier to debug issues by service

---

### **Bootstrap (One-time setup for CLUSTER)**
**When:** ONCE per development environment
**What it does:**
1. Install Kind (Kubernetes)
2. Create cluster
3. Install Ingress controller
4. Install PostgreSQL database
5. Run database migrations (with readiness + retries)
6. Load local dev images into kind
7. Deploy manifests (dev overlays point to local :dev images)

**Files:** `bootstrap.sh`

**Why separate?**
- Cluster infrastructure is stable
- No need to recreate on every code change
- Should be run by DevOps/Platform team

---

## Deployment Flow

```
Developer pushes code
    ↓
Jenkins Pipeline triggers
    ├─ Lint & test
    ├─ Build image
    ├─ Push to Docker Hub
    └─ Update gitops/apps/[service]/overlays/dev/kustomization.yaml
         with new image tag
    └─ (Optional on branch/main) E2E smoke on disposable kind
    ↓
ArgoCD watches gitops/ repo (optional)
    ├─ Detects new image tag
    └─ Auto-deploys to Kubernetes
    ↓
Services running in cluster
    ├─ user-service:8081
    └─ auth-service:8082
    ↓
Ingress exposes via HTTP
    ├─ api.local/api/v1/users
    └─ api.local/api/v1/login
```

---

## Setup Instructions

### **First Time ONLY:**
```bash
bash bootstrap.sh
```

### **After Code Changes:**
1. Push code to GitHub
2. Jenkins automatically builds & pushes image
3. GitOps manifests update with new image tag
4. (Optional) ArgoCD syncs and deploys

### **Manual Deployment (without ArgoCD):**
```bash
# If not using ArgoCD, manually apply manifests
kubectl apply -k gitops/apps/user-service/overlays/dev
kubectl apply -k gitops/apps/auth-service/overlays/dev
```

---

## What Your Jenkinsfiles Do (Correct!)

✅ **Checkout code** - Get source
✅ **Format check** - Ensure code style
✅ **Vet analysis** - Check for common errors
✅ **Unit tests** - Validate functionality
✅ **Build image** - Create Docker image
✅ **Push image** - Store on Docker Hub
✅ **Update GitOps** - Trigger deployment via git

This is the **correct separation of concerns**. Your pipelines should NOT:
- ❌ Create the cluster
- ❌ Install databases
- ❌ Setup infrastructure

Those are one-time tasks handled by bootstrap.sh.

---

## Testing the Setup

After running `bootstrap.sh`:

```bash
# Create a user
curl -X POST http://api.local/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass123"}'

# Authenticate
curl -X POST http://api.local/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass123"}'
```

---

## Future: GitOps with ArgoCD

To enable auto-deployment via ArgoCD:
1. Fill in placeholders in `gitops/argocd/*.yaml`
2. Deploy ArgoCD to cluster
3. Point ArgoCD at your GitHub repo
4. ArgoCD will auto-sync whenever gitops/ changes

This makes deployments fully automated!
