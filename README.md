# Pi-hole Ingress Operator

A Kubernetes operator that automatically registers DNS records in Pi-hole based on Ingress annotations. When you annotate an Ingress with `pihole.io/register: "true"`, the operator extracts hostnames and creates corresponding DNS A records in your Pi-hole instance.

## How It Works

```
┌─────────────────┐     watch      ┌──────────────────┐
│   K8s Ingress   │ ─────────────► │     Operator     │
│   Resources     │                │                  │
└─────────────────┘                └────────┬─────────┘
                                            │
                                            │ Pi-hole v6 API
                                            ▼
                                   ┌──────────────────┐
                                   │     Pi-hole      │
                                   │   Local DNS      │
                                   └──────────────────┘
```

The operator watches for Ingress resources with the `pihole.io/register: "true"` annotation and:

1. Extracts hostnames from `spec.rules[].host`
2. Creates DNS A records in Pi-hole pointing to your ingress controller IP
3. Cleans up records when the Ingress is deleted or annotation removed
4. Tracks managed records to avoid conflicts with manually-created entries

## Prerequisites

- Kubernetes v1.24+
- Pi-hole v6.x with API access
- An ingress controller with a known IP address

## Installation

### 1. Create the namespace and secret

```bash
kubectl create namespace pihole-operator

kubectl create secret generic pihole-operator-secret \
  --namespace pihole-operator \
  --from-literal=PIHOLE_PASSWORD='your-pihole-password'
```

### 2. Create the ConfigMap

```bash
kubectl create configmap pihole-operator-config \
  --namespace pihole-operator \
  --from-literal=PIHOLE_URL='http://192.168.1.2' \
  --from-literal=DEFAULT_TARGET_IP='192.168.1.100' \
  --from-literal=LOG_LEVEL='info'
```

### 3. Deploy the operator

```bash
make deploy IMG=ghcr.io/rsjames-ttrpg/pihole-ingress-operator:latest
```

Or using the install manifest:

```bash
kubectl apply -f https://raw.githubusercontent.com/rsjames-ttrpg/pihole-ingress-operator/main/dist/install.yaml
```

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PIHOLE_URL` | Yes | - | Base URL of Pi-hole instance (e.g., `http://192.168.1.2`) |
| `PIHOLE_PASSWORD` | Yes | - | Pi-hole web interface password |
| `DEFAULT_TARGET_IP` | Yes | - | Default IP for DNS A records (your ingress controller IP) |
| `LOG_LEVEL` | No | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `WATCH_NAMESPACE` | No | `""` | Namespace to watch (empty = all namespaces) |

## Usage

### Basic Usage

Add the `pihole.io/register: "true"` annotation to your Ingress:

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

This creates a DNS record: `app.home.local → 192.168.1.100` (your configured default IP)

### Annotations

| Annotation | Required | Default | Description |
|------------|----------|---------|-------------|
| `pihole.io/register` | Yes | - | Set to `"true"` to enable DNS registration |
| `pihole.io/target-ip` | No | `DEFAULT_TARGET_IP` | Override the target IP for this Ingress |
| `pihole.io/hosts` | No | from `spec.rules` | Comma-separated list of hostnames to register |

### Override Target IP

```yaml
metadata:
  annotations:
    pihole.io/register: "true"
    pihole.io/target-ip: "10.0.0.50"
```

### Specify Custom Hostnames

```yaml
metadata:
  annotations:
    pihole.io/register: "true"
    pihole.io/hosts: "api.local,web.local,admin.local"
```

## Development

### Run Locally

```bash
# Set required environment variables
export PIHOLE_URL="http://192.168.1.2"
export PIHOLE_PASSWORD="your-password"
export DEFAULT_TARGET_IP="192.168.1.100"

# Run against your current kubeconfig context
make run
```

### Build and Test

```bash
# Run tests
make test

# Run linter
make lint

# Build binary
make build

# Build container image
make docker-build IMG=pihole-operator:dev
```

### Project Structure

```
├── cmd/
│   └── main.go                  # Entrypoint
├── internal/
│   ├── config/                  # Configuration loading
│   ├── controller/              # Ingress reconciliation logic
│   └── pihole/                  # Pi-hole v6 API client
├── config/
│   ├── manager/                 # Deployment manifests
│   └── rbac/                    # RBAC configuration
└── Makefile
```

## Limitations

- **Single Pi-hole instance**: The operator targets one Pi-hole at a time
- **No finalizers**: If the operator is down when an Ingress is deleted, orphaned DNS records may remain
- **A records only**: CNAME records are not supported
- **Pi-hole v6 only**: Uses the v6 REST API (not compatible with v5.x)

## Troubleshooting

### Check operator logs

```bash
kubectl logs -n pihole-operator -l control-plane=controller-manager -f
```

### Verify Pi-hole connectivity

```bash
kubectl exec -n pihole-operator deploy/controller-manager -- wget -q -O- http://your-pihole/api/auth
```

### Common issues

**401 Unauthorized**: Check that `PIHOLE_PASSWORD` is correct

**Connection refused**: Verify `PIHOLE_URL` is reachable from the cluster

**Records not created**: Ensure the Ingress has `pihole.io/register: "true"` annotation

## License

Apache License 2.0
