#!/bin/bash
# deploy.sh — Deploy caddy-admin to ECS (run locally)
# Usage: ECS_IP=1.2.3.4 ./deploy/deploy.sh
set -e

ECS_IP="${ECS_IP:?set ECS_IP}"
ECS_USER="${ECS_USER:-root}"
PROJECT="caddy-admin"
PORT=8090

echo "=== [1/4] Build frontend ==="
cd "$(dirname "$0")/../.."   # repo root (demos/caddy-admin/)
cd caddy-admin/frontend && npm ci && npm run build && cd ../..

echo "=== [2/4] Upload frontend dist ==="
rsync -avz caddy-admin/frontend/dist/ ${ECS_USER}@${ECS_IP}:/var/www/${PROJECT}/dist/

echo "=== [3/4] Upload backend + start ==="
rsync -avz caddy-admin/backend/ ${ECS_USER}@${ECS_IP}:/opt/${PROJECT}/

ssh ${ECS_USER}@${ECS_IP} "
  cd /opt/${PROJECT}
  docker compose -f deploy/docker-compose.yml up -d --build
"

echo "=== [4/4] Update Caddyfile + reload ==="
ssh ${ECS_USER}@${ECS_IP} "
  # Append caddy-admin block if not already present
  if ! grep -q '${PROJECT}.yeanhua.asia' /etc/caddy/Caddyfile; then
    cat /opt/${PROJECT}/deploy/Caddyfile >> /etc/caddy/Caddyfile
    systemctl reload caddy
    echo 'Caddyfile updated and reloaded'
  else
    echo 'Caddyfile already contains this project, skipping'
  fi
"

echo ""
echo "✅ Deployed!"
echo "   https://${PROJECT}.yeanhua.asia"
echo "   https://api.${PROJECT}.yeanhua.asia/api/sites"
