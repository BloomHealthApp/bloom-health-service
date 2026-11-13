# deploy/k8s/

## Purpose
Single-node k3s deployment on an Oracle Cloud Always Free Ampere (arm64) VM. `README.md` is the runbook.

## Rules
- Images are built on the VM (native arm64) with Docker and imported into k3s's containerd; pods use `imagePullPolicy: Never`. Docker runs with `iptables: false` and builds with `--network host` so it can't break k3s networking.
- No public ingress: Traefik and servicelb are disabled, the Oracle security list allows SSH only. Public traffic will come through Cloudflare Tunnel from inside the `bloom` namespace (the NetworkPolicy already allows same-namespace pods).
- The Deployment has no `replicas` field: the HPA owns it (2–4).
- Secrets come from the gitignored `deploy/k8s/.env.secrets` via `secrets.sh`; values must match the Vercel deployment while both run.

## Gotchas
- Oracle Ubuntu images reject FORWARD traffic and most INPUT in `/etc/iptables/rules.v4`; `setup-vm.sh` allows the k3s pod and service CIDRs and removes the FORWARD reject.
- Per-IP rate limits are per pod, so the effective limit scales with the replica count.
- Once Cloudflare Tunnel fronts the API, every request comes from the cloudflared pod: client IPs must come from `CF-Connecting-IP` before rate limits are meaningful.
