package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/RiddhiKatarki/ctx/internal/clierr"
	"github.com/RiddhiKatarki/ctx/internal/discovery"
	"github.com/RiddhiKatarki/ctx/internal/export"
	importctx "github.com/RiddhiKatarki/ctx/internal/import"
	"github.com/RiddhiKatarki/ctx/internal/inspect"
	"github.com/RiddhiKatarki/ctx/internal/providers"
	"github.com/RiddhiKatarki/ctx/internal/reporter"
	"github.com/RiddhiKatarki/ctx/internal/summary"
)

const version = "2.0.0"

// Output flags — persisted across subcommands.
var (
	jsonOutput   bool
	streamOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "ctx",
	Short: "Context Handoff — export and import AI development session context",
	Long: `ctx is a cross-platform CLI tool that allows developers to export and import
the working context of an AI-assisted software development session.

It serializes the current development state (git metadata, modified files,
prompt history, AI summary) into a portable .ctx bundle so another developer
or AI agent can continue working with minimal onboarding.`,
	Version:    version,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		jsonOutput, _ = cmd.Flags().GetBool("json")
		streamOutput, _ = cmd.Flags().GetBool("stream")
	},
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Capture project context and create a .ctx bundle",
	Long: `Captures the current development state including:
  - Git metadata (branch, HEAD, dirty flag, remote, tag)
  - Modified files (from git diff)
  - Uncommitted changes (git diff)
  - Prompt history (from file or mock)
  - AI-generated project summary

Packages everything into a portable .ctx archive.

Use --json to emit a structured result suitable for scripting and tools.`,
	RunE:           runExport,
	SilenceErrors:  true,
	SilenceUsage:   true,
}

var inspectCmd = &cobra.Command{
	Use:   "inspect [path]",
	Short: "Display summary info from a .ctx bundle without importing",
	Long: `Reads a .ctx bundle and displays a formatted summary of its contents
without extracting or importing the full archive.

Use --json to emit a structured result suitable for tooling.`,
	Args:           cobra.MaximumNArgs(1),
	RunE:           runInspect,
	SilenceErrors:  true,
	SilenceUsage:   true,
}

var importCmd = &cobra.Command{
	Use:   "import [path]",
	Short: "Extract and validate a .ctx bundle",
	Long: `Extracts a .ctx bundle locally, validates the manifest,
and displays a summary of its contents.

Use --outdir to extract the bundle contents to a directory.
Use --json to emit a structured result suitable for tooling.`,
	Args:           cobra.MaximumNArgs(1),
	RunE:           runImport,
	SilenceErrors:  true,
	SilenceUsage:   true,
}

var listCmd = &cobra.Command{
	Use:   "list [dir]",
	Short: "List .ctx bundles in a directory",
	Long: `Scans the given directory non-recursively for .ctx bundles
and returns a quick overview of each (size, project, branch, file count).

Defaults to the current working directory.
Always emits JSON — this command is built for tooling consumption.`,
	Args:          cobra.MaximumNArgs(1),
	RunE:          runList,
	SilenceErrors: true,
	SilenceUsage:  true,
}

var infoCmd = &cobra.Command{
	Use:   "info <path>",
	Short: "Show structured metadata for a .ctx bundle",
	Long: `Reads a .ctx bundle and returns its manifest, metadata, git state,
file list, and bundle size.

Always emits JSON — this command is built for tooling consumption.`,
	Args:          cobra.ExactArgs(1),
	RunE:          runInfo,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Export flags
var (
	exportOutput        string
	exportProjectName   string
	exportPromptsFile   string
	exportPromptsSource string
	exportExtraFiles    string
	exportSummaryProv   string
	exportAPIKey        string
	exportAPIBaseURL    string
	exportModel         string
)

// Import flags
var (
	importOutDir string
)

func init() {
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(infoCmd)

	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "emit machine-readable JSON to stdout (single result object)")
	rootCmd.PersistentFlags().BoolVar(&streamOutput, "stream", false, "emit NDJSON progress events to stdout (one line per stage)")

	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "project.ctx", "output bundle path")
	exportCmd.Flags().StringVar(&exportProjectName, "project-name", "", "override project name (default: repo dir name)")
	exportCmd.Flags().StringVar(&exportPromptsFile, "prompts", "", "path to JSON file with prompt history")
	exportCmd.Flags().StringVar(&exportPromptsSource, "prompts-source", "auto", "prompt source: auto, file, claudecode, opencode, cursor, aider, mock")
	exportCmd.Flags().StringVar(&exportExtraFiles, "files", "", "comma-separated extra file paths to include")
	exportCmd.Flags().StringVar(&exportSummaryProv, "summary-provider", "template", "summary provider: template (default) or openai")
	exportCmd.Flags().StringVar(&exportAPIKey, "api-key", "", "API key for LLM summary provider")
	exportCmd.Flags().StringVar(&exportAPIBaseURL, "api-base-url", "", "base URL for LLM API (default: https://api.openai.com/v1)")
	exportCmd.Flags().StringVar(&exportModel, "model", "", "LLM model name (default: gpt-4o)")

	importCmd.Flags().StringVar(&importOutDir, "outdir", "", "directory to extract bundle contents to")
}

