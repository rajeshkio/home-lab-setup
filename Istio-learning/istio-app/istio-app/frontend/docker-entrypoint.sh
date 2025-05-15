#!/bin/sh
# docker-entrypoint.sh

# Default value for API_SERVICE_URL if not set
export API_SERVICE_URL=${API_SERVICE_URL:-http://api-service:5000}

# Process the Nginx configuration template
envsubst '${API_SERVICE_URL}' < /etc/nginx/templates/default.conf.template > /etc/nginx/conf.d/default.conf

# Execute the CMD from the Dockerfile
exec "$@"
