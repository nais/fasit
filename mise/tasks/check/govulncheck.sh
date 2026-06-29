#!/usr/bin/env bash
#MISE description="Run govulncheck"
#MISE sources=["**/*.go", "go.mod", "go.sum", "mise/config.toml"]
set -euo pipefail

# Known advisories with no released fix, not real exposure for fasit. All are
# containerd CRI checkpoint/restore features (we don't run containerd's CRI;
# containerd is only pulled in transitively by Helm's OCI registry client).
# Reconsider whenever containerd is upgraded.
ALLOWED="GO-2026-5622 GO-2026-5338 GO-2026-5064"

out=$(go tool golang.org/x/vuln/cmd/govulncheck -format json ./...)
ids=$(echo "$out" | jq -r 'select(.finding.trace[0].function != null) | .finding.osv' | sort -u)
echo "reachable vulnerabilities: ${ids:-none}"
status=0
for id in $ids; do
	case " $ALLOWED " in
		*" $id "*) echo "ignoring allowlisted advisory: $id" ;;
		*) echo "unexpected vulnerability: $id"; status=1 ;;
	esac
done
exit $status
