package compatibility

import (
	"fmt"
	"strings"
)

const (
	MatrixGeneratedStart = "<!-- agentic-coding-result:start -->"
	MatrixGeneratedEnd   = "<!-- agentic-coding-result:end -->"
)

func RenderArtifactMarkdown(artifact Artifact) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Latest command:\n\n```sh\n%s\n```\n\n", artifact.Command)
	fmt.Fprintf(&b, "Total duration: %.3f seconds.\n\n", artifact.TotalDurationSeconds)
	b.WriteString("| Candidate | Provider endpoint | Native model | Required checks | Cache accounting | Status | Duration |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, row := range artifact.Rows {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s | %s | %s | %.2fs |\n",
			row.Candidate,
			row.Provider,
			row.NativeModel,
			requiredDisplay(row.RequiredStatus),
			statusDisplay(row.CacheAccounting),
			row.Status,
			row.DurationSeconds,
		)
	}
	return b.String()
}

func ReplaceGeneratedSection(markdown string, generated string) (string, error) {
	start := strings.Index(markdown, MatrixGeneratedStart)
	end := strings.Index(markdown, MatrixGeneratedEnd)
	if start < 0 || end < 0 || end < start {
		return "", fmt.Errorf("generated section markers not found")
	}
	var b strings.Builder
	b.WriteString(markdown[:start+len(MatrixGeneratedStart)])
	b.WriteString("\n\n")
	b.WriteString(strings.TrimSpace(generated))
	b.WriteString("\n\n")
	b.WriteString(markdown[end:])
	return b.String(), nil
}

func requiredDisplay(value string) string {
	if value == "" {
		return "unknown"
	}
	if value == "passed" {
		return "pass"
	}
	return value
}

func statusDisplay(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
