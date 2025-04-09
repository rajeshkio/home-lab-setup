# 🚀 Rancher Local Cluster Deployment Guide

This document provides step-by-step instructions for deploying a Rancher local cluster with Cilium, Vault, External Secrets, and cert-manager. Please ensure you replace all placeholder values with your actual configuration data.

## Prerequisites

- SSH access to your target server nodes
- `kubectl` installed on your local machine
- Access to the required AWS resources (if using AWS IAM Roles Anywhere)
- Domain name with ability to manage DNS records

## Step 1: Install K3s on the First Node

```sh
curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.31.6+k3s1 INSTALL_K3S_EXEC="server" sh -
```

## Step 2: Access the K3s Kubeconfig

```sh
cat /etc/rancher/k3s/k3s.yaml
```

## Step 3: Configure KUBECONFIG on Your Local Machine

Copy the `k3s.yaml` content to your local system and export it as `KUBECONFIG`. If the server URL is set to `localhost:6443`, modify the `clusters.cluster.server` value to point to your server's actual IP or hostname.

```sh
export KUBECONFIG=/path/to/your/k3s.yaml
```

## Step 4: Deploy Cilium for CNI and Load Balancing

First, install the Gateway API CRDs:

```sh
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/refs/heads/main/config/crd/experimental/gateway.networking.k8s.io_tlsroutes.yaml
```

Then install and configure Cilium:

```sh
cilium install -f rancher-master/cilium-values.yaml
cilium status
kubectl apply -f rancher-master/cilium-announce.yaml -f cilium-ip-pool.yaml
```

