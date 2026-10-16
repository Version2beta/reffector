// Command reffector reports statically reachable Go side effects.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Version2beta/reffector/internal/analyzer"
	"github.com/Version2beta/reffector/internal/output"
	"github.com/Version2beta/reffector/internal/skillinstall"
	"github.com/Version2beta/reffector/skills/reffector"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "skill" {
		return runSkill(args[1:], stdout, stderr)
	}
	flags := flag.NewFlagSet("reffector", flag.ContinueOnError)
	flags.SetOutput(stderr)
	direct := flags.Bool("direct", false, "report only direct effects")
	tests := flags.Bool("tests", false, "include functions defined in test files")
	json := flags.Bool("json", false, "write a JSON array of effect records")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: reffector [flags] [packages]")
		fmt.Fprintln(stderr, "Report statically reachable side effects; packages default to .")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	records, err := analyzer.Analyze(flags.Args(), analyzer.Options{Direct: *direct, Tests: *tests})
	if err != nil {
		fmt.Fprintln(stderr, "reffector:", err)
		return 1
	}
	if err := output.Write(stdout, records, *json); err != nil {
		fmt.Fprintln(stderr, "reffector:", err)
		return 1
	}
	return 0
}

func runSkill(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "install" {
		fmt.Fprintln(stderr, "Usage: reffector skill install")
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(stderr, "reffector: determine home directory:", err)
		return 1
	}
	result, err := skillinstall.Install(skillinstall.Config{Home: home, Name: "reffector", Content: reffectorskill.Content})
	if err != nil {
		fmt.Fprintln(stderr, "reffector:", err)
		return 1
	}
	if result.Updated {
		fmt.Fprintln(stdout, "Installed skill:", result.Path)
	} else {
		fmt.Fprintln(stdout, "Skill is current:", result.Path)
	}
	return 0
}