func runExport(cmd *cobra.Command, args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return exitError(clierr.CodeSystem, "failed to get working directory", err)
	}

	var extraFiles []string
	if exportExtraFiles != "" {
		for _, f := range strings.Split(exportExtraFiles, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				extraFiles = append(extraFiles, f)
			}
		}
	}

	promptSource := providers.Source(exportPromptsSource)
	if promptSource == "" {
		promptSource = providers.SourceAuto
	}
	promptProv, err := providers.NewPromptProvider(providers.Options{
		Source:      promptSource,
		PromptsFile: exportPromptsFile,
		WorkingDir:  wd,
	})
	if err != nil {
		return exitError(clierr.CodeUser, "prompt provider configuration error", err)
	}

	summProv, err := summary.NewSummaryProvider(summary.Options{
		Provider:   exportSummaryProv,
		APIKey:     exportAPIKey,
		APIBaseURL: exportAPIBaseURL,
		Model:      exportModel,
	})
	if err != nil {
		return exitError(clierr.CodeUser, "summary provider configuration error", err)
	}

	rep := buildReporter()
	result, runErr := export.Run(export.Config{
		OutputPath:      exportOutput,
		ProjectName:     exportProjectName,
		WorkingDir:      wd,
		PromptProvider:  promptProv,
		SummaryProvider: summProv,
		ExtraFiles:      extraFiles,
		Reporter:        rep,
	})
	if runErr != nil {
		return classifyExportError(runErr)
	}

	// Reporter.Done already emitted JSON/Stream result; human mode
	// has already printed everything via Info().
	_ = result
	return nil
}

func runInspect(cmd *cobra.Command, args []string) error {
	path := "project.ctx"
	if len(args) > 0 {
		path = args[0]
	}

	if _, err := os.Stat(path); err != nil {
		return exitError(clierr.CodeUser, fmt.Sprintf("bundle file not found: %s", path), err)
	}

	rep := buildReporter()
	result, runErr := inspect.Run(inspect.Config{Path: path, Reporter: rep})
	if runErr != nil {
		if isZipErr(runErr) {
			return exitError(clierr.CodeBundle, "invalid .ctx bundle", runErr)
		}
		return exitError(clierr.CodeBundle, "inspect failed", runErr)
	}

	_ = result
	return nil
}

func runImport(cmd *cobra.Command, args []string) error {
	path := "project.ctx"
	if len(args) > 0 {
		path = args[0]
	}

	if _, err := os.Stat(path); err != nil {
		return exitError(clierr.CodeUser, fmt.Sprintf("bundle file not found: %s", path), err)
	}

	rep := buildReporter()
	result, runErr := importctx.Run(importctx.Config{
		Path:      path,
		OutDir:    importOutDir,
		Reporter:  rep,
	})
	if runErr != nil {
		if isZipErr(runErr) {
			return exitError(clierr.CodeBundle, "invalid .ctx bundle", runErr)
		}
		return exitError(clierr.CodeBundle, "import failed", runErr)
	}

	_ = result
	return nil
}

func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("failed to encode result: %w", err)
	}
	return nil
}

// buildReporter returns the Reporter corresponding to the current
// flag state. Stream wins over JSON when both are set (per CLI spec).
func buildReporter() reporter.Reporter {
	mode := reporter.ModeHuman
	switch {
	case streamOutput:
		mode = reporter.ModeStream
	case jsonOutput:
		mode = reporter.ModeJSON
	}
	return reporter.New(mode, os.Stdout, os.Stderr)
}

