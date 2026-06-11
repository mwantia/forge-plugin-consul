package consul

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/mwantia/forge-sdk/pkg/data"
	"github.com/mwantia/forge-sdk/pkg/plugins"
)

type ConsulToolsPlugin struct {
	plugins.UnimplementedToolsPlugin
	driver *ConsulDriver
}

func (p *ConsulToolsPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *ConsulToolsPlugin) System(_ context.Context) (string, error) {
	prompt := data.NewPrompt()

	prompt.Section(`
HashiCorp Consul: service catalog, health, KV store, agent, ACLs, sessions, events. Use when the user asks about cluster state, service discovery, node health, or wants to read/write KV configuration.

Available tools are gated by the configured token's policies — the set you see is already filtered. KV writes and any *_deregister are destructive; confirm before calling. Health/catalog reads are cheap and idempotent.
	`)

	prompt.Header(3, "ACL-Aware Tool Surface")
	prompt.Section(`
The set of Consul tools available in this session is *dynamically filtered by the configured ACL token's policies*. Not all endpoints listed below may be present — their absence means the token lacks the corresponding permission (e.g. 'intentions:read', 'kv:write', 'operator:read').

**When a tool you expect is missing:**
- Do **not** assume the Consul cluster lacks that capability. It is almost certainly an ACL restriction.
- Inform the user that the operation requires a policy grant the current token does not possess, and suggest which policy capability is needed (e.g. '"This requires the intentions:read policy on your Consul ACL token"').
- Where possible, offer the equivalent CLI command or API path the user can run directly with a suitably privileged token.

**When a read tool exists but its write counterpart does not** (e.g. 'kv_get' present, 'kv_put' absent), this is intentional — the token has read but not write permission on that resource. Do not suggest workarounds that circumvent ACL boundaries.
	`)

	if p.driver.config.Datacenter != "" {
		prompt.Sectionf("Default datacenter: `%s`. Omit the `datacenter` argument unless the user asks about a different one.", p.driver.config.Datacenter)
	}

	return prompt.String(), nil
}

func (p *ConsulToolsPlugin) ListTools(_ context.Context, filter plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	tools := make([]plugins.ToolDefinition, 0, len(p.driver.enabled))

	for _, name := range p.driver.enabled {
		def, ok := FlatToolsDefinitions[name]
		if !ok {
			continue
		}

		if matchesFilter(def, filter) {
			tools = append(tools, def)
		}
	}

	return &plugins.ListToolsResponse{
		Tools: tools,
	}, nil
}

func (p *ConsulToolsPlugin) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	if !slices.Contains(p.driver.enabled, name) {
		return nil, fmt.Errorf("tool definition %q not found or access not permitted by token", name)
	}

	definition, ok := FlatToolsDefinitions[name]
	if !ok {
		return nil, fmt.Errorf("tool definition %q not found", name)
	}

	return &definition, nil
}

func (p *ConsulToolsPlugin) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	if !slices.Contains(p.driver.enabled, req.Tool) {
		return nil, fmt.Errorf("tool definition %q not found or access not permitted by token", req.Tool)
	}

	definition, ok := FlatToolsDefinitions[req.Tool]
	if !ok {
		return &plugins.ValidateResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("unknown tool definition %q", req.Tool)},
		}, nil
	}

	var errs []string
	for _, r := range definition.Parameters.Required {
		v := req.Args.Get(r)
		if v.IsNull() || v.StringOr("") == "" {
			errs = append(errs, fmt.Sprintf("tool parameter %q is required", r))
			continue
		}

		if paramDef, ok := definition.Parameters.Properties[r]; ok && len(paramDef.Enum) > 0 {
			val := v.StringOr("")
			if !containsStr(paramDef.Enum, val) {
				errs = append(errs, fmt.Sprintf("tool parameter %q must be one of: %s", r, strings.Join(paramDef.Enum, ", ")))
			}
		}
	}

	return &plugins.ValidateResponse{
		Valid:  len(errs) == 0,
		Errors: errs,
	}, nil
}

