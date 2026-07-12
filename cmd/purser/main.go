// Command purser is the composition root: the only place that wires
// concrete adapters to services, installs the slog handler, and (once
// telemetry export is configured) constructs the OTel SDK. See
// docs/adr/0001-hexagonal-architecture.md, docs/adr/0007-telemetry.md, and
// docs/adr/0008-structured-logging.md.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
