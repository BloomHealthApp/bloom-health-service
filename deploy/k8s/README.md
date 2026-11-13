# Deploying to Oracle Cloud (k3s)

Runs bloom-health-service on a single Oracle Cloud **Always Free** Ampere (arm64) VM with k3s. Nothing is exposed
publicly yet: the VM accepts SSH only, and the API is reached through a port-forward until Cloudflare Tunnel is added.

```
Mac ──ssh──▶ Oracle VM (Ubuntu 24.04 arm64, reserved public IP, inbound: SSH only)
              ├─ docker      builds the image natively (arm64), never runs workloads
              └─ k3s         namespace bloom
                  ├─ Deployment bloom-health-service (2–4 pods, HPA on CPU)
                  ├─ Service ClusterIP :3000, PodDisruptionBudget, NetworkPolicy
                  └─ Secret  bloom-health-service (from .env.secrets)
                        │
                        └──▶ MongoDB Atlas (allowlist: the reserved IP), SMTP, Google keys
```

| File | Runs on | Purpose |
|---|---|---|
| `setup-vm.sh` | VM, once | Packages, Oracle firewall fix, Docker (builds only), k3s |
| `secrets.sh` | Mac | Loads `.env.secrets` into the cluster over SSH |
| `deploy.sh` | Mac | Syncs this checkout, builds on the VM, imports into k3s, rolls out, smoke-tests |
| `base/` | — | Kustomize manifests |

## 1. Oracle account and VM (console, once)
1. Sign up at cloud.oracle.com. **Pick the home region carefully: it can't be changed**, and Always Free Ampere VMs
   only run there. Mumbai or Hyderabad are close to Indian users.
2. Optional but recommended: upgrade to **Pay As You Go** (Billing → Upgrade). Always Free resources stay free, and
   Oracle stops reclaiming idle free VMs. Then add a **budget alert** (Billing → Budgets) at a tiny amount.
3. Networking → **VCN Wizard → Create VCN with Internet Connectivity** (defaults are fine).
4. In the VCN's public subnet **Security List**, keep only the SSH rule and narrow its source from `0.0.0.0/0` to your
   own IP (`curl -s ifconfig.me`) if your IP is stable. Do **not** open 80, 443 or 6443.
5. Create an SSH key on the Mac for this server only:
   ```bash
   ssh-keygen -t ed25519 -f ~/.ssh/bloom-oracle -C bloom-oracle
   ```
6. Compute → Instances → **Create instance**:
   - Image: **Canonical Ubuntu 24.04** (aarch64). Shape: **VM.Standard.A1.Flex**, 2 OCPU / 12 GB (leaves half the free
     allowance for a second node or the worker; 4 / 24 also works).
   - Networking: the public subnet from step 3, assign a public IPv4.
   - SSH keys: paste `~/.ssh/bloom-oracle.pub`. Boot volume: 50–100 GB (200 GB is free in total).
   - "Out of capacity" is common for A1: retry later, try another availability domain, or a smaller shape.
7. Make the IP permanent: Instance → **Attached VNICs → IPv4 addresses → Edit → Reserved public IP → Create**.
8. MongoDB Atlas → Network Access → **add the reserved IP**.

Add an SSH alias on the Mac (`~/.ssh/config`), which the scripts use:
```
Host bloom-oracle
  HostName <reserved IP>
  User ubuntu
  IdentityFile ~/.ssh/bloom-oracle
  IdentitiesOnly yes
```

## 2. Bootstrap the VM (once)
```bash
scp deploy/k8s/setup-vm.sh bloom-oracle:
```
```bash
ssh bloom-oracle 'sudo bash setup-vm.sh'
```
It prints the node as `Ready`. Re-running it is safe.

## 3. Secrets (once, and whenever a value changes)
```bash
cp deploy/k8s/secrets.example deploy/k8s/.env.secrets
```
Fill it with the **same values as the Vercel deployment** (Vercel → Project → Settings → Environment Variables), so
both hosts share the database and existing sessions keep working. No quotes around values. Then:
```bash
deploy/k8s/secrets.sh
```

## 4. Deploy (every release)
```bash
deploy/k8s/deploy.sh
```
The image tag is the commit (plus `-dirty-<time>` for uncommitted changes). The rollout adds a pod before removing
one, waits for readiness, and ends with `/livez` and `/health` checks through a port-forward.

To call the API from the Mac before Cloudflare Tunnel exists:
```bash
ssh -L 3000:localhost:3000 bloom-oracle 'sudo k3s kubectl -n bloom port-forward svc/bloom-health-service 3000:3000'
```
then `curl localhost:3000/health` in another terminal.

## Operating it
| Task | Command (prefix with `ssh bloom-oracle`) |
|---|---|
| Pods and versions | `sudo k3s kubectl -n bloom get pods -o wide` |
| Logs (JSON, one line per request) | `sudo k3s kubectl -n bloom logs deploy/bloom-health-service -f` |
| Roll back one release | `sudo k3s kubectl -n bloom rollout undo deploy/bloom-health-service` |
| Restart after a secret change | `sudo k3s kubectl -n bloom rollout restart deploy/bloom-health-service` |
| Autoscaler state | `sudo k3s kubectl -n bloom get hpa` |

## Design notes
- **Probes:** liveness uses `/livez` (no dependencies), readiness uses `/health` (pings MongoDB). A database outage
  takes pods out of rotation without restart loops. Successful probes are not request-logged.
- **Zero-downtime rollout:** `maxUnavailable: 0`, a PodDisruptionBudget of 1, a 5 s `preStop` sleep before the server's
  10 s graceful shutdown, and a 30 s termination grace period.
- **Hardening:** restricted Pod Security Standard on the namespace, non-root distroless image, read-only root
  filesystem, no capabilities, no service-account token, secrets encrypted at rest in k3s, and a NetworkPolicy that
  only lets pods in `bloom` connect.
- **Memory:** `GOMEMLIMIT` follows the container memory limit so the Go GC works before the kernel OOM-kills.
- **Docker next to k3s:** Docker is only a builder, with `iptables: false` and `--network host` builds, so it never
  rewrites the firewall rules k3s depends on. Images are imported into k3s's containerd (`imagePullPolicy: Never`), so
  there is no registry to secure yet.
- **Known limits:** one VM means no node redundancy (keep Vercel deployable as a fallback); the in-process rate
  limiters count per pod, so with N pods a client gets up to N× the per-IP limit.

## Next
Cloudflare Tunnel (`cloudflared` in the `bloom` namespace) for public HTTPS with no open ports, then reading the
client IP from `CF-Connecting-IP`; GitHub Actions building multi-arch images to GHCR; Prometheus and Grafana.
