package main

import "github.com/tbcrawford/opm/cmd"

// version and commit are injected at build time by GoReleaser via ldflags:
//
//	-X main.version={{.Version}} -X main.commit={{.Commit}}
var (
	version = "dev"
	commit  = "none"
)

func main() {
	cmd.SetVersionInfo(version, commit)
	cmd.Execute()
}
