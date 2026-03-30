# forge-plugin-consul

Tools plugin for [HashiCorp Consul](https://www.consul.io/), exposing catalog, health, KV store, and agent operations to the Forge agent.

## Capabilities

| Capability | Supported |
|---|---|
| Tools | yes |
| Async execution | no |

## Configuration

```hcl
plugin "consul" {
  address     = "http://localhost:8500"  # default
  token       = ""                       # ACL token
  datacenter  = ""                       # default datacenter
  namespace   = ""
  partition   = ""
  timeout     = 30                       # seconds
}
```

### TLS

```hcl
plugin "consul" {
  address = "https://consul.example.com:8501"

  tls {
    ca_file             = "/etc/consul/ca.pem"
    cert_file           = "/etc/consul/client.pem"
    key_file            = "/etc/consul/client-key.pem"
    insecure_skip_verify = false
  }
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `address` | string | `http://localhost:8500` | Consul server address |
| `token` | string | — | ACL token |
| `datacenter` | string | — | Default datacenter |
| `namespace` | string | — | Consul namespace (Enterprise) |
| `partition` | string | — | Consul partition (Enterprise) |
| `timeout` | int | `30` | HTTP timeout in seconds |
| `tls.ca_file` | string | — | CA certificate path |
| `tls.cert_file` | string | — | Client certificate path |
| `tls.key_file` | string | — | Client private key path |
| `tls.insecure_skip_verify` | bool | `false` | Skip TLS verification |

## Tools

### Catalog

| Tool | Description | Destructive |
|---|---|---|
| `catalog_datacenters` | List all known datacenters | no |
| `catalog_nodes` | List nodes, optional filter | no |
| `catalog_services` | List services with tags | no |
| `catalog_service` | Nodes providing a specific service | no |
| `catalog_node` | Services registered on a specific node | no |

### Health

| Tool | Description |
|---|---|
| `health_service` | Health status of service instances |
| `health_node` | Health checks for a node |
| `health_checks` | List checks by state (passing/warning/critical) |

### Key-Value

| Tool | Description | Destructive |
|---|---|---|
| `kv_get` | Read a key | no |
| `kv_list` | List keys under a prefix | no |
| `kv_put` | Write a key | no |
| `kv_delete` | Delete a key — requires confirmation | yes |

### Agent

| Tool | Description |
|---|---|
| `agent_members` | List Serf gossip members |
| `agent_services` | Services registered with the local agent |
