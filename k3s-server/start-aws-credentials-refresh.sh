#!/bin/bash

# Install necessary packages (cron, inotify-tools, bash)
apk add --no-cache inotify-tools

# Set up the cron job to refresh AWS credentials every 58 minutes
echo '*/58 * * * * /bin/sh /scripts/generate-aws-crds.sh' > /etc/crontabs/root

# Start the cron daemon in the background
crond -f &

# Path to the credentials file being monitored
CREDENTIALS_FILE="/shared-aws-credentials/aws-credentials"

# Monitor the credentials file for changes and send SIGHUP to external-dns process
while inotifywait -e modify "$CREDENTIALS_FILE"; do
    echo "Credentials updated. Sending SIGHUP to external-dns process."
    
    # Send SIGHUP to the external-dns process
    kill -HUP $(pidof external-dns) || true
done
