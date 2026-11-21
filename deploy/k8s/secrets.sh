#!/usr/bin/env bash
# Creates or updates the bloom-health-service Secret from deploy/k8s/.env.secrets, over SSH, without printing values.
# Usage: deploy/k8s/secrets.sh            (BLOOM_HOST defaults to the ssh alias "bloom-oracle")
set -euo pipefail
cd "$(dirname "$0")"
HOST="${BLOOM_HOST:-bloom-oracle}"
FILE=.env.secrets
[[ -f $FILE ]] || { echo "missing deploy/k8s/$FILE (copy secrets.example)" >&2; exit 1; }

for key in MONGODB_URI ACCESS_TOKEN_SECRET SMTP_HOST SMTP_USER SMTP_PASS MAIL_FROM; do
  grep -Eq "^${key}=.+" $FILE || { echo "$key is empty in $FILE" >&2; exit 1; }
done

# Drop comments and empty values so unset optional features stay off instead of being set to "".
grep -E '^[A-Z_]+=.+' $FILE | ssh "$HOST" 'set -e
  tmp=$(mktemp); trap "rm -f $tmp" EXIT; chmod 600 $tmp; cat > $tmp
  sudo k3s kubectl create namespace bloom --dry-run=client -o yaml | sudo k3s kubectl apply -f - >/dev/null
  sudo k3s kubectl -n bloom create secret generic bloom-health-service --from-env-file=$tmp --dry-run=client -o yaml |
    sudo k3s kubectl apply -f -'
echo "secret updated; restart pods to pick it up: ssh $HOST sudo k3s kubectl -n bloom rollout restart deploy/bloom-health-service"
