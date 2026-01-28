package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	BuildTime = ""
	GitCommit = ""
)

var rootCmd = &cobra.Command{
	Use:     "supalite",
	Short:   "Supalite - lightweight Supabase-compatible backend",
	Long:    `A single-binary backend with embedded PostgreSQL, pREST, and Supabase Auth (GoTrue).`,
	Version: Version,
}

func init() {
	versionTmpl := "supalite version {{.Version}}"
	if BuildTime != "" {
		versionTmpl += " (built " + BuildTime
		if GitCommit != "" {
			versionTmpl += ", commit " + GitCommit
		}
		versionTmpl += ")"
	}
	versionTmpl += "\n"
	rootCmd.SetVersionTemplate(versionTmpl)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