func emitJSONError(code, message string, cause error) error {
	envelope := map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	if cause != nil {
		envelope["error"].(map[string]any)["cause"] = cause.Error()
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(envelope)
	return &clierr.Error{Code: code, Message: message, Cause: cause}
}

func exitError(code, message string, cause error) error {
	if jsonOutput {
		return emitJSONError(code, message, cause)
	}
	if cause != nil {
		fmt.Fprintf(os.Stderr, "Error: %s: %v\n", message, cause)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", message)
	}
	return &clierr.Error{Code: code, Message: message, Cause: cause}
}

func classifyExportError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not a git repository"):
		return exitError(clierr.CodeUser, "not in a git repository", err)
	case strings.Contains(msg, "git "):
		return exitError(clierr.CodeSystem, "git command failed", err)
	default:
		return exitError(clierr.CodeUser, "export failed", err)
	}
}

func isZipErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not a valid .ctx archive") ||
		strings.Contains(err.Error(), "not a ZIP file")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(exitCodeFor(err))
	}
}

func runList(cmd *cobra.Command, args []string) error {
	dir := ""
	if len(args) > 0 {
		dir = args[0]
	}

	entries, err := discovery.List(dir)
	if err != nil {
		return exitError(clierr.CodeSystem, "list failed", err)
	}

	if entries == nil {
		entries = []discovery.Entry{}
	}

	if jsonOutput {
		result := map[string]any{
			"directory": dir,
			"count":     len(entries),
			"bundles":   entries,
		}
		if dir == "" {
			if wd, err := os.Getwd(); err == nil {
				result["directory"] = wd
			}
		}
		return emitJSON(result)
	}

	// Human-readable table output
	return printListHuman(dir, entries)
}

func printListHuman(dir string, entries []discovery.Entry) error {
	if dir == "" {
		if wd, err := os.Getwd(); err == nil {
			dir = wd
		}
	}

	fmt.Printf("Directory: %s\n", dir)
	fmt.Printf("Bundles:   %d\n\n", len(entries))

	if len(entries) == 0 {
		return nil
	}

	fmt.Printf("%-40s  %10s  %-25s  %-20s  %5s  %s\n", "NAME", "SIZE", "PROJECT", "BRANCH", "FILES", "DIRTY")
	fmt.Println(strings.Repeat("-", 110))
	for _, e := range entries {
		size := formatBytes(e.Size)
		dirtyMark := " "
		if e.Dirty {
			dirtyMark = "*"
		}
		fmt.Printf("%-40s  %10s  %-25s  %-20s  %5d  %s\n",
			truncate(e.Name, 40),
			size,
			truncate(e.ProjectName, 25),
			truncate(e.Branch, 20),
			e.FileCount,
			dirtyMark,
		)
	}
	return nil
}

func formatBytes(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func runInfo(cmd *cobra.Command, args []string) error {
	path := args[0]

	if _, err := os.Stat(path); err != nil {
		return exitError(clierr.CodeUser, fmt.Sprintf("bundle file not found: %s", path), err)
	}

	result, err := discovery.Info(path)
	if err != nil {
		if isZipErr(err) {
			return exitError(clierr.CodeBundle, "invalid .ctx bundle", err)
		}
		return exitError(clierr.CodeSystem, "info failed", err)
	}

	if jsonOutput {
		return emitJSON(result)
	}

	// Human-readable output (also supports --json for tooling)
	fmt.Printf("Path:       %s\n", result.Path)
	fmt.Printf("Size:       %s\n", formatBytes(result.Size))
	fmt.Printf("Valid:      %t\n", result.Valid)
	fmt.Printf("Files:      %d\n", result.FileCount)
	fmt.Println()
	if v, ok := result.Manifest["version"]; ok {
		fmt.Printf("Manifest:   version=%v tool=%v\n", v, result.Manifest["tool"])
	}
	if pn, ok := result.Metadata["project_name"]; ok {
		fmt.Printf("Metadata:   project=%v branch=%v\n", pn, result.Metadata["branch"])
	}
	if len(result.Files) > 0 {
		fmt.Println()
		fmt.Println("Files:")
		for _, f := range result.Files {
			fmt.Printf("  %s\n", f)
		}
	}
	return nil
}

func exitCodeFor(err error) int {
	var ce *clierr.Error
	if errors.As(err, &ce) {
		return ce.ExitCode()
	}
	return clierr.ExitUser
}
