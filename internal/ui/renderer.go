package ui

import (
	"coding-contest-client-cli/internal/model"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
)

type TerminalRenderer struct{}

func NewTerminalRenderer() *TerminalRenderer {
	return &TerminalRenderer{}
}

func (r *TerminalRenderer) Render(snapshot model.Snapshot, deltas []model.RowDelta, topN int) {
	clearScreen()

	title := color.New(color.FgCyan, color.Bold).Sprint("Coding Contest Results Watcher")
	fmt.Printf("%s\n", title)
	fmt.Printf("Updated: %s | Submissions: %d\n\n",
		snapshot.GeneratedAt.Format(time.RFC3339),
		snapshot.SubmissionTotal,
	)

	rows := snapshot.Rows
	if topN > 0 && topN < len(rows) {
		rows = rows[:topN]
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Rank", "Team", "Total Score", "Submissions", "Last Status", "Last Updated"})

	deltaMap := map[string]model.RowDelta{}
	for _, d := range deltas {
		deltaMap[d.TeamID] = d
	}

	for _, row := range rows {
		teamLabel := row.TeamName

		if d, ok := deltaMap[row.TeamID]; ok {
			teamLabel = fmt.Sprintf("%s (%s)", row.TeamName, d.Message)
		}

		t.AppendRow(table.Row{
			row.Rank,
			teamLabel,
			fmt.Sprintf("%.2f", row.TotalScore),
			row.SubmissionCount,
			row.LastStatus,
			formatTime(row.LastSubmissionAt),
		})
	}

	t.Render()

	if len(deltas) > 0 {
		fmt.Println()
		fmt.Println(color.New(color.FgYellow, color.Bold).Sprint("Recent changes:"))
		for _, d := range deltas {
			fmt.Printf("- %s: rank %d -> %d, score %.2f -> %.2f (%s)\n",
				shortTeamID(d.TeamID),
				d.OldRank, d.NewRank,
				d.OldScore, d.NewScore,
				d.Message,
			)
		}
	}

	fmt.Println()
	fmt.Println("Press Ctrl+C to stop.")
}

func (r *TerminalRenderer) RenderError(err error) {
	fmt.Println()
	fmt.Println(color.New(color.FgRed, color.Bold).Sprint("Polling error:"))
	fmt.Printf("- %s\n\n", err.Error())
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func shortTeamID(v string) string {
	if len(v) <= 8 {
		return v
	}
	return v[:8]
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return strings.ReplaceAll(t.Format(time.RFC3339), "T", " ")
}
