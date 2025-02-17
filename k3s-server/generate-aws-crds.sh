#!/bin/bash
set -e


echo "The script last ran on `date`"
apk add --no-cache jq curl libc6-compat

# Step 1: Download aws_signing_helper tool
echo "Downloading aws_signing_helper..."
curl -Lo /usr/local/bin/aws_signing_helper https://rolesanywhere.amazonaws.com/releases/1.4.0/X86_64/Linux/aws_signing_helper 
chmod +x /usr/local/bin/aws_signing_helper

#echo $PATH
#ldd /usr/local/bin/aws_signing_helper

# Step 2: Load required values from the mounted secrets
echo "Loading secrets from the mounted volume..."
CLIENT_CERT=/etc/aws-roles-anywhere/client.crt
PRIVATE_KEY=/etc/aws-roles-anywhere/client.key
PROFILE_ARN=$(cat /etc/aws-roles-anywhere/profile-arn)
TRUST_ANCHOR_ARN=$(cat /etc/aws-roles-anywhere/trust-anchor-arn)
ROLE_ARN=$(cat /etc/aws-roles-anywhere/role-arn)

# Step 3: Generate temporary AWS credentials using aws_signing_helper
echo "Generating temporary AWS credentials..."
credentials=$(/usr/local/bin/aws_signing_helper credential-process \
  --certificate $CLIENT_CERT \
  --private-key $PRIVATE_KEY \
  --trust-anchor-arn $TRUST_ANCHOR_ARN \
  --profile-arn $PROFILE_ARN \
  --role-arn $ROLE_ARN)

echo $credentials

echo " "

# Step 4: Parse credentials and write to the AWS credentials file
cat <<EOF > /shared-aws-credentials/aws-credentials
[default]
aws_access_key_id=$(echo $credentials | jq -r '.AccessKeyId')
aws_secret_access_key=$(echo $credentials | jq -r '.SecretAccessKey')
aws_session_token=$(echo $credentials | jq -r '.SessionToken')
EOF

echo "Temporary AWS credentials saved successfully." 
echo "The script last completed on `date`"