For detailed Cilium configuration with Kubernetes Gateway API, refer to [this guide](https://medium.com/@rk90229/the-complete-guide-to-setting-up-cilium-on-k3s-with-kubernetes-gateway-api-8f78adcddb4d).

## Step 5: Create SSL Certificate Secret

Create the certificate namespace and apply your certificate:

```sh
kubectl create namespace certificate-ns
kubectl apply -f rancher-master/rajesh-kumar-cert.yaml
```

If you don't have SSL certificates, you can generate them using certbot:

```sh
certbot certonly --manual --preferred-challenges dns -d your-domain.com
kubectl create secret tls rajesh-tls-cert --cert=fullchain1.pem --key=privkey1.pem -n certificate-ns
```

## Step 6: Configure Kubernetes Gateway

Create the gateway namespace and configure cross-namespace references:

```sh
kubectl create namespace cilium-gateway
kubectl apply -f rancher-master/refrence-grant.yaml
kubectl apply -f rancher-master/rancher-master-common-gateway.yaml
```

## Step 7: Deploy Rancher

Install Rancher without cert-manager (we'll deploy a custom one later):

```sh
helm install rancher rancher-latest/rancher \
  -n cattle-system \
  -f rancher-master/rancher-helm-values.yaml \
  --create-namespace

kubectl -n cattle-system get pods
kubectl -n cattle-system apply -f rancher-master/rancher-master-http-route.yaml
```

The gateway should now have received an IP address. Map this IP to your domain that has the SSL certificate. Verify the Gateway, HTTPRoute, and Cilium resources (CiliumL2AnnouncementPolicy and CiliumLoadBalancerIPPool) are properly configured.

## Step 8: Deploy External-Secrets Operator

```sh
helm install external-secrets external-secrets/external-secrets \
  -n external-secrets \
  --create-namespace
```

## Step 9: Deploy Vault

Follow the instructions in `rancher-master/Vaults/README.md` to deploy Vault, then create an HTTPRoute:

```sh
kubectl apply -f rancher-master/Vaults/vault-http.yaml
```

## Step 10: Configure Vault with Secret Store

Create a Vault token secret and ClusterSecretStore:

```sh
kubectl apply -f rancher-master/Vaults/vaultTokenSecret.yaml
kubectl apply -f rancher-master/Vaults/clusterSecretStore.yaml
```

## Step 11: Prepare AWS IAM Roles Anywhere Configuration

Retrieve the necessary AWS configuration details:

```sh
export AWS_PROFILE=personal

aws rolesanywhere list-trust-anchors | grep trustAnchorArn
aws iam list-roles | grep -i cert
aws rolesanywhere list-profiles | grep profileArn

# Get your certificates
cat rancher-master/client.crt
cat rancher-master/client.key
```

## Step 12: Configure Vault with AWS Credentials

Access the Vault pod and store the AWS credentials:

```sh
kubectl -n vault exec -it vault-0 -- sh

# Login to Vault
vault login <INITIAL_ROOT_TOKEN>

# Store AWS credentials
vault kv put kv/aws-roles-anywhere \
  client.crt="-----BEGIN CERTIFICATE-----
...certificate content...
  -----END CERTIFICATE-----" \
  client.key="-----BEGIN PRIVATE KEY-----
...key content...
  -----END PRIVATE KEY-----" \
  profile-arn="arn:aws:iam::123456789012:profile/example-profile" \
  trust-anchor-arn="arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/example-anchor" \
  role-arn="arn:aws:iam::123456789012:role/cert-manger-external-dns"
```

For more information on AWS IAM Roles Anywhere, refer to [this guide](https://medium.com/@rk90229/getting-started-with-aws-iam-roles-anywhere-a-step-by-step-guide-8902a9ddee62).

## Step 13: Enable Kubernetes Authentication in Vault

```sh
kubectl -n vault exec -it vault-0 -- sh

# Enable Kubernetes auth for rancher-master
vault auth enable -path=rancher-master kubernetes

# Verify auth methods
vault auth list
```

## Step 14: Create Vault Policies

Prepare policy files locally:

```sh
mkdir -p vault-configs
cd vault-configs

# Policy for cert-manager
cat > cert-manager-policy.hcl << 'EOF'
# Allow cert-manager to create and update secrets
path "kv/*" {
  capabilities = ["create", "update", "read", "list"]
}
EOF

# Policy for external-secrets
cat > external-secrets-policy.hcl << 'EOF'
# Read access for secrets
path "kv/*" {
  capabilities = ["read", "list"]
}
path "kv/data/*" {
  capabilities = ["read", "list"]
}
path "kv/metadata/*" {
  capabilities = ["read", "list"]
}
path "secret/*" {
  capabilities = ["read", "list"]
}
path "secret/data/*" {
  capabilities = ["read", "list"]
}
path "secret/metadata/*" {
  capabilities = ["read", "list"]
}
EOF
```

Copy and apply the policies:

```sh
kubectl -n vault cp cert-manager-policy.hcl vault-0:/tmp/cert-manager-policy.hcl
kubectl -n vault cp external-secrets-policy.hcl vault-0:/tmp/external-secrets-policy.hcl

kubectl -n vault exec -it vault-0 -- sh

# Apply policies
vault policy write cert-manager-policy /tmp/cert-manager-policy.hcl
vault policy write external-secrets-policy /tmp/external-secrets-policy.hcl

# Verify policies
vault policy read cert-manager-policy
vault policy read external-secrets-policy
```

## Step 15: Set Up Service Account for Vault Authentication

```sh
# Create service account
kubectl create serviceaccount vault-auth -n vault

# Configure permissions
kubectl create clusterrolebinding vault-auth-tokenreview-binding \
  --clusterrole=system:auth-delegator \
  --serviceaccount=vault:vault-auth
```

## Step 16: Configure Kubernetes Authentication in Vault

Obtain the required Kubernetes information:

```sh
# Get Kubernetes API server URL
KUBE_HOST=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')

# Get CA certificate
kubectl config view --raw --minify -o jsonpath='{.clusters[0].cluster.certificate-authority-data}' | base64 --decode > /tmp/rancher-ca.crt

# Create a token for the vault-auth service account
kubectl create token vault-auth -n vault --duration=8760h > /tmp/vault-reviewer-token.jwt

# Copy files to Vault pod
kubectl -n vault cp /tmp/rancher-ca.crt vault-0:/tmp/rancher-ca.crt
kubectl -n vault cp /tmp/vault-reviewer-token.jwt vault-0:/tmp/vault-rancher-reviewer-token.jwt
```

Configure Vault:

```sh
kubectl -n vault exec -it vault-0 -- sh

# Set your actual Kubernetes API server URL
KUBE_HOST="https://192.168.1.102:6443"

# Configure Kubernetes auth
vault write auth/rancher-master/config \
  kubernetes_host="$KUBE_HOST" \
  kubernetes_ca_cert=@/tmp/rancher-ca.crt \
  token_reviewer_jwt=@/tmp/vault-rancher-reviewer-token.jwt

# Verify configuration
vault read auth/rancher-master/config
```

## Step 17: Create Vault Roles for Service Accounts

```sh
kubectl -n vault exec -it vault-0 -- sh

# Role for cert-manager
vault write auth/rancher-master/role/cert-manager \
  bound_service_account_names=cert-manager \
  bound_service_account_namespaces=cert-manager \
  policies=cert-manager-policy \
  ttl=1h

# Role for external-secrets
vault write auth/rancher-master/role/external-secrets \
  bound_service_account_names=external-secrets \
  bound_service_account_namespaces=external-secrets \
  policies=external-secrets-policy \
  ttl=1h

# Role for cert-pusher
vault write auth/rancher-master/role/cert-pusher \
  bound_service_account_names=cert-pusher \
  bound_service_account_namespaces=cert-manager \
  policies=cert-manager-policy \
  ttl=1h

# Verify roles
vault read auth/rancher-master/role/cert-manager
vault read auth/rancher-master/role/external-secrets
vault read auth/rancher-master/role/cert-pusher
```

## Step 18: Apply External-Secrets Configuration

```sh
kubectl apply -f argocd/kustomize/base/external-secrets-config/externalsecret.yaml
```

## Step 19: Deploy cert-manager

Install cert-manager and apply configuration:

```sh
helm upgrade --install cert-manager bitnami/cert-manager \
  -n cert-manager \
  --create-namespace \
  -f argocd/values/rancher-master/cert-manager/values.yaml

kubectl apply -f argocd/kustomize/base/cert-manager-config/externalsecret.yaml
```

For more information on certificate management with Vault, refer to [this guide](https://medium.com/@rk90229/vault-kubernetes-auth-the-certificate-management-solution-i-wish-id-known-earlier-c90084a4ff10).

## Verification and Troubleshooting

After deployment, verify all components are running correctly:

```sh
# Check Rancher components
kubectl -n cattle-system get pods

# Verify Gateway and HTTPRoute status
kubectl -n cilium-gateway get gateway
kubectl -n cattle-system get httproute

# Check Cilium resources
kubectl get CiliumL2AnnouncementPolicy
kubectl get CiliumLoadBalancerIPPool
```

## Additional Resources

- [Cilium on K3s with Gateway API Guide](https://medium.com/@rk90229/the-complete-guide-to-setting-up-cilium-on-k3s-with-kubernetes-gateway-api-8f78adcddb4d)
- [AWS IAM Roles Anywhere Guide](https://medium.com/@rk90229/getting-started-with-aws-iam-roles-anywhere-a-step-by-step-guide-8902a9ddee62)
- [Vault Certificate Management Solution](https://medium.com/@rk90229/vault-kubernetes-auth-the-certificate-management-solution-i-wish-id-known-earlier-c90084a4ff10)