func (p *ConsulToolsPlugin) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	switch req.Tool {
	// Catalog
	case "catalog_datacenters":
		return p.execCatalogDatacenters(ctx, req.Args)
	case "catalog_nodes":
		return p.execCatalogNodes(ctx, req.Args)
	case "catalog_services":
		return p.execCatalogServices(ctx, req.Args)
	case "catalog_service":
		return p.execCatalogService(ctx, req.Args)
	case "catalog_node":
		return p.execCatalogNode(ctx, req.Args)
	case "catalog_deregister":
		return p.execCatalogDeregister(ctx, req.Args)

	// Health
	case "health_service":
		return p.execHealthService(ctx, req.Args)
	case "health_node":
		return p.execHealthNode(ctx, req.Args)
	case "health_checks":
		return p.execHealthChecks(ctx, req.Args)
	case "health_checks_service":
		return p.execHealthChecksService(ctx, req.Args)
	case "health_connect":
		return p.execHealthConnect(ctx, req.Args)

	// KV
	case "kv_get":
		return p.execKVGet(ctx, req.Args)
	case "kv_list":
		return p.execKVList(ctx, req.Args)
	case "kv_put":
		return p.execKVPut(ctx, req.Args)
	case "kv_delete":
		return p.execKVDelete(ctx, req.Args)

	// Agent
	case "agent_self":
		return p.execAgentSelf(ctx, req.Args)
	case "agent_checks":
		return p.execAgentChecks(ctx, req.Args)
	case "agent_members":
		return p.execAgentMembers(ctx, req.Args)
	case "agent_services":
		return p.execAgentServices(ctx, req.Args)
	case "agent_maintenance":
		return p.execAgentMaintenance(ctx, req.Args)
	case "agent_reload":
		return p.execAgentReload(ctx, req.Args)
	case "agent_service_register":
		return p.execAgentServiceRegister(ctx, req.Args)
	case "agent_service_deregister":
		return p.execAgentServiceDeregister(ctx, req.Args)
	case "agent_check_deregister":
		return p.execAgentCheckDeregister(ctx, req.Args)
	case "agent_check_update":
		return p.execAgentCheckUpdate(ctx, req.Args)

	// Operator / Status
	case "status_leader":
		return p.execStatusLeader(ctx, req.Args)
	case "status_peers":
		return p.execStatusPeers(ctx, req.Args)
	case "operator_raft_config":
		return p.execOperatorRaftConfig(ctx, req.Args)

	// ACL
	case "acl_token_self":
		return p.execACLTokenSelf(ctx, req.Args)
	case "acl_tokens_list":
		return p.execACLTokensList(ctx, req.Args)
	case "acl_token_read":
		return p.execACLTokenRead(ctx, req.Args)
	case "acl_policies_list":
		return p.execACLPoliciesList(ctx, req.Args)
	case "acl_policy_read":
		return p.execACLPolicyRead(ctx, req.Args)

	// Events
	case "event_list":
		return p.execEventList(ctx, req.Args)

	// Sessions
	case "session_list":
		return p.execSessionList(ctx, req.Args)
	case "session_info":
		return p.execSessionInfo(ctx, req.Args)

	default:
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("unknown tool: %s", req.Tool),
			IsError: true,
		}, nil
	}
}

// =============================================================================
// Catalog
// =============================================================================

func (p *ConsulToolsPlugin) execCatalogDatacenters(_ context.Context, _ plugins.Args) (*plugins.ExecuteResponse, error) {
	dcs, err := p.driver.client.Catalog().Datacenters()
	if err != nil {
		return errorf("catalog_datacenters: %v", err), nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"datacenters": dcs}}, nil
}

