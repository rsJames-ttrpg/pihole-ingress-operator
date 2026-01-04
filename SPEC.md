# Pi-hole Ingress Operator - Technical Specification

## 1. Overview

### 1.1 Purpose

Automatically synchronize Kubernetes Ingress hostnames to Pi-hole local DNS records, enabling seamless internal DNS resolution for services exposed via Ingress in a bare-metal Kubernetes cluster.

### 1.2 Goals

- Zero-touch DNS registration for annotated Ingresses
- Minimal configuration surface
- Reliable eventual consistency between Ingress state and Pi-hole DNS
- Clear operational visibility via logging

### 1.3 Non-Goals

- Multi-Pi-hole support
- Pi-hole v5 compatibility
- CRD-based configuration
- Guaranteed deletion cleanup (no finalizers)
- CNAME record support (A records only)

## 2. Functional Requirements

### 2.1 Ingress Watching

| Requirement | Description |
|-------------|-------------|
| FR-1 | Operator MUST watch all Ingress resources across all namespaces |
| FR-2 | Operator MUST filter to only process Ingresses with `pihole.io/register: "true"` annotation |
| FR-3 | Operator MUST reconcile on Ingress create, update, and delete events |
| FR-4 | Operator MUST handle Ingress updates that add or remove the registration annotation |

### 2.2 Hostname Extraction

| Requirement | Description |
|-------------|-------------|
| FR-5 | Operator MUST extract hostnames from `spec.rules[].host` by default |
| FR-6 | Operator MUST support `pihole.io/hosts` annotation to override extracted hostnames |
| FR-7 | Operator MUST ignore rules without a `host` field (wildcard rules) |
| FR-8 | Operator MUST handle Ingresses with zero valid hostnames gracefully (no-op) |

### 2.3 Target IP Resolution

| Requirement | Description |
|-------------|-------------|
| FR-9 | Operator MUST use `DEFAULT_TARGET_IP` from configuration as the default target |
| FR-10 | Operator MUST support `pihole.io/target-ip` annotation to override the target IP per-Ingress |
| FR-11 | Operator MUST validate IP addresses (IPv4 format) |

### 2.4 Pi-hole Synchronization

| Requirement | Description |
|-------------|-------------|
| FR-12 | Operator MUST create DNS A records in Pi-hole for each hostname |
| FR-13 | Operator MUST update existing DNS records if the target IP changes |
| FR-14 | Operator MUST delete DNS records when an Ingress is deleted |
| FR-15 | Operator MUST delete DNS records when the `pihole.io/register` annotation is removed |
| FR-16 | Operator MUST delete DNS records for hostnames removed from an Ingress |
| FR-17 | Operator MUST handle Pi-hole API errors with exponential backoff retry |

### 2.5 State Tracking

| Requirement | Description |
|-------------|-------------|
| FR-18 | Operator MUST track which DNS records it manages to avoid deleting user-created records |
| FR-19 | Operator SHOULD use Ingress annotations or status to track managed hostnames |

## 3. Non-Functional Requirements

| Requirement | Description |
|-------------|-------------|
| NFR-1 | Operator MUST start up and become ready within 30 seconds |
| NFR-2 | Operator MUST consume less than 64MB memory under normal operation |
| NFR-3 | Operator MUST handle at least 100 Ingress resources |
| NFR-4 | Operator MUST log all reconciliation actions at INFO level |
| NFR-5 | Operator MUST log errors with sufficient context for debugging |

## 4. Configuration Specification

### 4.1 Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PIHOLE_URL` | Yes | - | Base URL of Pi-hole instance (e.g., `http://192.168.1.2`) |
| `PIHOLE_API_TOKEN` | Yes | - | Pi-hole API authentication token |
| `DEFAULT_TARGET_IP` | Yes | - | Default IP address for DNS A records |
| `LOG_LEVEL` | No | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `WATCH_NAMESPACE` | No | `""` (all) | Namespace to watch, empty for all namespaces |

### 4.2 Configuration Validation

On startup, the operator MUST validate:
- `PIHOLE_URL` is a valid HTTP(S) URL
- `PIHOLE_API_TOKEN` is non-empty
- `DEFAULT_TARGET_IP` is a valid IPv4 address
- `LOG_LEVEL` is one of the allowed values

The operator MUST exit with a non-zero code if validation fails.

## 5. API Specification

### 5.1 Pi-hole v6 API Integration

#### Authentication

All requests include:
```
Authorization: Bearer <PIHOLE_API_TOKEN>
```

#### List DNS Records

```
GET {PIHOLE_URL}/admin/api/dns/local
```

Response:
```json
{
  "data": [
    {"domain": "app.local", "ip": "192.168.1.100"},
    {"domain": "api.local", "ip": "192.168.1.100"}
  ]
}
```

#### Create DNS Record

```
POST {PIHOLE_URL}/admin/api/dns/local
Content-Type: application/json

{"domain": "app.local", "ip": "192.168.1.100"}
```

Response: `201 Created`

#### Delete DNS Record

```
DELETE {PIHOLE_URL}/admin/api/dns/local/{domain}
```

Response: `204 No Content`

### 5.2 Error Handling

| HTTP Status | Operator Behavior |
|-------------|-------------------|
| 2xx | Success, continue |
| 400 | Log error, do not retry (invalid request) |
| 401/403 | Log error, requeue with backoff (auth issue) |
| 404 (DELETE) | Success (record already gone) |
| 404 (other) | Log error, requeue with backoff |
| 5xx | Log error, requeue with backoff |
| Timeout | Log error, requeue with backoff |

