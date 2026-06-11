package consul

import (
	"context"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
)

type ConsulCapabilitySet struct {
	AgentRead    bool
	AgentWrite   bool
	CatalogRead  bool
	CatalogWrite bool
	HealthRead   bool
	KVRead       bool
	KVWrite      bool
	ACLRead      bool
	EventRead    bool
	SessionRead  bool
	OperatorRead bool
}

func (c ConsulCapabilitySet) Summary() string {
	var parts []string
	add := func(label string, ok bool) {
		if ok {
			parts = append(parts, label)
		}
	}

	add("agent:read", c.AgentRead)
	add("agent:write", c.AgentWrite)
	add("catalog:read", c.CatalogRead)
	add("catalog:write", c.CatalogWrite)
	add("health:read", c.HealthRead)
	add("kv:read", c.KVRead)
	add("kv:write", c.KVWrite)
	add("acl:read", c.ACLRead)
	add("event:read", c.EventRead)
	add("session:read", c.SessionRead)
	add("operator:read", c.OperatorRead)

	return strings.Join(parts, ", ")
}

// ProbeCapabilities tests each Consul API surface and returns a capabilitySet
// reflecting what the configured token can access. Read capabilities are probed
// via lightweight GET calls. KV write is probed via a sentinel key that is
// immediately deleted. Other write capabilities are inferred from the token's
// policy rules to avoid side effects.
func ProbeCapabilities(ctx context.Context, client *api.Client, log hclog.Logger) ConsulCapabilitySet {
	caps := ConsulCapabilitySet{}

	if _, err := client.Agent().Self(); err == nil {
		caps.AgentRead = true
	}
	log.Debug("capability probe", "agent:read", caps.AgentRead)

	if _, err := client.Catalog().Datacenters(); err == nil {
		caps.CatalogRead = true
	}
	log.Debug("capability probe", "catalog:read", caps.CatalogRead)

	if _, _, err := client.Health().State("any", nil); err == nil {
		caps.HealthRead = true
	}
	log.Debug("capability probe", "health:read", caps.HealthRead)

	if _, _, err := client.KV().Keys("", "", nil); err == nil {
		caps.KVRead = true
	}
	log.Debug("capability probe", "kv:read", caps.KVRead)

	// KV write: sentinel put + immediate delete to avoid leaving state.
	const probeKey = "__forge/capability_probe"
	if _, err := client.KV().Put(&api.KVPair{Key: probeKey, Value: []byte("probe")}, nil); err == nil {
		caps.KVWrite = true
		client.KV().Delete(probeKey, nil) //nolint:errcheck
	}
	log.Debug("capability probe", "kv:write", caps.KVWrite)

	// ACL read also drives write-capability inference from policy rules.
	if token, _, err := client.ACL().TokenReadSelf(nil); err == nil {
		caps.ACLRead = true
		caps = InferWriteFromPolicies(client, token, caps, log)
	}
	log.Debug("capability probe", "acl:read", caps.ACLRead)

	if _, _, err := client.Event().List("", nil); err == nil {
		caps.EventRead = true
	}
	log.Debug("capability probe", "event:read", caps.EventRead)

	if _, _, err := client.Session().List(nil); err == nil {
		caps.SessionRead = true
	}
	log.Debug("capability probe", "session:read", caps.SessionRead)

	if _, err := client.Status().Leader(); err == nil {
		caps.OperatorRead = true
	}
	log.Debug("capability probe", "operator:read", caps.OperatorRead)

	return caps
}

// InferWriteFromPolicies reads each policy attached to the token and uses
// simple rule-string analysis to determine write capabilities that can't be
// probed safely without side effects.
func InferWriteFromPolicies(client *api.Client, token *api.ACLToken, caps ConsulCapabilitySet, log hclog.Logger) ConsulCapabilitySet {
	for _, link := range token.Policies {
		policy, _, err := client.ACL().PolicyRead(link.ID, nil)
		if err != nil {
			log.Warn("capability probe: failed to read policy", "policy_id", link.ID, "err", err)
			continue
		}
		rules := policy.Rules

		if !caps.CatalogWrite && (HasWritePermission(rules, "service") || HasWritePermission(rules, "node")) {
			caps.CatalogWrite = true
			log.Debug("capability probe", "catalog:write", true, "policy", policy.Name)
		}
		if !caps.AgentWrite && HasWritePermission(rules, "agent") {
			caps.AgentWrite = true
			log.Debug("capability probe", "agent:write", true, "policy", policy.Name)
		}
	}
	return caps
}

// HasWritePermission reports whether the Consul ACL HCL rules string grants
// write access to the named resource. It handles both inline assignments
// (e.g. agent = "write") and block-style rules
// (e.g. agent_prefix "" { policy = "write" }).
func HasWritePermission(rules, resource string) bool {
	rest := rules
	for {
		i := strings.Index(rest, resource)
		if i < 0 {
			break
		}
		segment := rest[i:]

		// Bound the search to the current statement or block.
		end := len(segment)
		if close := strings.IndexByte(segment, '}'); close >= 0 && close < end {
			end = close + 1
		} else if nl := strings.IndexByte(segment, '\n'); nl >= 0 && nl < end {
			end = nl + 1
		}

		if strings.Contains(segment[:end], `"write"`) {
			return true
		}
		rest = rest[i+len(resource):]
	}
	return false
}

func BuildEnabledTools(caps ConsulCapabilitySet) []string {
	tools := make([]string, 0)

	for _, category := range ToolDefinitionsCategory {
		for name, tool := range category.Tools {
			var check func(ConsulCapabilitySet) bool

			if tool.Annotations.ReadOnly {
				check = category.Read
			} else {
				check = category.Write
			}
			if check != nil && check(caps) {
				tools = append(tools, name)
			}
		}
	}

	return tools
}
