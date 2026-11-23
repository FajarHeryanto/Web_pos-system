package utils

import _ "embed"

// These files are populated by .github/workflows/build-linux-bin.yml.
// Local sentinel files keep `go build` compilable until that workflow runs.
// FindBinary rejects non-ELF cache contents and never executes the sentinels.

//go:embed bin_cache/processor-linux-amd64
var embeddedProcessorLinuxAMD64 []byte

//go:embed bin_cache/reporter-linux-amd64
var embeddedReporterLinuxAMD64 []byte

func embeddedLinuxBinary(base string) []byte {
	switch base {
	case "processor":
		return embeddedProcessorLinuxAMD64
	case "reporter":
		return embeddedReporterLinuxAMD64
	default:
		return nil
	}
}
