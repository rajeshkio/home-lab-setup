#!/bin/sh
set -e

echo "The script last ran on `date`"
apk add --no-cache jq curl libc6-compat

# Step 1: Download aws_signing_helper tool
echo "Downloading aws_signing_helper..."
curl -Lo /usr/local/bin/aws_signing_helper https://rolesanywhere.amazonaws.com/releases/1.4.0/X86_64/Linux/aws_signing_helper 
chmod +x /usr/local/bin/aws_signing_helper

# Extract certs from environment variables or files
CLIENT_CERT=$(cat /etc/aws-roles-anywhere/client.crt)
PRIVATE_KEY=$(cat /etc/aws-roles-anywhere/client.key)
PROFILE_ARN=$(cat /etc/aws-roles-anywhere/profile-arn)
ROLE_ARN=$(cat /etc/aws-roles-anywhere/role-arn)
TRUST_ANCHOR_ARN=$(cat /etc/aws-roles-anywhere/trust-anchor-arn)
PROFILE_ARN=$(cat /etc/aws-roles-anywhere/profile-arn)
AWS_REGION=${AWS_REGION:-ap-south-1}


# Save to temp files for aws_signing_helper
echo "$PRIVATE_KEY" > /tmp/private-key.pem
echo "$CLIENT_CERT" > /tmp/certificate.pem

# Generate temporary credentials
CREDS=$(/usr/local/bin/aws_signing_helper credential-process \
  --private-key /tmp/private-key.pem \
  --certificate /tmp/certificate.pem \
  --trust-anchor-arn $TRUST_ANCHOR_ARN \
  --profile-arn $PROFILE_ARN \
  --role-arn $ROLE_ARN)

# Parse the credentials
ACCESS_KEY=$(echo $CREDS | jq -r '.AccessKeyId')
SECRET_KEY=$(echo $CREDS | jq -r '.SecretAccessKey')
SESSION_TOKEN=$(echo $CREDS | jq -r '.SessionToken')

# Write the credentials file
mkdir -p /shared-aws-credentials
cat > /shared-aws-credentials/aws-credentials << AWSCREDS
[default]
aws_access_key_id = $ACCESS_KEY
aws_secret_access_key = $SECRET_KEY
aws_session_token = $SESSION_TOKEN
AWSCREDS

echo "AWS credentials generated successfully"

# Clean up
rm -f /tmp/private-key.pem /tmp/certificate.pem