## 6. Controller Specification

### 6.1 Reconciliation Logic

```
FUNCTION Reconcile(ingress):
    IF ingress is being deleted:
        CALL cleanupDNSRecords(ingress)
        RETURN success
    
    IF NOT hasRegistrationAnnotation(ingress):
        CALL cleanupDNSRecords(ingress)  // In case annotation was removed
        RETURN success
    
    desiredHosts = extractHosts(ingress)
    targetIP = resolveTargetIP(ingress)
    
    IF targetIP is invalid:
        LOG warning
        RETURN success (do not requeue)
    
    currentRecords = getPiholeRecords()
    managedHosts = getManagedHosts(ingress)
    
    // Create or update records for desired hosts
    FOR host IN desiredHosts:
        IF host NOT IN currentRecords OR currentRecords[host] != targetIP:
            CALL createOrUpdateRecord(host, targetIP)
    
    // Delete records for hosts no longer desired
    FOR host IN managedHosts:
        IF host NOT IN desiredHosts:
            CALL deleteRecord(host)
    
    updateManagedHosts(ingress, desiredHosts)
    RETURN success
```

### 6.2 Host Extraction Logic

```
FUNCTION extractHosts(ingress):
    IF ingress has "pihole.io/hosts" annotation:
        RETURN parseCommaSeparated(annotation value)
    
    hosts = []
    FOR rule IN ingress.spec.rules:
        IF rule.host is not empty:
            APPEND rule.host TO hosts
    
    RETURN hosts
```

### 6.3 Managed Hosts Tracking

The operator tracks which hosts it has registered for each Ingress using an annotation:

```yaml
annotations:
  pihole.io/managed-hosts: "app.local,api.local"
```

This annotation is updated by the operator after successful reconciliation.

### 6.4 Requeue Strategy

| Scenario | Requeue Delay |
|----------|---------------|
| Successful reconciliation | No requeue |
| Pi-hole API temporary error | 30 seconds (exponential backoff up to 5 minutes) |
| Rate limited | 60 seconds |

## 7. Data Types

### 7.1 Go Types

```go
// Config holds operator configuration
type Config struct {
    PiholeURL       string
    PiholeAPIToken  string
    DefaultTargetIP string
    LogLevel        string
    WatchNamespace  string
}

// DNSRecord represents a Pi-hole local DNS record
type DNSRecord struct {
    Domain string `json:"domain"`
    IP     string `json:"ip"`
}

// PiholeClient interface for Pi-hole API operations
type PiholeClient interface {
    ListRecords(ctx context.Context) ([]DNSRecord, error)
    CreateRecord(ctx context.Context, record DNSRecord) error
    DeleteRecord(ctx context.Context, domain string) error
}
```

## 8. Deployment Specification

### 8.1 RBAC Requirements

```yaml
rules:
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]
```

### 8.2 Resource Limits

```yaml
resources:
  requests:
    memory: "32Mi"
    cpu: "10m"
  limits:
    memory: "64Mi"
    cpu: "100m"
```

### 8.3 Health Probes

- **Liveness**: `/healthz` - Returns 200 if process is running
- **Readiness**: `/readyz` - Returns 200 if connected to K8s API and Pi-hole is reachable

## 9. Logging Specification

### 9.1 Log Format

Structured JSON logging via slog:

```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "msg": "dns record created",
  "ingress": "default/my-app",
  "host": "app.local",
  "ip": "192.168.1.100"
}
```

### 9.2 Log Events

| Level | Event | Fields |
|-------|-------|--------|
| DEBUG | Reconcile started | `ingress` |
| INFO | DNS record created | `ingress`, `host`, `ip` |
| INFO | DNS record updated | `ingress`, `host`, `old_ip`, `new_ip` |
| INFO | DNS record deleted | `ingress`, `host` |
| WARN | Invalid annotation | `ingress`, `annotation`, `value`, `error` |
| WARN | Ingress skipped (no hosts) | `ingress` |
| ERROR | Pi-hole API error | `ingress`, `operation`, `error` |
| ERROR | Reconcile failed | `ingress`, `error` |

## 10. Testing Specification

### 10.1 Unit Tests

| Component | Test Cases |
|-----------|------------|
| Config | Valid config, missing required fields, invalid IP, invalid URL |
| PiholeClient | Create/delete/list records, error handling, auth headers |
| Host extraction | From spec, from annotation, empty cases, mixed |
| IP resolution | Default IP, annotation override, invalid override |

### 10.2 Integration Tests

Using controller-runtime's envtest:

| Test Case | Description |
|-----------|-------------|
| Create annotated Ingress | Verify DNS record created |
| Update Ingress hosts | Verify records added/removed |
| Remove annotation | Verify records cleaned up |
| Delete Ingress | Verify records cleaned up |
| Invalid annotation | Verify Ingress skipped, no error |

## 11. Future Considerations

Items explicitly out of scope but may be added later:

- **Prometheus metrics**: Reconcile counts, latency, error rates
- **Multi-Pi-hole**: Sync to multiple instances
- **CNAME support**: For external service references
- **Webhook validation**: Reject invalid annotations at admission time
- **Leader election**: For HA deployments
