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

## CI deploy (`.github/workflows/ci.yml`, job `deploy`)
- Push to `main` only, and only after the `test` job passes (`needs: test`); runs `deploy/k8s/deploy.sh` unchanged, with `BLOOM_HOST=$ORACLE_SSH_USER@$ORACLE_SSH_HOST` instead of the local `bloom-oracle` SSH alias.
- Needs repo secrets `ORACLE_SSH_HOST`, `ORACLE_SSH_USER`, `ORACLE_SSH_KEY` (a CI-only key, separate from the operator's personal `~/.ssh/bloom-oracle`, so it can be revoked on its own). The key is written to a throwaway file on the ephemeral runner, never printed, loaded into `ssh-agent`.
- `concurrency: deploy-oracle` (no cancel-in-progress) serializes deploys if two merges land close together; a half-finished rollout on the single VM must not be interrupted by another one starting.
- Relies on the `ubuntu` user's passwordless `sudo` (Oracle's cloud-init default) — the deploy/secrets scripts already assume this for the manual path.
