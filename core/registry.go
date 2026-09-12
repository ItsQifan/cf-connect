package core

import (
	"fmt"
	"strings"
)

// PlatformFactory creates a Platform from config options.
type PlatformFactory func(opts map[string]any) (Platform, error)

// AgentFactory creates an Agent from config options.
type AgentFactory func(opts map[string]any) (Agent, error)

var (
	platformFactories = make(map[string]PlatformFactory)
	agentFactories    = make(map[string]AgentFactory)
)

func RegisterPlatform(name string, factory PlatformFactory) {
	platformFactories[name] = factory
}

func RegisterAgent(name string, factory AgentFactory) {
	agentFactories[name] = factory
}

func CreatePlatform(name string, opts map[string]any) (Platform, error) {
	f, ok := platformFactories[name]
	if !ok {
		available := make([]string, 0, len(platformFactories))
		for k := range platformFactories {
			available = append(available, k)
		}
		return nil, fmt.Errorf("unknown platform %q, available: %v", name, available)
	}
	return f(opts)
}

func ListRegisteredAgents() []string {
	names := make([]string, 0, len(agentFactories))
	for k := range agentFactories {
		names = append(names, k)
	}
	return names
}

func ListRegisteredPlatforms() []string {
	names := make([]string, 0, len(platformFactories))
	for k := range platformFactories {
		names = append(names, k)
	}
	return names
}

func CreateAgent(name string, opts map[string]any) (Agent, error) {
	f, ok := agentFactories[name]
	if !ok {
		available := make([]string, 0, len(agentFactories))
		for k := range agentFactories {
			available = append(available, k)
		}
		return nil, fmt.Errorf("unknown agent %q, available: %v", name, available)
	}
	return f(opts)
}

// AgentDisplayName returns the label to show users for this agent.
//
// Agent.Name() is the registry key used to create instances and to tag session
// ownership, so it stays stable ("opencode"). But one adapter can drive more
// than one CLI brand — the opencode adapter also runs codefree-o, a rebrand —
// and showing the registry key in chat would label a CodeFree-O deployment as
// "opencode 会话列表".
//
// Agents that implement AgentDoctorInfo expose the name of the CLI they are
// actually configured to drive, so prefer that and fall back to the registry
// name for agents that do not.
//
// Use this for anything a user reads (list/status/skills titles, progress
// labels). Keep using Agent.Name() for registry lookups, config keys, preset
// lookups and session bookkeeping.
func AgentDisplayName(agent Agent) string {
	if agent == nil {
		return ""
	}
	if info, ok := agent.(AgentDoctorInfo); ok {
		if n := strings.TrimSpace(info.CLIDisplayName()); n != "" {
			return n
		}
	}
	return agent.Name()
}
