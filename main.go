package main

import "github.com/fishfisher/homeyctl/cmd"

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cmd.SetSkillFS(skillFS, "homeyctl-skill", "homeyctl")
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}