func (p *ConsulToolsPlugin) execCatalogNodes(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	q := queryOptions(args)
	if f := args.Get("filter").StringOr(""); f != "" {
		q.Filter = f
	}
	nodes, _, err := p.driver.client.Catalog().Nodes(q)
	if err != nil {
		return errorf("catalog_nodes: %v", err), nil
	}

	type entry struct {
		ID         string            `json:"id"`
		Node       string            `json:"node"`
		Address    string            `json:"address"`
		Datacenter string            `json:"datacenter"`
		Meta       map[string]string `json:"meta,omitempty"`
	}
	out := make([]entry, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, entry{
			ID:         n.ID,
			Node:       n.Node,
			Address:    n.Address,
			Datacenter: n.Datacenter,
			Meta:       n.Meta,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"nodes": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execCatalogServices(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	services, _, err := p.driver.client.Catalog().Services(queryOptions(args))
	if err != nil {
		return errorf("catalog_services: %v", err), nil
	}

	type entry struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	out := make([]entry, 0, len(services))
	for name, tags := range services {
		out = append(out, entry{Name: name, Tags: tags})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"services": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execCatalogService(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	service := args.Get("service").StringOr("")
	if service == "" {
		return &plugins.ExecuteResponse{Result: `"service" is required`, IsError: true}, nil
	}
	tag := args.Get("tag").StringOr("")

	entries, _, err := p.driver.client.Catalog().Service(service, tag, queryOptions(args))
	if err != nil {
		return errorf("catalog_service: %v", err), nil
	}

	type entry struct {
		Node           string            `json:"node"`
		NodeAddress    string            `json:"node_address"`
		Datacenter     string            `json:"datacenter"`
		ServiceID      string            `json:"service_id"`
		ServiceName    string            `json:"service_name"`
		ServiceAddress string            `json:"service_address"`
		ServicePort    int               `json:"service_port"`
		ServiceTags    []string          `json:"service_tags"`
		ServiceMeta    map[string]string `json:"service_meta,omitempty"`
	}
	out := make([]entry, 0, len(entries))
	for _, e := range entries {
		out = append(out, entry{
			Node:           e.Node,
			NodeAddress:    e.Address,
			Datacenter:     e.Datacenter,
			ServiceID:      e.ServiceID,
			ServiceName:    e.ServiceName,
			ServiceAddress: e.ServiceAddress,
			ServicePort:    e.ServicePort,
			ServiceTags:    e.ServiceTags,
			ServiceMeta:    e.ServiceMeta,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"service": service, "nodes": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execCatalogNode(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	node := args.Get("node").StringOr("")
	if node == "" {
		return &plugins.ExecuteResponse{Result: `"node" is required`, IsError: true}, nil
	}
	info, _, err := p.driver.client.Catalog().Node(node, queryOptions(args))
	if err != nil {
		return errorf("catalog_node: %v", err), nil
	}
	if info == nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("node %q not found", node), IsError: true}, nil
	}

	type svcEntry struct {
		ID      string            `json:"id"`
		Name    string            `json:"name"`
		Address string            `json:"address"`
		Port    int               `json:"port"`
		Tags    []string          `json:"tags"`
		Meta    map[string]string `json:"meta,omitempty"`
	}
	svcs := make([]svcEntry, 0, len(info.Services))
	for _, s := range info.Services {
		svcs = append(svcs, svcEntry{
			ID:      s.ID,
			Name:    s.Service,
			Address: s.Address,
			Port:    s.Port,
			Tags:    s.Tags,
			Meta:    s.Meta,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{
		"node":     info.Node.Node,
		"address":  info.Node.Address,
		"services": svcs,
	}}, nil
}

func (p *ConsulToolsPlugin) execCatalogDeregister(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	node := args.Get("node").StringOr("")
	if node == "" {
		return &plugins.ExecuteResponse{Result: `"node" is required`, IsError: true}, nil
	}
	dereg := &api.CatalogDeregistration{Node: node}
	if sid, ok := args.Get("service_id").String(); ok {
		dereg.ServiceID = sid
	}
	if cid, ok := args.Get("check_id").String(); ok {
		dereg.CheckID = cid
	}
	if _, err := p.driver.client.Catalog().Deregister(dereg, writeOptions(args)); err != nil {
		return errorf("catalog_deregister: %v", err), nil
	}
	p.driver.log.Info("catalog deregister", "node", node, "service_id", dereg.ServiceID, "check_id", dereg.CheckID)
	return &plugins.ExecuteResponse{Result: map[string]any{"ok": true, "node": node}}, nil
}

// =============================================================================
// Health
// =============================================================================

func (p *ConsulToolsPlugin) execHealthNode(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	node := args.Get("node").StringOr("")
	if node == "" {
		return &plugins.ExecuteResponse{Result: `"node" is required`, IsError: true}, nil
	}
	checks, _, err := p.driver.client.Health().Node(node, queryOptions(args))
	if err != nil {
		return errorf("health_node: %v", err), nil
	}

	type entry struct {
		CheckID     string `json:"check_id"`
		Name        string `json:"name"`
		ServiceID   string `json:"service_id,omitempty"`
		ServiceName string `json:"service_name,omitempty"`
		Status      string `json:"status"`
		Output      string `json:"output,omitempty"`
	}
	out := make([]entry, 0, len(checks))
	for _, c := range checks {
		out = append(out, entry{
			CheckID:     c.CheckID,
			Name:        c.Name,
			ServiceID:   c.ServiceID,
			ServiceName: c.ServiceName,
			Status:      c.Status,
			Output:      c.Output,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"node": node, "checks": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execHealthChecks(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	state := args.Get("state").StringOr("any")
	checks, _, err := p.driver.client.Health().State(state, queryOptions(args))
	if err != nil {
		return errorf("health_checks: %v", err), nil
	}

	type entry struct {
		Node        string `json:"node"`
		CheckID     string `json:"check_id"`
		Name        string `json:"name"`
		ServiceID   string `json:"service_id,omitempty"`
		ServiceName string `json:"service_name,omitempty"`
		Status      string `json:"status"`
		Output      string `json:"output,omitempty"`
	}
	out := make([]entry, 0, len(checks))
	for _, c := range checks {
		out = append(out, entry{
			Node:        c.Node,
			CheckID:     c.CheckID,
			Name:        c.Name,
			ServiceID:   c.ServiceID,
			ServiceName: c.ServiceName,
			Status:      c.Status,
			Output:      c.Output,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"state": state, "checks": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execHealthChecksService(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	service := args.Get("service").StringOr("")
	if service == "" {
		return &plugins.ExecuteResponse{Result: `"service" is required`, IsError: true}, nil
	}
	checks, _, err := p.driver.client.Health().Checks(service, queryOptions(args))
	if err != nil {
		return errorf("health_checks_service: %v", err), nil
	}

	type entry struct {
		Node      string `json:"node"`
		CheckID   string `json:"check_id"`
		Name      string `json:"name"`
		ServiceID string `json:"service_id,omitempty"`
		Status    string `json:"status"`
		Output    string `json:"output,omitempty"`
	}
	out := make([]entry, 0, len(checks))
	for _, c := range checks {
		out = append(out, entry{
			Node:      c.Node,
			CheckID:   c.CheckID,
			Name:      c.Name,
			ServiceID: c.ServiceID,
			Status:    c.Status,
			Output:    c.Output,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"service": service, "checks": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execHealthService(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	service := args.Get("service").StringOr("")
	if service == "" {
		return &plugins.ExecuteResponse{Result: `"service" is required`, IsError: true}, nil
	}
	tag := args.Get("tag").StringOr("")
	passingOnly := args.Get("passing_only").BoolOr(false)

	entries, _, err := p.driver.client.Health().Service(service, tag, passingOnly, queryOptions(args))
	if err != nil {
		return errorf("health_service: %v", err), nil
	}

	type checkSummary struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Output string `json:"output,omitempty"`
	}
	type instance struct {
		Node        string         `json:"node"`
		NodeAddress string         `json:"node_address"`
		ServiceID   string         `json:"service_id"`
		ServiceName string         `json:"service_name"`
		ServicePort int            `json:"service_port"`
		Checks      []checkSummary `json:"checks"`
	}
	out := make([]instance, 0, len(entries))
	for _, e := range entries {
		cs := make([]checkSummary, 0, len(e.Checks))
		for _, c := range e.Checks {
			cs = append(cs, checkSummary{Name: c.Name, Status: c.Status, Output: c.Output})
		}
		out = append(out, instance{
			Node:        e.Node.Node,
			NodeAddress: e.Node.Address,
			ServiceID:   e.Service.ID,
			ServiceName: e.Service.Service,
			ServicePort: e.Service.Port,
			Checks:      cs,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"service": service, "instances": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execHealthConnect(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	service := args.Get("service").StringOr("")
	if service == "" {
		return &plugins.ExecuteResponse{Result: `"service" is required`, IsError: true}, nil
	}
	tag := args.Get("tag").StringOr("")
	passingOnly := args.Get("passing_only").BoolOr(false)

	entries, _, err := p.driver.client.Health().Connect(service, tag, passingOnly, queryOptions(args))
	if err != nil {
		return errorf("health_connect: %v", err), nil
	}

	type instance struct {
		Node        string `json:"node"`
		NodeAddress string `json:"node_address"`
		ServiceID   string `json:"service_id"`
		ServiceName string `json:"service_name"`
		ServicePort int    `json:"service_port"`
		Status      string `json:"status"`
	}
	out := make([]instance, 0, len(entries))
	for _, e := range entries {
		status := "passing"
		for _, c := range e.Checks {
			if c.Status == "critical" {
				status = "critical"
				break
			} else if c.Status == "warning" {
				status = "warning"
			}
		}
		out = append(out, instance{
			Node:        e.Node.Node,
			NodeAddress: e.Node.Address,
			ServiceID:   e.Service.ID,
			ServiceName: e.Service.Service,
			ServicePort: e.Service.Port,
			Status:      status,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"service": service, "instances": out, "count": len(out)}}, nil
}

// =============================================================================
// KV
// =============================================================================

func (p *ConsulToolsPlugin) execKVGet(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	key := args.Get("key").StringOr("")
	if key == "" {
		return &plugins.ExecuteResponse{Result: `"key" is required`, IsError: true}, nil
	}
	pair, _, err := p.driver.client.KV().Get(key, queryOptions(args))
	if err != nil {
		return errorf("kv_get: %v", err), nil
	}
	if pair == nil {
		return &plugins.ExecuteResponse{Result: map[string]any{"key": key, "found": false}}, nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{
		"key":          pair.Key,
		"value":        string(pair.Value),
		"flags":        pair.Flags,
		"session":      pair.Session,
		"modify_index": pair.ModifyIndex,
		"found":        true,
	}}, nil
}

func (p *ConsulToolsPlugin) execKVList(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	prefix := args.Get("prefix").StringOr("")
	keys, _, err := p.driver.client.KV().Keys(prefix, "", queryOptions(args))
	if err != nil {
		return errorf("kv_list: %v", err), nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"prefix": prefix, "keys": keys, "count": len(keys)}}, nil
}

func (p *ConsulToolsPlugin) execKVPut(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	key := args.Get("key").StringOr("")
	value := args.Get("value").StringOr("")
	if key == "" {
		return &plugins.ExecuteResponse{Result: `"key" is required`, IsError: true}, nil
	}
	if _, err := p.driver.client.KV().Put(&api.KVPair{Key: key, Value: []byte(value)}, writeOptions(args)); err != nil {
		return errorf("kv_put: %v", err), nil
	}
	p.driver.log.Info("kv put", "key", key)
	return &plugins.ExecuteResponse{Result: map[string]any{"key": key, "ok": true}}, nil
}

func (p *ConsulToolsPlugin) execKVDelete(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	key := args.Get("key").StringOr("")
	if key == "" {
		return &plugins.ExecuteResponse{Result: `"key" is required`, IsError: true}, nil
	}
	recurse := args.Get("recurse").BoolOr(false)
	var err error
	if recurse {
		_, err = p.driver.client.KV().DeleteTree(key, writeOptions(args))
	} else {
		_, err = p.driver.client.KV().Delete(key, writeOptions(args))
	}
	if err != nil {
		return errorf("kv_delete: %v", err), nil
	}
	p.driver.log.Info("kv delete", "key", key, "recurse", recurse)
	return &plugins.ExecuteResponse{Result: map[string]any{"key": key, "recurse": recurse, "ok": true}}, nil
}

// =============================================================================
// Agent
// =============================================================================

func (p *ConsulToolsPlugin) execAgentSelf(_ context.Context, _ plugins.Args) (*plugins.ExecuteResponse, error) {
	info, err := p.driver.client.Agent().Self()
	if err != nil {
		return errorf("agent_self: %v", err), nil
	}
	config, _ := info["Config"]
	member, _ := info["Member"]
	return &plugins.ExecuteResponse{Result: map[string]any{
		"config": config,
		"member": member,
	}}, nil
}

func (p *ConsulToolsPlugin) execAgentChecks(_ context.Context, _ plugins.Args) (*plugins.ExecuteResponse, error) {
	checks, err := p.driver.client.Agent().Checks()
	if err != nil {
		return errorf("agent_checks: %v", err), nil
	}

	type entry struct {
		CheckID     string `json:"check_id"`
		Name        string `json:"name"`
		ServiceID   string `json:"service_id,omitempty"`
		ServiceName string `json:"service_name,omitempty"`
		Status      string `json:"status"`
		Type        string `json:"type,omitempty"`
		Output      string `json:"output,omitempty"`
	}
	out := make([]entry, 0, len(checks))
	for _, c := range checks {
		out = append(out, entry{
			CheckID:     c.CheckID,
			Name:        c.Name,
			ServiceID:   c.ServiceID,
			ServiceName: c.ServiceName,
			Status:      c.Status,
			Type:        c.Type,
			Output:      c.Output,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"checks": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execAgentMembers(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	wan := args.Get("wan").BoolOr(false)
	members, err := p.driver.client.Agent().Members(wan)
	if err != nil {
		return errorf("agent_members: %v", err), nil
	}

	type entry struct {
		Name   string            `json:"name"`
		Addr   string            `json:"address"`
		Port   uint16            `json:"port"`
		Status int               `json:"status"`
		Tags   map[string]string `json:"tags,omitempty"`
	}
	out := make([]entry, 0, len(members))
	for _, m := range members {
		out = append(out, entry{
			Name:   m.Name,
			Addr:   m.Addr,
			Port:   m.Port,
			Status: m.Status,
			Tags:   m.Tags,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"members": out, "count": len(out), "wan": wan}}, nil
}

func (p *ConsulToolsPlugin) execAgentServices(_ context.Context, _ plugins.Args) (*plugins.ExecuteResponse, error) {
	services, err := p.driver.client.Agent().Services()
	if err != nil {
		return errorf("agent_services: %v", err), nil
	}

	type entry struct {
		ID      string            `json:"id"`
		Name    string            `json:"service"`
		Address string            `json:"address"`
		Port    int               `json:"port"`
		Tags    []string          `json:"tags"`
		Meta    map[string]string `json:"meta,omitempty"`
	}
	out := make([]entry, 0, len(services))
	for _, s := range services {
		out = append(out, entry{
			ID:      s.ID,
			Name:    s.Service,
			Address: s.Address,
			Port:    s.Port,
			Tags:    s.Tags,
			Meta:    s.Meta,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"services": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execAgentMaintenance(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	enable := args.Get("enable").BoolOr(false)
	reason := args.Get("reason").StringOr("")

	var err error
	if enable {
		err = p.driver.client.Agent().EnableNodeMaintenance(reason)
	} else {
		err = p.driver.client.Agent().DisableNodeMaintenance()
	}
	if err != nil {
		return errorf("agent_maintenance: %v", err), nil
	}
	p.driver.log.Info("agent maintenance", "enable", enable, "reason", reason)
	return &plugins.ExecuteResponse{Result: map[string]any{"enable": enable, "ok": true}}, nil
}

func (p *ConsulToolsPlugin) execAgentReload(_ context.Context, _ plugins.Args) (*plugins.ExecuteResponse, error) {
	if err := p.driver.client.Agent().Reload(); err != nil {
		return errorf("agent_reload: %v", err), nil
	}
	p.driver.log.Info("agent reload triggered")
	return &plugins.ExecuteResponse{Result: map[string]any{"ok": true}}, nil
}

func (p *ConsulToolsPlugin) execAgentServiceRegister(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	name := args.Get("name").StringOr("")
	if name == "" {
		return &plugins.ExecuteResponse{Result: `"name" is required`, IsError: true}, nil
	}
	reg := &api.AgentServiceRegistration{Name: name}
	if id, ok := args.Get("id").String(); ok {
		reg.ID = id
	}
	if addr, ok := args.Get("address").String(); ok {
		reg.Address = addr
	}
	if port, ok := args.Get("port").Int(); ok {
		reg.Port = port
	}
	if tags := args.Get("tags").StringSlice(); len(tags) > 0 {
		reg.Tags = tags
	}
	if err := p.driver.client.Agent().ServiceRegister(reg); err != nil {
		return errorf("agent_service_register: %v", err), nil
	}
	id := reg.ID
	if id == "" {
		id = name
	}
	p.driver.log.Info("agent service registered", "name", name, "id", id)
	return &plugins.ExecuteResponse{Result: map[string]any{"name": name, "id": id, "ok": true}}, nil
}

func (p *ConsulToolsPlugin) execAgentServiceDeregister(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	serviceID := args.Get("service_id").StringOr("")
	if serviceID == "" {
		return &plugins.ExecuteResponse{Result: `"service_id" is required`, IsError: true}, nil
	}
	if err := p.driver.client.Agent().ServiceDeregister(serviceID); err != nil {
		return errorf("agent_service_deregister: %v", err), nil
	}
	p.driver.log.Info("agent service deregistered", "service_id", serviceID)
	return &plugins.ExecuteResponse{Result: map[string]any{"service_id": serviceID, "ok": true}}, nil
}

func (p *ConsulToolsPlugin) execAgentCheckDeregister(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	checkID := args.Get("check_id").StringOr("")
	if checkID == "" {
		return &plugins.ExecuteResponse{Result: `"check_id" is required`, IsError: true}, nil
	}
	if err := p.driver.client.Agent().CheckDeregister(checkID); err != nil {
		return errorf("agent_check_deregister: %v", err), nil
	}
	p.driver.log.Info("agent check deregistered", "check_id", checkID)
	return &plugins.ExecuteResponse{Result: map[string]any{"check_id": checkID, "ok": true}}, nil
}

func (p *ConsulToolsPlugin) execAgentCheckUpdate(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	checkID := args.Get("check_id").StringOr("")
	status := args.Get("status").StringOr("")
	output := args.Get("output").StringOr("")
	if checkID == "" {
		return &plugins.ExecuteResponse{Result: `"check_id" is required`, IsError: true}, nil
	}
	if err := p.driver.client.Agent().UpdateTTL(checkID, output, status); err != nil {
		return errorf("agent_check_update: %v", err), nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"check_id": checkID, "status": status, "ok": true}}, nil
}

// =============================================================================
// Operator / Status
// =============================================================================

func (p *ConsulToolsPlugin) execStatusLeader(_ context.Context, _ plugins.Args) (*plugins.ExecuteResponse, error) {
	leader, err := p.driver.client.Status().Leader()
	if err != nil {
		return errorf("status_leader: %v", err), nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"leader": leader}}, nil
}

func (p *ConsulToolsPlugin) execStatusPeers(_ context.Context, _ plugins.Args) (*plugins.ExecuteResponse, error) {
	peers, err := p.driver.client.Status().Peers()
	if err != nil {
		return errorf("status_peers: %v", err), nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"peers": peers, "count": len(peers)}}, nil
}

func (p *ConsulToolsPlugin) execOperatorRaftConfig(_ context.Context, _ plugins.Args) (*plugins.ExecuteResponse, error) {
	cfg, err := p.driver.client.Operator().RaftGetConfiguration(nil)
	if err != nil {
		return errorf("operator_raft_config: %v", err), nil
	}

	type server struct {
		ID      string `json:"id"`
		Node    string `json:"node"`
		Address string `json:"address"`
		Leader  bool   `json:"leader"`
		Voter   bool   `json:"voter"`
	}
	servers := make([]server, 0, len(cfg.Servers))
	for _, s := range cfg.Servers {
		servers = append(servers, server{
			ID:      string(s.ID),
			Node:    string(s.Node),
			Address: string(s.Address),
			Leader:  s.Leader,
			Voter:   s.Voter,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"servers": servers, "index": cfg.Index}}, nil
}

// =============================================================================
// ACL
// =============================================================================

func (p *ConsulToolsPlugin) execACLTokenSelf(_ context.Context, _ plugins.Args) (*plugins.ExecuteResponse, error) {
	token, _, err := p.driver.client.ACL().TokenReadSelf(nil)
	if err != nil {
		return errorf("acl_token_self: %v", err), nil
	}
	policies := make([]string, 0, len(token.Policies))
	for _, p := range token.Policies {
		policies = append(policies, p.Name)
	}
	return &plugins.ExecuteResponse{Result: map[string]any{
		"accessor_id": token.AccessorID,
		"description": token.Description,
		"policies":    policies,
		"local":       token.Local,
	}}, nil
}

func (p *ConsulToolsPlugin) execACLTokensList(_ context.Context, _ plugins.Args) (*plugins.ExecuteResponse, error) {
	tokens, _, err := p.driver.client.ACL().TokenList(nil)
	if err != nil {
		return errorf("acl_tokens_list: %v", err), nil
	}

	type entry struct {
		AccessorID  string   `json:"accessor_id"`
		Description string   `json:"description"`
		Policies    []string `json:"policies"`
		Local       bool     `json:"local"`
	}
	out := make([]entry, 0, len(tokens))
	for _, t := range tokens {
		policies := make([]string, 0, len(t.Policies))
		for _, p := range t.Policies {
			policies = append(policies, p.Name)
		}
		out = append(out, entry{
			AccessorID:  t.AccessorID,
			Description: t.Description,
			Policies:    policies,
			Local:       t.Local,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"tokens": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execACLTokenRead(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	accessorID := args.Get("accessor_id").StringOr("")
	if accessorID == "" {
		return &plugins.ExecuteResponse{Result: `"accessor_id" is required`, IsError: true}, nil
	}
	token, _, err := p.driver.client.ACL().TokenRead(accessorID, nil)
	if err != nil {
		return errorf("acl_token_read: %v", err), nil
	}
	policies := make([]string, 0, len(token.Policies))
	for _, p := range token.Policies {
		policies = append(policies, p.Name)
	}
	return &plugins.ExecuteResponse{Result: map[string]any{
		"accessor_id": token.AccessorID,
		"description": token.Description,
		"policies":    policies,
		"local":       token.Local,
	}}, nil
}

func (p *ConsulToolsPlugin) execACLPoliciesList(_ context.Context, _ plugins.Args) (*plugins.ExecuteResponse, error) {
	policies, _, err := p.driver.client.ACL().PolicyList(nil)
	if err != nil {
		return errorf("acl_policies_list: %v", err), nil
	}

	type entry struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}
	out := make([]entry, 0, len(policies))
	for _, p := range policies {
		out = append(out, entry{ID: p.ID, Name: p.Name, Description: p.Description})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"policies": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execACLPolicyRead(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	policyID := args.Get("policy_id").StringOr("")
	if policyID == "" {
		return &plugins.ExecuteResponse{Result: `"policy_id" is required`, IsError: true}, nil
	}
	policy, _, err := p.driver.client.ACL().PolicyRead(policyID, nil)
	if err != nil {
		return errorf("acl_policy_read: %v", err), nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{
		"id":          policy.ID,
		"name":        policy.Name,
		"description": policy.Description,
		"rules":       policy.Rules,
	}}, nil
}

// =============================================================================
// Events
// =============================================================================

func (p *ConsulToolsPlugin) execEventList(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	name := args.Get("name").StringOr("")
	events, _, err := p.driver.client.Event().List(name, nil)
	if err != nil {
		return errorf("event_list: %v", err), nil
	}

	type entry struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Payload string `json:"payload,omitempty"`
	}
	out := make([]entry, 0, len(events))
	for _, e := range events {
		out = append(out, entry{ID: e.ID, Name: e.Name, Payload: string(e.Payload)})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"events": out, "count": len(out)}}, nil
}

// =============================================================================
// Sessions
// =============================================================================

func (p *ConsulToolsPlugin) execSessionList(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	sessions, _, err := p.driver.client.Session().List(queryOptions(args))
	if err != nil {
		return errorf("session_list: %v", err), nil
	}

	type entry struct {
		ID        string   `json:"id"`
		Name      string   `json:"name,omitempty"`
		Node      string   `json:"node"`
		LockDelay string   `json:"lock_delay"`
		Behavior  string   `json:"behavior"`
		Checks    []string `json:"checks,omitempty"`
	}
	out := make([]entry, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, entry{
			ID:        s.ID,
			Name:      s.Name,
			Node:      s.Node,
			LockDelay: s.LockDelay.String(),
			Behavior:  s.Behavior,
			Checks:    s.Checks,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"sessions": out, "count": len(out)}}, nil
}

func (p *ConsulToolsPlugin) execSessionInfo(_ context.Context, args plugins.Args) (*plugins.ExecuteResponse, error) {
	sessionID := args.Get("session_id").StringOr("")
	if sessionID == "" {
		return &plugins.ExecuteResponse{Result: `"session_id" is required`, IsError: true}, nil
	}
	entry, _, err := p.driver.client.Session().Info(sessionID, queryOptions(args))
	if err != nil {
		return errorf("session_info: %v", err), nil
	}
	if entry == nil {
		return &plugins.ExecuteResponse{Result: map[string]any{"session_id": sessionID, "found": false}}, nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{
		"id":         entry.ID,
		"name":       entry.Name,
		"node":       entry.Node,
		"lock_delay": entry.LockDelay.String(),
		"behavior":   entry.Behavior,
		"checks":     entry.Checks,
		"found":      true,
	}}, nil
}

// =============================================================================
// Helpers
// =============================================================================

func queryOptions(args plugins.Args) *api.QueryOptions {
	q := &api.QueryOptions{}
	if dc := args.Get("datacenter").StringOr(""); dc != "" {
		q.Datacenter = dc
	}
	return q
}

func writeOptions(args plugins.Args) *api.WriteOptions {
	w := &api.WriteOptions{}
	if dc := args.Get("datacenter").StringOr(""); dc != "" {
		w.Datacenter = dc
	}
	return w
}

func errorf(format string, args ...any) *plugins.ExecuteResponse {
	return &plugins.ExecuteResponse{
		Result:  fmt.Sprintf(format, args...),
		IsError: true,
	}
}

func matchesFilter(def plugins.ToolDefinition, f plugins.ListToolsFilter) bool {
	if def.Deprecated && !f.Deprecated {
		return false
	}
	if f.Prefix != "" && !strings.HasPrefix(def.Name, f.Prefix) {
		return false
	}
	if len(f.Tags) > 0 {
		for _, want := range f.Tags {
			for _, have := range def.Tags {
				if have == want {
					goto tagMatched
				}
			}
		}
		return false
	tagMatched:
	}
	return true
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func splitTrimmed(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := parts[:0]
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
