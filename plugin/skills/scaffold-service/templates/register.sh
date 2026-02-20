#!/bin/sh
set -e

CADDY_ADMIN_URL="${CADDY_ADMIN_URL:-http://caddy-admin-api:8090}"
SERVICE_NAME="${SERVICE_NAME:-__PROJECT_NAME__}"
SERVICE_DOMAIN="${SERVICE_DOMAIN:-__PROJECT_NAME__.yeanhua.asia}"
SERVICE_UPSTREAM="${SERVICE_UPSTREAM:-__PROJECT_NAME__-frontend:80}"

echo "Waiting for caddy-admin API at ${CADDY_ADMIN_URL}..."

RETRIES=30
for i in $(seq 1 $RETRIES); do
  if curl -sf "${CADDY_ADMIN_URL}/api/status" > /dev/null 2>&1; then
    echo "caddy-admin API is ready."
    break
  fi
  if [ "$i" -eq "$RETRIES" ]; then
    echo "ERROR: caddy-admin API not ready after ${RETRIES} attempts."
    exit 1
  fi
  echo "  attempt $i/${RETRIES}..."
  sleep 2
done

echo "Registering service: ${SERVICE_NAME} -> ${SERVICE_DOMAIN} -> ${SERVICE_UPSTREAM}"

RESPONSE=$(curl -sf -X POST "${CADDY_ADMIN_URL}/api/services" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"${SERVICE_NAME}\",\"domain\":\"${SERVICE_DOMAIN}\",\"upstream\":\"${SERVICE_UPSTREAM}\"}")

echo "Response: ${RESPONSE}"
echo "${SERVICE_NAME} registered successfully."
