# Pi-hole Ingress Operator

A Kubernetes operator that automatically registers DNS records in Pi-hole based on Ingress annotations.

## Project Overview

This operator watches Kubernetes Ingress resources and syncs their hostnames to a Pi-hole instance's local DNS. When an Ingress is annotated with `pihole.io/register: "true"`, the operator extracts hostnames from the Ingress rules and creates corresponding DNS A records in Pi-hole pointing to the configured ingress controller IP.

## Architecture

```
┌─────────────────┐     watch      ┌──────────────────┐
│   K8s Ingress   │ ─────────────► │     Operator     │
│   Resources     │                │                  │
└─────────────────┘                └────────┬─────────┘
                                            │
                                            │ HTTP API (v6)
                                            ▼
                                   ┌──────────────────┐
                                   │     Pi-hole      │
                                   │   Local DNS      │
                                   └──────────────────┘
```

## Tech Stack

- **Language**: Go 1.22+
- **K8s Client**: controller-runtime (kubebuilder style)
- **HTTP Client**: net/http (stdlib)
- **Configuration**: ConfigMap + Environment variables
- **Logging**: slog (stdlib structured logging)

## Project Structure

```
pihole-ingress-operator/
├── cmd/
│   └── operator/
│       └── main.go              # Entrypoint
├── internal/
│   ├── controller/
│   │   └── ingress_controller.go # Reconciliation logic
│   ├── pihole/
│   │   ├── client.go            # Pi-hole API client
│   │   └── types.go             # API request/response types
│   └── config/
│       └── config.go            # Configuration loading
├── deploy/
│   ├── configmap.yaml           # Operator configuration
│   ├── deployment.yaml          # Operator deployment
│   ├── rbac.yaml                # ServiceAccount, Role, RoleBinding
│   └── kustomization.yaml       # Kustomize base
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
├── CLAUDE.md
└── SPEC.md
```

## Key Design Decisions

### No CRDs
Configuration is handled entirely via ConfigMap and environment variables. This keeps the operator simple and avoids CRD lifecycle management.

### No Finalizers
The operator does not block Ingress deletion. If an Ingress is deleted while the operator is down, orphaned DNS records may remain in Pi-hole. This is acceptable for a home lab use case – manual cleanup is straightforward.

### Single Pi-hole Instance
The operator targets a single Pi-hole instance. The Pi-hole URL and API token are configured via ConfigMap/Secret.

### Static Target IP
For bare-metal setups, the operator uses a statically configured IP (the ingress controller's IP) as the DNS target. Per-Ingress overrides are supported via annotation.

### Idempotent Reconciliation
The controller follows standard Kubernetes reconciliation patterns. It compares desired state (Ingress annotations) with actual state (Pi-hole DNS records) and syncs accordingly.

## Annotations

| Annotation | Required | Default | Description |
|------------|----------|---------|-------------|
| `pihole.io/register` | Yes | - | Set to `"true"` to enable DNS registration |
| `pihole.io/target-ip` | No | Global default | Override the target IP for this Ingress |
| `pihole.io/hosts` | No | `spec.rules[].host` | Comma-separated list of hostnames to register (overrides spec extraction) |

## Configuration

### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: pihole-operator-config
  namespace: pihole-operator
data:
  PIHOLE_URL: "http://192.168.1.2"
  DEFAULT_TARGET_IP: "192.168.1.100"
  LOG_LEVEL: "info"
```

### Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: pihole-operator-secret
  namespace: pihole-operator
type: Opaque
stringData:
  PIHOLE_API_TOKEN: "your-api-token-here"
```

## Pi-hole v6 API

The operator uses the Pi-hole v6 REST API:

- `GET /admin/api/dns/local` - List local DNS records
- `POST /admin/api/dns/local` - Create a DNS record
- `DELETE /admin/api/dns/local/{domain}` - Delete a DNS record

Authentication is via Bearer token in the Authorization header.

## Development Commands

```bash
# Run locally against current kubeconfig
make run

# Build container image
make docker-build IMG=pihole-operator:dev

# Deploy to cluster
make deploy IMG=pihole-operator:dev

# Run tests
make test

# Generate manifests (if using controller-gen)
make manifests
```

## Code Style

- Use stdlib where possible (slog for logging, net/http for HTTP)
- Strongly typed configuration structs
- Table-driven tests
- Context propagation for cancellation
- Structured logging with consistent field names
- Error wrapping with `fmt.Errorf("context: %w", err)`

## Error Handling

- Pi-hole API failures: Log and requeue with backoff
- Invalid annotations: Log warning, skip Ingress (don't requeue)
- Missing hosts in Ingress: Log debug, no-op

## Metrics (Future)

Potential Prometheus metrics:
- `pihole_operator_reconcile_total{result="success|error"}`
- `pihole_operator_dns_records_synced`
- `pihole_operator_pihole_api_latency_seconds`

## Testing Strategy

- **Unit tests**: Pi-hole client (mock HTTP), config parsing
- **Integration tests**: Controller with envtest (fake K8s API)
- **E2E tests**: Optional, against real Pi-hole in CI
