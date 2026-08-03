package sandbox

import (
	"fmt"
	"strings"
)

// GenerateFirewallScript creates an iptables-based firewall script that
// resolves allowed domains to IPs and blocks all other outbound traffic.
// This is only useful when the policy has network=true with specific domains.
func GenerateFirewallScript(domains []string) string {
	if len(domains) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString("#!/bin/sh\n")
	b.WriteString("# Trovery Forge domain allowlist firewall\n")
	b.WriteString("# Auto-generated — do not edit\n\n")

	// Allow loopback
	b.WriteString("iptables -A OUTPUT -o lo -j ACCEPT\n")

	// Allow established connections
	b.WriteString("iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT\n\n")

	// Allow DNS so we can resolve domains
	b.WriteString("# Allow DNS resolution\n")
	b.WriteString("iptables -A OUTPUT -p udp --dport 53 -j ACCEPT\n")
	b.WriteString("iptables -A OUTPUT -p tcp --dport 53 -j ACCEPT\n\n")

	// Resolve and allow each domain
	b.WriteString("# Resolve and allow listed domains\n")
	for _, domain := range domains {
		b.WriteString(fmt.Sprintf("for ip in $(getent hosts %s | awk '{print $1}'); do\n", domain))
		b.WriteString(fmt.Sprintf("  iptables -A OUTPUT -d \"$ip\" -j ACCEPT\n"))
		b.WriteString("done\n")
	}

	b.WriteString("\n")

	// Drop everything else
	b.WriteString("# Block all other outbound traffic\n")
	b.WriteString("iptables -A OUTPUT -j DROP\n")

	return b.String()
}
