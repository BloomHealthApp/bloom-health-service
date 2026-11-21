#!/usr/bin/env bash
# Builds the current checkout on the VM (native arm64), imports the image into k3s and rolls it out.
# Usage: deploy/k8s/deploy.sh            (BLOOM_HOST defaults to the ssh alias "bloom-oracle")
set -euo pipefail
cd "$(dirname "$0")/../.."
HOST="${BLOOM_HOST:-bloom-oracle}"
TAG="$(git rev-parse --short=12 HEAD)"
[[ -z "$(git status --porcelain)" ]] || TAG="$TAG-dirty-$(date +%Y%m%d%H%M%S)"
IMAGE="bloom-health-service:$TAG"
SRC="bloom-build/bloom-health-service"

echo "==> sync source to $HOST:~/$SRC"
ssh "$HOST" "mkdir -p ~/$SRC"
rsync -az --delete --exclude .git --exclude '.env*' --exclude bin --exclude .logs --exclude coverage.out \
  ./ "$HOST:~/$SRC/"

echo "==> build $IMAGE on the VM and import it into k3s"
ssh "$HOST" "set -e; cd ~/$SRC
  sudo docker buildx build --network host --provenance=false -t $IMAGE --load .
  sudo docker save $IMAGE | sudo k3s ctr images import - >/dev/null
  sudo docker image prune -f >/dev/null"

echo "==> apply manifests"
ssh "$HOST" "set -e; cd ~/$SRC/deploy/k8s/base
  sed -i 's/^    newTag: .*/    newTag: $TAG/' kustomization.yaml
  sudo k3s kubectl get -n bloom secret bloom-health-service >/dev/null 2>&1 || { echo 'run deploy/k8s/secrets.sh first' >&2; exit 1; }
  sudo k3s kubectl apply -k .
  sudo k3s kubectl -n bloom rollout status deploy/bloom-health-service --timeout=180s"

echo "==> smoke test through a port-forward"
ssh "$HOST" 'set -e
  sudo k3s kubectl -n bloom port-forward svc/bloom-health-service 13000:3000 >/dev/null 2>&1 & pf=$!
  trap "kill $pf" EXIT; sleep 2
  curl -fsS localhost:13000/livez; echo; curl -fsS localhost:13000/health; echo'
echo "==> deployed $IMAGE"
