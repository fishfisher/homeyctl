package cmd

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	skillFS      fs.FS
	skillDirName string
	skillName    string
)

// Known AI tool skill directories.
type skillLocation struct {
	Label string
	Dir   string // relative to home, e.g. ".claude/skills"
}

var knownSkillLocations = []skillLocation{
	{"Claude Code", ".claude/skills"},
	{"OpenAI Codex", ".codex/skills"},
	{"OpenCode", ".config/opencode/skill"},
	{"GitHub Copilot", ".copilot/skills"},
	{"Agents", ".agents/skills"},
}

// SetSkillFS receives the embedded filesystem and metadata from main.
func SetSkillFS(fsys fs.FS, dirName, name string) {
	skillFS = fsys
	skillDirName = dirName
	skillName = name
}

// choose prints a numbered menu and returns the 0-based index, or -1 on invalid input.
func choose(prompt string, options []string) int {
	fmt.Println(prompt)
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	fmt.Print("> ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	n := 0
	if _, err := fmt.Sscanf(line, "%d", &n); err != nil || n < 1 || n > len(options) {
		return -1
	}
	return n - 1
}

var installSkillCmd = &cobra.Command{
	Use:   "install-skill",
	Short: "Install the AI coding skill for this CLI",
	Long:  "Copies the embedded skill files to an AI tool's skill directory so it can discover them globally.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if skillFS == nil {
			return fmt.Errorf("no skill files embedded")
		}

		baseDir, _ := cmd.Flags().GetString("path")
		force, _ := cmd.Flags().GetBool("force")

		if baseDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("finding home directory: %w", err)
			}

			if term.IsTerminal(int(os.Stdin.Fd())) {
				options := make([]string, 0, len(knownSkillLocations)+1)
				for _, loc := range knownSkillLocations {
					options = append(options, loc.Label)
				}
				options = append(options, "Custom path")

				choice := choose("Install skill to:", options)
				if choice < 0 {
					fmt.Println("Aborted.")
					return nil
				}
				if choice < len(knownSkillLocations) {
					baseDir = filepath.Join(home, knownSkillLocations[choice].Dir)
				} else {
					fmt.Print("Enter path: ")
					reader := bufio.NewReader(os.Stdin)
					entered, _ := reader.ReadString('\n')
					entered = strings.TrimSpace(entered)
					if entered == "" {
						fmt.Println("Aborted.")
						return nil
					}
					if strings.HasPrefix(entered, "~/") {
						entered = filepath.Join(home, entered[2:])
					}
					baseDir = entered
				}
			} else {
				// Non-interactive: default to Claude Code
				baseDir = filepath.Join(home, knownSkillLocations[0].Dir)
			}
		}
		destDir := filepath.Join(baseDir, skillName)

		type fileEntry struct {
			relPath string
			embPath string
		}
		var files []fileEntry

		err := fs.WalkDir(skillFS, skillDirName, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(skillDirName, path)
			files = append(files, fileEntry{relPath: rel, embPath: path})
			return nil
		})
		if err != nil {
			return fmt.Errorf("reading embedded files: %w", err)
		}

		var existing []string
		for _, f := range files {
			dest := filepath.Join(destDir, f.relPath)
			if _, err := os.Stat(dest); err == nil {
				existing = append(existing, f.relPath)
			}
		}

		bold := color.New(color.Bold)
		bold.Printf("Installing to %s\n", destDir)
		for _, f := range files {
			marker := ""
			for _, e := range existing {
				if e == f.relPath {
					marker = color.YellowString(" (exists)")
					break
				}
			}
			fmt.Printf("  %s%s\n", f.relPath, marker)
		}

		if len(existing) > 0 && !force {
			fmt.Printf("\n%s Overwrite %d existing file(s)? [y/N] ", color.YellowString("?"), len(existing))
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(answer)) != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		for _, f := range files {
			dest := filepath.Join(destDir, f.relPath)
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			data, err := fs.ReadFile(skillFS, f.embPath)
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.relPath, err)
			}
			if err := os.WriteFile(dest, data, 0644); err != nil {
				return fmt.Errorf("writing %s: %w", f.relPath, err)
			}
		}

		color.Green("Installed %d file(s) to %s", len(files), destDir)
		return nil
	},
}

func init() {
	installSkillCmd.Flags().StringP("path", "p", "", "Parent directory (default: interactive selection)")
	installSkillCmd.Flags().BoolP("force", "f", false, "Overwrite existing files without prompting")
	rootCmd.AddCommand(installSkillCmd)
}
