# Community Golden Paths

Example `goldenpath.yml` files for common engineering contexts. Use them as-is or as a starting point for your own.

| File | Who it's for |
|------|-------------|
| `platform-baseline.yml` | Platform/SRE teams — the community root; other paths extend this |
| `kubernetes-workloads.yml` | Backend service teams on K8s — extends platform-baseline |
| `fintech-regulated.yml` | Payments, identity, regulated services — PCI/SOC 2 oriented |
| `solo-engineer.yml` | Solo engineers and small teams — standalone, low noise |

## How to use

```bash
# Use directly
infragenie review --goldenpath examples/golden-paths/platform-baseline.yml --diff HEAD~1

# Extend in your own repo
cat > goldenpath.yml <<EOF
version: 1
extends: ./platform-baseline.yml

# add your overrides here
security:
  require_network_policy: true
EOF
```

## Inheritance

`kubernetes-workloads.yml` and `fintech-regulated.yml` both extend `platform-baseline.yml` using the `extends:` key. The child file's fields take precedence — you only need to specify what changes.

```
platform-baseline.yml
├── kubernetes-workloads.yml   (adds network policy, Helm validation, cost-centre label)
└── fintech-regulated.yml      (adds SAST, secret scan, escalates severities to critical)

solo-engineer.yml              (standalone — no parent)
```
