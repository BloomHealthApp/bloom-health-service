#!/usr/bin/env bash
# One-time bootstrap of an Oracle Cloud Ampere (arm64) Ubuntu VM as a single-node k3s cluster that can also
# build the service image. Run ON the VM:  sudo bash setup-vm.sh
# Idempotent: safe to run again. Optional: K3S_VERSION=v1.xx.y+k3s1 pins k3s (default: the stable channel).
set -euo pipefail

if [[ $EUID -ne 0 ]]; then echo "run with sudo" >&2; exit 1; fi
if [[ "$(uname -m)" != "aarch64" ]]; then echo "warning: expected an arm64 (Ampere) VM, got $(uname -m)" >&2; fi

echo "==> packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq rsync curl iptables-persistent docker.io docker-buildx >/dev/null

echo "==> host firewall (Oracle images reject forwarded and most inbound traffic)"
# Oracle's Ubuntu images ship /etc/iptables/rules.v4 with REJECT rules on INPUT and FORWARD. FORWARD REJECT breaks
# pod networking and INPUT REJECT stops pods reaching the API server on the host. Allow the k3s pod (10.42/16) and
# service (10.43/16) ranges and drop the FORWARD reject. Public inbound stays SSH-only.
RULES=/etc/iptables/rules.v4
if [[ -f $RULES ]]; then
  cp -n $RULES $RULES.orig || true
  awk '
    /^-A FORWARD -j REJECT/ { next }
    /^-A INPUT -s 10\.4[23]\.0\.0\/16 -j ACCEPT$/ { next }
    /^-A INPUT -j REJECT/ && !done { print "-A INPUT -s 10.42.0.0/16 -j ACCEPT"; print "-A INPUT -s 10.43.0.0/16 -j ACCEPT"; done = 1 }
    { print }
  ' $RULES > $RULES.new
  mv $RULES.new $RULES
  iptables-restore < $RULES
fi

echo "==> docker (image builds only; it must not manage iptables next to k3s)"
mkdir -p /etc/docker
cat > /etc/docker/daemon.json <<'JSON'
{ "iptables": false, "ip6tables": false, "log-driver": "local" }
JSON
systemctl enable --now docker >/dev/null
systemctl restart docker

echo "==> k3s"
if ! command -v k3s >/dev/null; then
  # Traefik and the service load balancer are off: nothing is exposed publicly; Cloudflare Tunnel will reach the
  # service from inside the cluster later. Secrets are encrypted at rest in the k3s datastore.
  curl -sfL https://get.k3s.io | INSTALL_K3S_CHANNEL=stable INSTALL_K3S_VERSION="${K3S_VERSION:-}" sh -s - server \
    --disable traefik --disable servicelb --write-kubeconfig-mode 600 --secrets-encryption
fi
until k3s kubectl get nodes 2>/dev/null | grep -q " Ready"; do sleep 3; done

echo "==> done"
k3s kubectl get nodes -o wide
k3s --version | head -1
docker version --format 'docker {{.Server.Version}}'
