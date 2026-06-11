package consul

import "github.com/mwantia/forge-sdk/pkg/plugins"

// ConsulToolDefinitionCategory groups related tool definitions under a shared
// capability gate. Read and Write are separate checks so that a read-only token
// still exposes the read tools for a category while silently omitting its write
// tools. A nil check means the category has no tools of that access level.
type ConsulToolDefinitionCategory struct {
	Read  func(ConsulCapabilitySet) bool
	Write func(ConsulCapabilitySet) bool
	Tools map[string]plugins.ToolDefinition
}

// toolDefinitions is the single source of truth for every tool the consul
// plugin exposes: its definition, its tags, its parameters, and which
// capability gate controls its availability.
var ToolDefinitionsCategory = map[string]ConsulToolDefinitionCategory{
	// -------------------------------------------------------------------------
	// ACL
	// -------------------------------------------------------------------------
	"acl": {
		Read: func(c ConsulCapabilitySet) bool { return c.ACLRead },
		Tools: map[string]plugins.ToolDefinition{
			"acl_token_self": {
				Name:        "acl_token_self",
				Description: "Return the ACL token used by the current request, including its policies and roles",
				Tags:        []string{"consul", "acl"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Use when the user asks "what can this Consul integration do?" or "what's my token allowed to read". 
Returns the policies attached to the token currently in use — the source of truth for why a given tool may or may not work. Secret IDs are never returned.
`,
				},
				Parameters: plugins.ToolParameters{
					Type:       "object",
					Properties: map[string]plugins.ToolProperty{},
				},
			},
			"acl_tokens_list": {
				Name:        "acl_tokens_list",
				Description: "List all ACL tokens (accessor IDs, descriptions, policies). Does not expose secret IDs",
				Tags:        []string{"consul", "acl"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Inventory of every ACL token. Use for audits ("who has write to KV?") or to find a token's accessor_id by description before acl_token_read. Secret IDs are intentionally omitted by Consul — do not promise them.
`,
				},
				Parameters: plugins.ToolParameters{
					Type:       "object",
					Properties: map[string]plugins.ToolProperty{},
				},
			},
			"acl_token_read": {
				Name:        "acl_token_read",
				Description: "Read a specific ACL token by its accessor ID, including attached policies and roles",
				Tags:        []string{"consul", "acl"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Resolve accessor_id via acl_tokens_list first; users will refer to tokens by description, not UUID. Returns policy names but not the underlying rules — chain to acl_policy_read for that.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"accessor_id": {Type: "string", Description: "Accessor ID of the token to read", Format: "uuid"},
					},
					Required: []string{"accessor_id"},
				},
			},
			"acl_policies_list": {
				Name:        "acl_policies_list",
				Description: "List all ACL policies with their names and descriptions",
				Tags:        []string{"consul", "acl"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Returns the catalog of policy names + descriptions only. The HCL/JSON rules body is not included here — call acl_policy_read once you've identified the policy_id of interest.
`,
				},
				Parameters: plugins.ToolParameters{
					Type:       "object",
					Properties: map[string]plugins.ToolProperty{},
				},
			},
			"acl_policy_read": {
				Name:        "acl_policy_read",
				Description: "Read the full rules of a specific ACL policy by its ID",
				Tags:        []string{"consul", "acl"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Returns the raw HCL/JSON rules body. Use when explaining "why can this token read X but not Y". Resolve policy_id from acl_policies_list — the user will know the policy by name, not ID.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"policy_id": {Type: "string", Description: "ID of the policy to read", Format: "uuid"},
					},
					Required: []string{"policy_id"},
				},
			},
		},
	},
	// -------------------------------------------------------------------------
	// Agent
	// -------------------------------------------------------------------------
	"agent": {
		Read:  func(c ConsulCapabilitySet) bool { return c.AgentRead },
		Write: func(c ConsulCapabilitySet) bool { return c.AgentWrite },
		Tools: map[string]plugins.ToolDefinition{
			"agent_self": {
				Name:        "agent_self",
				Description: "Return configuration and status of the local Consul agent: node name, datacenter, version, server/client mode",
				Tags:        []string{"consul", "agent"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Local agent introspection — the node forge talks to, not the cluster as a whole. Use to confirm version, datacenter, ACL/TLS posture, or server-vs-client role before running operator-level reasoning.
`,
				},
				Parameters: plugins.ToolParameters{
					Type:       "object",
					Properties: map[string]plugins.ToolProperty{},
				},
			},
			"agent_checks": {
				Name:        "agent_checks",
				Description: "List all health checks registered with the local Consul agent",
				Tags:        []string{"consul", "agent", "health"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Local-agent scope only. For "what's failing across the cluster" use health_checks instead — agent_checks only sees what's registered on this single node.
`,
				},
				Parameters: plugins.ToolParameters{
					Type:       "object",
					Properties: map[string]plugins.ToolProperty{},
				},
			},
			"agent_members": {
				Name:        "agent_members",
				Description: "List all members visible to the local Consul agent via Serf gossip",
				Tags:        []string{"consul", "agent"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Serf-level membership view — includes nodes that are alive, failed, or left. Pass wan=true to see federated DCs. Status is an integer (0=none, 1=alive, 2=leaving, 3=left, 4=failed); translate before showing the user.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"wan": {Type: "boolean", Description: "List WAN members instead of LAN members"},
					},
				},
			},
			"agent_services": {
				Name:        "agent_services",
				Description: "List all services registered with the local Consul agent",
				Tags:        []string{"consul", "agent", "service"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Local agent only. For a cluster-wide service list use catalog_services. Useful when diagnosing a specific node's registrations or comparing agent vs catalog drift.
`,
				},
				Parameters: plugins.ToolParameters{
					Type:       "object",
					Properties: map[string]plugins.ToolProperty{},
				},
			},
			"agent_maintenance": {
				Name:        "agent_maintenance",
				Description: "Enable or disable maintenance mode on the local Consul agent node. While enabled the node is marked critical and excluded from service discovery",
				Tags:        []string{"consul", "agent"},
				Annotations: plugins.ToolAnnotations{
					Idempotent:           true,
					RequiresConfirmation: true,
					CostHint:             plugins.ToolCostCheap,
					System: `
Drains the node from service discovery by marking every check critical. Always pass a meaningful reason — it shows up in dashboards and check output. 
Confirm with the user before enabling on a production node; pair with a follow-up call to disable when done.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"enable": {Type: "boolean", Description: "True to enable maintenance mode, false to disable"},
						"reason": {Type: "string", Description: "Human-readable reason stored in the check output"},
					},
					Required: []string{"enable"},
				},
			},
			"agent_reload": {
				Name:        "agent_reload",
				Description: "Instruct the local Consul agent to reload its configuration from disk",
				Tags:        []string{"consul", "agent"},
				Annotations: plugins.ToolAnnotations{
					RequiresConfirmation: true,
					CostHint:             plugins.ToolCostCheap,
					System: `
Re-reads agent config files. Not all settings are reloadable (bind addresses, encryption keys are not) — warn the user that some changes still require a restart. Confirm before calling against a production agent.
`,
				},
				Parameters: plugins.ToolParameters{
					Type:       "object",
					Properties: map[string]plugins.ToolProperty{},
				},
			},
			"agent_service_register": {
				Name:        "agent_service_register",
				Description: "Register a service with the local Consul agent",
				Tags:        []string{"consul", "agent", "service"},
				Annotations: plugins.ToolAnnotations{
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Idempotent — calling with the same id overwrites the existing registration. Default the id to the name unless the user explicitly wants multiple instances on one agent. 
This tool does not configure health checks; the registered service will appear without checks.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"name":    {Type: "string", Description: "Service name"},
						"id":      {Type: "string", Description: "Unique service ID (defaults to name if omitted)"},
						"address": {Type: "string", Description: "Service address (defaults to agent address)"},
						"port":    {Type: "integer", Description: "Service port"},
						"tags":    {Type: "string", Description: "Comma-separated list of tags"},
					},
					Required: []string{"name"},
				},
			},
			"agent_service_deregister": {
				Name:        "agent_service_deregister",
				Description: "Deregister a service from the local Consul agent by service ID",
				Tags:        []string{"consul", "agent", "service"},
				Annotations: plugins.ToolAnnotations{
					Destructive:          true,
					Idempotent:           true,
					RequiresConfirmation: true,
					CostHint:             plugins.ToolCostCheap,
					System: `
Removes the service and all its checks from the agent and the catalog. Resolve service_id via agent_services first — users say "deregister api" but the API requires the exact id. Confirm before calling.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"service_id": {Type: "string", Description: "ID of the service to deregister"},
					},
					Required: []string{"service_id"},
				},
			},
			"agent_check_deregister": {
				Name:        "agent_check_deregister",
				Description: "Deregister a health check from the local Consul agent by check ID",
				Tags:        []string{"consul", "agent", "health"},
				Annotations: plugins.ToolAnnotations{
					Destructive:          true,
					Idempotent:           true,
					RequiresConfirmation: true,
					CostHint:             plugins.ToolCostCheap,
					System: `
Removes one specific check while leaving the service registered. Use when a check is misconfigured and the user wants to silence it without removing the whole service. Resolve check_id from agent_checks.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"check_id": {Type: "string", Description: "ID of the check to deregister"},
					},
					Required: []string{"check_id"},
				},
			},
			"agent_check_update": {
				Name:        "agent_check_update",
				Description: "Update the status and output of a TTL health check registered with the local agent",
				Tags:        []string{"consul", "agent", "health"},
				Annotations: plugins.ToolAnnotations{
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Only works on TTL-style checks (where the application heartbeats into Consul). HTTP/script checks reject this call. 
Use status="passing" as a heartbeat or to recover a check that flipped critical due to a transient issue.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"check_id": {Type: "string", Description: "ID of the TTL check to update"},
						"status":   {Type: "string", Description: "New check status", Enum: []string{"passing", "warning", "critical"}},
						"output":   {Type: "string", Description: "Human-readable output message stored in the check"},
					},
					Required: []string{"check_id", "status"},
				},
			},
		},
	},
	// -------------------------------------------------------------------------
	// Catalog
	// -------------------------------------------------------------------------
	"catalog": {
		Read:  func(c ConsulCapabilitySet) bool { return c.CatalogRead },
		Write: func(c ConsulCapabilitySet) bool { return c.CatalogWrite },
		Tools: map[string]plugins.ToolDefinition{
			"catalog_datacenters": {
				Name:        "catalog_datacenters",
				Description: "List all known datacenters in the Consul cluster",
				Tags:        []string{"consul", "catalog"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Cheap discovery call. Run once at the start of cross-DC questions to confirm which datacenters exist before using the datacenter argument on any other catalog/health tool.
`,
				},
				Parameters: plugins.ToolParameters{
					Type:       "object",
					Properties: map[string]plugins.ToolProperty{},
				},
			},
			"catalog_nodes": {
				Name:        "catalog_nodes",
				Description: "List all nodes registered in the Consul catalog",
				Tags:        []string{"consul", "catalog", "node"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Cluster-wide node inventory. The filter argument takes a Consul filter expression — prefer it over post-filtering output when the user asks "how many production nodes" or similar. Example: 'Meta.env == "prod"'.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"datacenter": {Type: "string", Description: "Datacenter to query (defaults to agent's datacenter)"},
						"filter":     {Type: "string", Description: "Consul filter expression (e.g. 'Meta.env == \"production\"')"},
					},
				},
			},
			"catalog_node": {
				Name:        "catalog_node",
				Description: "Get all services registered on a specific node",
				Tags:        []string{"consul", "catalog", "node"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Use to answer "what runs on host X". Returns service registrations without health — pair with health_node when the user wants both.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"node":       {Type: "string", Description: "Node name or ID"},
						"datacenter": {Type: "string", Description: "Datacenter to query"},
					},
					Required: []string{"node"},
				},
			},
			"catalog_services": {
				Name:        "catalog_services",
				Description: "List all services registered in the Consul catalog with their tags",
				Tags:        []string{"consul", "catalog", "service"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Service-name index for the datacenter. Returns names + tags only — follow up with catalog_service or health_service to get instance details. 
For "what services exist" use this; for "is X healthy" jump straight to health_service.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"datacenter": {Type: "string", Description: "Datacenter to query (defaults to agent's datacenter)"},
					},
				},
			},
			"catalog_service": {
				Name:        "catalog_service",
				Description: "Get all nodes providing a specific service, including health status and metadata",
				Tags:        []string{"consul", "catalog", "service"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Catalog-level view of service instances (registration data, no live health summary). For "is service X up" prefer health_service which joins in check status. 
Use the tag argument to scope to canary/prod variants.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"service":    {Type: "string", Description: "Service name to look up"},
						"tag":        {Type: "string", Description: "Filter results to nodes with this tag"},
						"datacenter": {Type: "string", Description: "Datacenter to query"},
					},
					Required: []string{"service"},
				},
			},
			"catalog_deregister": {
				Name:        "catalog_deregister",
				Description: "Deregister a node, service, or check from the Consul catalog. Omitting service_id and check_id removes the entire node",
				Tags:        []string{"consul", "catalog"},
				Annotations: plugins.ToolAnnotations{
					Destructive:          true,
					Idempotent:           true,
					RequiresConfirmation: true,
					CostHint:             plugins.ToolCostCheap,
					System: `
Removes catalog entries the agent itself did not register. Specify service_id or check_id to scope the removal — omitting both removes the entire node and all of its services. 
Confirm with the user before calling against a production datacenter.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"node":       {Type: "string", Description: "Node name to deregister from"},
						"service_id": {Type: "string", Description: "Service ID to deregister (omit to deregister the node)"},
						"check_id":   {Type: "string", Description: "Check ID to deregister (omit to deregister the node or service)"},
						"datacenter": {Type: "string", Description: "Datacenter to target"},
					},
					Required: []string{"node"},
				},
			},
		},
	},
	// -------------------------------------------------------------------------
	// Health
	// -------------------------------------------------------------------------
	"health": {
		Read: func(c ConsulCapabilitySet) bool { return c.HealthRead },
		Tools: map[string]plugins.ToolDefinition{
			"health_node": {
				Name:        "health_node",
				Description: "Get all health checks registered for a specific node",
				Tags:        []string{"consul", "health", "node"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Use when the user names a specific node ("is db-01 healthy?"). Returns node-level + service-level checks for that one host. For service-wide health across nodes use health_service or health_checks_service.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"node":       {Type: "string", Description: "Node name"},
						"datacenter": {Type: "string", Description: "Datacenter to query"},
					},
					Required: []string{"node"},
				},
			},
			"health_checks": {
				Name:        "health_checks",
				Description: "List all health checks across the cluster filtered by state",
				Tags:        []string{"consul", "health"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Best entry point for "what's broken right now" — pass state="critical" or "warning" to surface only the failing checks across every service and node. state="any" returns everything (large; avoid unless asked).
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"state":      {Type: "string", Description: "Health check state to filter by", Enum: []string{"passing", "warning", "critical", "any"}},
						"datacenter": {Type: "string", Description: "Datacenter to query"},
					},
					Required: []string{"state"},
				},
			},
			"health_checks_service": {
				Name:        "health_checks_service",
				Description: "List all health checks registered for a specific service across all nodes",
				Tags:        []string{"consul", "health", "service"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Per-service check list with status. Use when the user asks why a specific service is degraded — shows which checks across which nodes are failing.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"service":    {Type: "string", Description: "Service name"},
						"datacenter": {Type: "string", Description: "Datacenter to query"},
					},
					Required: []string{"service"},
				},
			},
			"health_service": {
				Name:        "health_service",
				Description: "Get full health status of all instances of a service, including node info, service metadata, and all associated checks",
				Tags:        []string{"consul", "health", "service"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Richest service-level view — joins instance metadata with all checks. Default to passing_only=true when the user asks "what can I connect to"; default false when diagnosing failures - tag filters to a specific deployment slice.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"service":      {Type: "string", Description: "Service name"},
						"tag":          {Type: "string", Description: "Filter by tag"},
						"passing_only": {Type: "boolean", Description: "Return only instances passing all health checks"},
						"datacenter":   {Type: "string", Description: "Datacenter to query"},
					},
					Required: []string{"service"},
				},
			},
			"health_connect": {
				Name:        "health_connect",
				Description: "Get health status of all Connect-capable instances of a service (sidecar proxies)",
				Tags:        []string{"consul", "health", "service", "connect"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Same shape as health_service but filtered to Connect/sidecar mesh instances. Use when the user is reasoning about service-mesh routing, intentions, or mTLS — not for plain HTTP service discovery.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"service":      {Type: "string", Description: "Service name"},
						"tag":          {Type: "string", Description: "Filter by tag"},
						"passing_only": {Type: "boolean", Description: "Return only instances passing all health checks"},
						"datacenter":   {Type: "string", Description: "Datacenter to query"},
					},
					Required: []string{"service"},
				},
			},
		},
	},
	// -------------------------------------------------------------------------
	// Key-Value
	// -------------------------------------------------------------------------
	"kv": {
		Read:  func(c ConsulCapabilitySet) bool { return c.KVRead },
		Write: func(c ConsulCapabilitySet) bool { return c.KVWrite },
		Tools: map[string]plugins.ToolDefinition{
			"kv_get": {
				Name:        "kv_get",
				Description: "Read a value from the Consul key-value store",
				Tags:        []string{"consul", "kv"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Returns the raw value as bytes (decoded to string). Missing keys are not an error — check the "found" flag in the result before treating an empty value as meaningful.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"key":        {Type: "string", Description: "Key path to read"},
						"datacenter": {Type: "string", Description: "Datacenter to query"},
					},
					Required: []string{"key"},
				},
			},
			"kv_list": {
				Name:        "kv_list",
				Description: "List all keys in the Consul KV store under a given prefix",
				Tags:        []string{"consul", "kv"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Returns key names only — no values. Use to discover layout before batching kv_get calls. Empty prefix returns every key in the store; prefer a narrow prefix on production stores.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"prefix":     {Type: "string", Description: "Key prefix to list (empty string lists all keys)"},
						"datacenter": {Type: "string", Description: "Datacenter to query"},
					},
					Required: []string{"prefix"},
				},
			},
			"kv_put": {
				Name:        "kv_put",
				Description: "Write a value to the Consul key-value store",
				Tags:        []string{"consul", "kv"},
				Annotations: plugins.ToolAnnotations{
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Overwrites whatever is at "key". Run kv_get first if you need the prior value. Values are stored as raw bytes — pass plain strings, not JSON unless that's what the consumer expects.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"key":        {Type: "string", Description: "Key path to write"},
						"value":      {Type: "string", Description: "Value to store"},
						"datacenter": {Type: "string", Description: "Datacenter to write to"},
					},
					Required: []string{"key", "value"},
				},
			},
			"kv_delete": {
				Name:        "kv_delete",
				Description: "Delete a key or key prefix from the Consul key-value store",
				Tags:        []string{"consul", "kv"},
				Annotations: plugins.ToolAnnotations{
					Destructive:          true,
					Idempotent:           true,
					RequiresConfirmation: true,
					CostHint:             plugins.ToolCostCheap,
					System: `
recurse=true is a tree wipe — every key under the prefix disappears. Run kv_list first to show the user what would be deleted; require explicit confirmation before recursive delete in production.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"key":        {Type: "string", Description: "Key path to delete"},
						"recurse":    {Type: "boolean", Description: "Delete all keys sharing this prefix"},
						"datacenter": {Type: "string", Description: "Datacenter to target"},
					},
					Required: []string{"key"},
				},
			},
		},
	},
	// -------------------------------------------------------------------------
	// Operator / Status
	// -------------------------------------------------------------------------
	"operator": {
		Read: func(c ConsulCapabilitySet) bool { return c.OperatorRead },
		Tools: map[string]plugins.ToolDefinition{
			"status_leader": {
				Name:        "status_leader",
				Description: "Return the Raft leader address for the current datacenter",
				Tags:        []string{"consul", "operator", "raft"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Quick liveness probe at the consensus layer. Empty result means no leader is elected — that is a quorum/cluster-down signal worth flagging loudly to the user.
`,
				},
				Parameters: plugins.ToolParameters{
					Type:       "object",
					Properties: map[string]plugins.ToolProperty{},
				},
			},
			"status_peers": {
				Name:        "status_peers",
				Description: "Return the Raft peer set (voting members) for the current datacenter",
				Tags:        []string{"consul", "operator", "raft"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Voting Raft members only — non-voting servers and clients are not listed. For the full topology with leader/voter flags use operator_raft_config.
`,
				},
				Parameters: plugins.ToolParameters{
					Type:       "object",
					Properties: map[string]plugins.ToolProperty{},
				},
			},
			"operator_raft_config": {
				Name:        "operator_raft_config",
				Description: "Return the current Raft configuration: all server peers, their roles, suffrage, and whether each is the leader",
				Tags:        []string{"consul", "operator", "raft"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
The authoritative Raft view. Use when diagnosing quorum problems — shows which servers are voters vs non-voters and which holds leadership. For client-facing health use status_leader; this tool is for operators.
`,
				},
				Parameters: plugins.ToolParameters{
					Type:       "object",
					Properties: map[string]plugins.ToolProperty{},
				},
			},
		},
	},
	// -------------------------------------------------------------------------
	// Events
	// -------------------------------------------------------------------------
	"event": {
		Read: func(c ConsulCapabilitySet) bool { return c.EventRead },
		Tools: map[string]plugins.ToolDefinition{
			"event_list": {
				Name:        "event_list",
				Description: "List recent user events fired in the cluster, optionally filtered by event name",
				Tags:        []string{"consul", "event"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Consul user events are best-effort gossip-delivered messages — not a durable audit log. Use only when the user is debugging a custom event broadcast they themselves set up. 
Don't suggest events for general cluster history; logs/telemetry are better suited.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"name": {Type: "string", Description: "Filter events by name (empty returns all)"},
					},
				},
			},
		},
	},
	// -------------------------------------------------------------------------
	// Sessions
	// -------------------------------------------------------------------------
	"session": {
		Read: func(c ConsulCapabilitySet) bool { return c.SessionRead },
		Tools: map[string]plugins.ToolDefinition{
			"session_list": {
				Name:        "session_list",
				Description: "List all active sessions in the cluster",
				Tags:        []string{"consul", "session"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Sessions back KV locks and leader election. Use when diagnosing stuck locks ("which session holds my-lock?") — pair with kv_get on the locked key to find session, then session_info to identify the holder.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"datacenter": {Type: "string", Description: "Datacenter to query"},
					},
				},
			},
			"session_info": {
				Name:        "session_info",
				Description: "Return details of a specific session by its ID",
				Tags:        []string{"consul", "session"},
				Annotations: plugins.ToolAnnotations{
					ReadOnly:   true,
					Idempotent: true,
					CostHint:   plugins.ToolCostCheap,
					System: `
Returns the node owning the session, its TTL/behavior, and any checks gating its liveness. behavior="release" frees locks on session end; "delete" wipes the locked keys — note this when explaining lock state.
`,
				},
				Parameters: plugins.ToolParameters{
					Type: "object",
					Properties: map[string]plugins.ToolProperty{
						"session_id": {Type: "string", Description: "Session ID to look up", Format: "uuid"},
						"datacenter": {Type: "string", Description: "Datacenter to query"},
					},
					Required: []string{"session_id"},
				},
			},
		},
	},
}

// FlatToolsDefinitions is a pre-built index of all tool definitions for O(1) lookups
// in GetTool, Validate, and Execute. It is populated once at package init.
var FlatToolsDefinitions map[string]plugins.ToolDefinition

func init() {
	FlatToolsDefinitions = make(map[string]plugins.ToolDefinition)
	for _, category := range ToolDefinitionsCategory {
		for name, tool := range category.Tools {
			FlatToolsDefinitions[name] = tool
		}
	}
}
