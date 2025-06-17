
### We have added automation to initialize and configure the vault for in-cluster. We need to perform below steps to onboard a new out-cluster

Multi-cluster Vault authentication setup for External Secrets Operator.

## Setup Instructions

Replace `asus-server` and `asus-cluster` with your actual cluster names throughout.

### 1. Prepare Target Cluster

```bash
# Switch to target cluster
kubectl config use-context asus-server

# Create namespace and service accounts
kubectl create ns vault
kubectl create serviceaccount vault-auth -n vault
kubectl create clusterrolebinding vault-auth-tokenreview-binding \
    --clusterrole=system:auth-delegator \
    --serviceaccount=vault:vault-auth

kubectl create serviceaccount external-secrets -n external-secrets
```

### 2. Extract Cluster Information

```bash
# Get cluster API server URL
kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}'

# Extract CA certificate
kubectl config view --raw --minify -o jsonpath='{.clusters[0].cluster.certificate-authority-data}' | base64 --decode > /tmp/asus-ca.crt

# Generate service account token
kubectl create token vault-auth -n vault --duration=8760h > /tmp/vault-asus-reviewer-token.jwt
```

### 3. Copy Files to Vault Pod

```bash
# Switch to Vault cluster
kubectl config use-context rancher-master

# Copy files to Vault pod
kubectl -n vault cp /tmp/asus-ca.crt rancher-master-vault-0:/tmp/asus-ca.crt
kubectl -n vault cp /tmp/vault-asus-reviewer-token.jwt rancher-master-vault-0:/tmp/vault-asus-reviewer-token.jwt
```

### 4. Configure Vault Authentication

```bash
# Exec into Vault pod
kubectl -n vault exec -it rancher-master-vault-0 -- sh

# Inside Vault pod:
# Enable Kubernetes auth for this cluster
vault auth enable -path=asus-cluster kubernetes

# Set cluster API server URL (update with your cluster's IP)
KUBE_HOST="https://192.168.90.111:6443"

# Configure the auth method
vault write auth/asus-cluster/config \
    kubernetes_host="$KUBE_HOST" \
    kubernetes_ca_cert=@/tmp/asus-ca.crt \
    token_reviewer_jwt=@/tmp/vault-asus-reviewer-token.jwt

# Verify configuration
vault read auth/asus-cluster/config

# Create role for External Secrets
vault write auth/asus-cluster/role/external-secrets \
    bound_service_account_names=external-secrets \
    bound_service_account_namespaces=external-secrets \
    policies=readonly-policy \
    ttl=1h
```

### 5. Verify Setup

```bash
# List auth methods to confirm
vault auth list

# Check role configuration
vault read auth/asus-cluster/role/external-secrets
```

## Customization

- **Cluster names**: Replace `asus-server` (kubeconfig) and `asus-cluster` (Vault path)
- **Vault pod name**: Update `rancher-master-vault-0` to match your pod
- **API server URL**: Update `KUBE_HOST` with your cluster's actual endpoint
- **Policies**: Adjust `readonly-policy` to match your Vault policy names

## Troubleshooting

- Ensure service account tokens have sufficient duration (8760h = 1 year)
- Verify cluster API server is accessible from Vault pod
- Check that CA certificate matches the cluster's actual CA
- Confirm policies exist in Vault before assigning to roles

## Repeat for Additional Clusters

For each new cluster, repeat steps 1-4 with different cluster names and paths.
