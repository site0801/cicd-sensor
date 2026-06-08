# cicd-sensor on Actions Runner Controller (ARC)

Manifests for running cicd-sensor against GitHub Actions jobs scheduled by
[Actions Runner Controller](https://github.com/actions/actions-runner-controller)
(`gha-runner-scale-set`) on Kubernetes.

See [GitHub Actions Runner Controller (ARC)](../../docs/user-guide/github-arc.md)
for the user-facing guide.
The design rationale is in [designs/arc-support.md](../../designs/arc-support.md).

## Files

| File | Applies in | Purpose |
| --- | --- | --- |
| `rbac.yaml` | cicd-sensor namespace | ServiceAccount, Role, RoleBinding for the Agent DaemonSet. Grants `get pods` on the ARC namespace so the Agent can read scale-set labels. |
| `daemonset.yaml` | cicd-sensor namespace | The cicd-sensor Agent DaemonSet. Includes the `install-cli` initContainer that copies the Agent CLI onto the node hostPath. |
| `configmap-hooks.yaml` | each ARC namespace | The `host-start.sh` / `host-end.sh` job hook scripts referenced by the values overlay. |
| `values-overlay.yaml` | each ARC `gha-runner-scale-set` Helm release | Adds the volumes, volumeMounts, and hook env vars to the runner pod template. |

## Apply order

```sh
# 1. Install the Agent on every node.
kubectl create namespace cicd-sensor
kubectl apply -n cicd-sensor -f rbac.yaml
kubectl apply -n cicd-sensor -f daemonset.yaml

# 2. For each ARC namespace that hosts an AutoscalingRunnerSet:
kubectl apply -n <arc-namespace> -f configmap-hooks.yaml

# 3. For each scale set, layer the values overlay on top of your existing
#    gha-runner-scale-set Helm values:
helm upgrade --install <release> \
  oci://ghcr.io/actions/actions-runner-controller-charts/gha-runner-scale-set \
  -n <arc-namespace> \
  -f my-values.yaml \
  -f values-overlay.yaml
```

## What the overlay does and does not do

The values overlay contains plumbing only: volume mounts, the hook env vars, and
the Agent CLI path.
Rules, output destinations, and detection toggles are delivered through
cicd-sensor-manager and do not require a Helm upgrade.

This split is intentional.
Mutating `AutoscalingRunnerSet.spec.template` causes ARC to recreate the
`EphemeralRunnerSet`, listener pod, and pending runner pods, which interrupts
in-flight jobs.
Manager-driven config changes do not touch the template.

## Per-scale-set isolation

The Agent reads `actions.github.com/scale-set-namespace` and
`actions.github.com/scale-set-name` from each runner pod to determine which
host scope configuration to apply.
Different `AutoscalingRunnerSet` resources can carry different rules and output
destinations without affecting each other.
The labels are written by the ARC controller and cannot be forged from inside
the runner pod.

## Prerequisites

- Kubernetes 1.25 or later with cgroup v2 unified hierarchy and the `systemd`
  kubelet cgroup driver.
- ARC chart `gha-runner-scale-set` 0.13 or later.
- A reachable cicd-sensor-manager.
  Set `--manager-url` and the manager bearer token in `daemonset.yaml`
  before applying.
