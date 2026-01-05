# Installation Guide

## Prerequisites

- Kubernetes v1.24+
- Pi-hole v6.x with API access
- `kubectl` configured to access your cluster

## Quick Install

```bash
# 1. Create namespace
kubectl create namespace pihole-ingress-operator-system

# 2. Create secret with Pi-hole password
kubectl create secret generic pihole-ingress-operator-pihole-operator-secret \
  --namespace pihole-ingress-operator-system \
  --from-literal=PIHOLE_PASSWORD='your-pihole-password'

# 3. Create ConfigMap with Pi-hole URL and target IP
kubectl create configmap pihole-ingress-operator-pihole-operator-config \
  --namespace pihole-ingress-operator-system \
  --from-literal=PIHOLE_URL='http://your-pihole-ip' \
  --from-literal=DEFAULT_TARGET_IP='your-ingress-controller-ip' \
  --from-literal=LOG_LEVEL='info'

# 4. Apply the operator manifest
kubectl apply -f https://github.com/rsjames-ttrpg/pihole-ingress-operator/releases/latest/download/install.yaml
```

## Configuration

### Required Settings

| Setting | Description | Example |
|---------|-------------|---------|
| `PIHOLE_URL` | Base URL of your Pi-hole instance | `http://192.168.1.2` |
| `PIHOLE_PASSWORD` | Pi-hole web interface password | `your-password` |
| `DEFAULT_TARGET_IP` | IP address for DNS A records (your ingress controller) | `192.168.1.100` |

### Optional Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `WATCH_NAMESPACE` | `""` | Namespace to watch (empty = all namespaces) |

## Usage

Annotate your Ingress resources to register DNS records:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    pihole.io/register: "true"
spec:
  rules:
  - host: app.home.local
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: my-app
            port:
              number: 80
```

### Available Annotations

| Annotation | Required | Description |
|------------|----------|-------------|
| `pihole.io/register` | Yes | Set to `"true"` to enable DNS registration |
| `pihole.io/target-ip` | No | Override the default target IP for this Ingress |
| `pihole.io/hosts` | No | Comma-separated list of hostnames (overrides spec.rules) |

## Verify Installation

```bash
# Check operator is running
kubectl get pods -n pihole-ingress-operator-system

# View operator logs
kubectl logs -n pihole-ingress-operator-system -l control-plane=controller-manager -f

# Check if DNS record was created (on Pi-hole)
# Navigate to Pi-hole Admin > Local DNS > DNS Records
```

## Uninstall

```bash
kubectl delete -f https://github.com/rsjames-ttrpg/pihole-ingress-operator/releases/latest/download/install.yaml
kubectl delete namespace pihole-ingress-operator-system
```
