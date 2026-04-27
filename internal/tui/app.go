package tui

import (
	"coding-contest-client-cli/internal/api"
	"coding-contest-client-cli/internal/config"
	"coding-contest-client-cli/internal/leaderboard"
	"coding-contest-client-cli/internal/model"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen string

const (
	screenContestList screen = "contest-list"
	screenContestInfo screen = "contest-info"
	screenLiveBoard   screen = "live-board"
)

type contestsLoadedMsg struct {
	Contests []model.Contest
	Err      error
}

type leaderboardLoadedMsg struct {
	Snapshot model.Snapshot
	Deltas   []model.RowDelta
	Err      error
}

type appModel struct {
	cfg    config.Config
	client *api.Client

	currentScreen screen
	contests      []model.Contest
	cursor        int
	selected      *model.Contest

	loading   bool
	errorText string

	watching bool
	prevRows []model.LeaderboardRow
	snapshot model.Snapshot
	deltas   []model.RowDelta

	width  int
	height int

	titleStyle   lipgloss.Style
	headerStyle  lipgloss.Style
	activeStyle  lipgloss.Style
	mutedStyle   lipgloss.Style
	errorStyle   lipgloss.Style
	borderStyle  lipgloss.Style
	hintStyle    lipgloss.Style
	successStyle lipgloss.Style
}

func NewApp(cfg config.Config, client *api.Client) tea.Model {
	return appModel{
		cfg:           cfg,
		client:        client,
		currentScreen: screenContestList,
		loading:       true,
		watching:      false,
		titleStyle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")),
		headerStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117")),
		activeStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("62")).Padding(0, 1),
		mutedStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		errorStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		borderStyle:   lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("63")).Padding(1, 2),
		hintStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("110")),
		successStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
	}
}

func (m appModel) Init() tea.Cmd {
	return m.loadContestsCmd()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		return m, nil

	case contestsLoadedMsg:
		m.loading = false
		if typed.Err != nil {
			m.errorText = typed.Err.Error()
			m.contests = nil
			m.cursor = 0
			return m, nil
		}
		m.errorText = ""
		m.contests = typed.Contests
		if len(m.contests) == 0 {
			m.cursor = 0
		} else if m.cursor >= len(m.contests) {
			m.cursor = len(m.contests) - 1
		}
		return m, nil

	case leaderboardLoadedMsg:
		m.loading = false
		if typed.Err != nil {
			m.errorText = typed.Err.Error()
			return m, nil
		}
		m.errorText = ""
		m.snapshot = typed.Snapshot
		m.deltas = typed.Deltas
		m.prevRows = typed.Snapshot.Rows
		if m.currentScreen == screenLiveBoard && m.watching {
			return m, m.tickCmd()
		}
		return m, nil

	case time.Time:
		if m.currentScreen == screenLiveBoard && m.watching && m.selected != nil {
			m.loading = true
			return m, m.loadLeaderboardCmd(m.selected.ID, m.prevRows)
		}
		return m, nil

	case tea.KeyMsg:
		switch typed.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

		switch m.currentScreen {
		case screenContestList:
			return m.updateContestListKeys(typed)
		case screenContestInfo:
			return m.updateContestInfoKeys(typed)
		case screenLiveBoard:
			return m.updateLiveBoardKeys(typed)
		}
	}

	return m, nil
}

func (m appModel) updateContestListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if len(m.contests) > 0 && m.cursor < len(m.contests)-1 {
			m.cursor++
		}
		return m, nil
	case "k", "up":
		if len(m.contests) > 0 && m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "enter":
		if len(m.contests) == 0 {
			return m, nil
		}
		selected := m.contests[m.cursor]
		m.selected = &selected
		m.currentScreen = screenContestInfo
		m.errorText = ""
		return m, nil
	case "r":
		m.loading = true
		m.errorText = ""
		return m, m.loadContestsCmd()
	default:
		return m, nil
	}
}

func (m appModel) updateContestInfoKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "b", "esc":
		m.currentScreen = screenContestList
		m.selected = nil
		m.errorText = ""
		return m, nil
	case "enter", "w":
		if m.selected == nil {
			return m, nil
		}
		m.currentScreen = screenLiveBoard
		m.watching = true
		m.loading = true
		m.errorText = ""
		m.prevRows = nil
		return m, m.loadLeaderboardCmd(m.selected.ID, nil)
	default:
		return m, nil
	}
}

func (m appModel) updateLiveBoardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "b", "esc":
		m.currentScreen = screenContestInfo
		m.watching = false
		m.loading = false
		m.errorText = ""
		return m, nil
	case " ":
		m.watching = !m.watching
		if m.watching {
			return m, m.tickCmd()
		}
		return m, nil
	case "r":
		if m.selected == nil {
			return m, nil
		}
		m.loading = true
		return m, m.loadLeaderboardCmd(m.selected.ID, m.prevRows)
	default:
		return m, nil
	}
}

func (m appModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading terminal..."
	}

	var body string
	switch m.currentScreen {
	case screenContestList:
		body = m.renderContestList()
	case screenContestInfo:
		body = m.renderContestInfo()
	case screenLiveBoard:
		body = m.renderLiveBoard()
	default:
		body = "Unknown screen"
	}

	return body
}

func (m appModel) renderContestList() string {
	lines := []string{
		m.titleStyle.Render("Coding Contest Watcher"),
		m.hintStyle.Render("Browse contests with j/k or arrows. Enter = details, r = refresh, q = quit"),
		"",
	}

	if m.loading {
		lines = append(lines, m.mutedStyle.Render("Loading contests..."))
		return m.borderStyle.Render(strings.Join(lines, "\n"))
	}

	if m.errorText != "" {
		lines = append(lines, m.errorStyle.Render("Error: "+m.errorText))
		lines = append(lines, "")
	}

	if len(m.contests) == 0 {
		lines = append(lines, m.mutedStyle.Render("No contests available."))
		return m.borderStyle.Render(strings.Join(lines, "\n"))
	}

	for i, contest := range m.contests {
		prefix := "  "
		label := fmt.Sprintf("%s  (%s)", contest.Name, contest.ID)
		if i == m.cursor {
			prefix = "▶ "
			label = m.activeStyle.Render(label)
		}
		lines = append(lines, prefix+label)
	}

	return m.borderStyle.Render(strings.Join(lines, "\n"))
}

func (m appModel) renderContestInfo() string {
	if m.selected == nil {
		return m.borderStyle.Render(m.errorStyle.Render("No contest selected."))
	}

	contest := m.selected
	description := "-"
	if contest.Description != nil && *contest.Description != "" {
		description = *contest.Description
	}

	reward := "-"
	if contest.Reward != nil && *contest.Reward != "" {
		reward = *contest.Reward
	}

	maxTeams := "unlimited"
	if contest.MaxTeams != nil {
		maxTeams = fmt.Sprintf("%d", *contest.MaxTeams)
	}

	status := m.mutedStyle.Render("inactive")
	if contest.IsActive {
		status = m.successStyle.Render("active")
	}

	lines := []string{
		m.titleStyle.Render("Contest Details"),
		m.hintStyle.Render("Enter/w = live leaderboard, b/esc = back, q = quit"),
		"",
		m.headerStyle.Render(contest.Name),
		fmt.Sprintf("ID: %s", contest.ID),
		fmt.Sprintf("Status: %s", status),
		fmt.Sprintf("Description: %s", description),
		fmt.Sprintf("Reward: %s", reward),
		fmt.Sprintf("Max Team Size: %d", contest.MaxTeamSize),
		fmt.Sprintf("Max Teams: %s", maxTeams),
		fmt.Sprintf("Start: %s", contest.StartDate.Format(time.RFC3339)),
		fmt.Sprintf("End: %s", contest.EndDate.Format(time.RFC3339)),
		fmt.Sprintf("Submission Deadline: %s", contest.SubmissionDeadline.Format(time.RFC3339)),
	}

	if m.errorText != "" {
		lines = append(lines, "", m.errorStyle.Render("Error: "+m.errorText))
	}

	return m.borderStyle.Render(strings.Join(lines, "\n"))
}

func (m appModel) renderLiveBoard() string {
	if m.selected == nil {
		return m.borderStyle.Render(m.errorStyle.Render("No contest selected."))
	}

	watchState := m.mutedStyle.Render("paused")
	if m.watching {
		watchState = m.successStyle.Render("live")
	}

	lines := []string{
		m.titleStyle.Render("Live Leaderboard"),
		m.hintStyle.Render("j/k not needed here. space = pause/resume, r = refresh, b/esc = back, q = quit"),
		"",
		fmt.Sprintf("Contest: %s", m.selected.Name),
		fmt.Sprintf("Mode: %s (interval %s)", watchState, m.cfg.Interval.String()),
	}

	if m.loading {
		lines = append(lines, "", m.mutedStyle.Render("Refreshing leaderboard..."))
	}

	if m.errorText != "" {
		lines = append(lines, "", m.errorStyle.Render("Error: "+m.errorText))
	}

	if !m.snapshot.GeneratedAt.IsZero() {
		lines = append(lines, "", fmt.Sprintf("Updated: %s", m.snapshot.GeneratedAt.Format(time.RFC3339)))
		lines = append(lines, fmt.Sprintf("Submissions: %d", m.snapshot.SubmissionTotal))
	}

	lines = append(lines, "", m.headerStyle.Render("Rank  Team                  Score   Subs   Last Status"))
	if len(m.snapshot.Rows) == 0 {
		lines = append(lines, m.mutedStyle.Render("No leaderboard rows yet."))
	} else {
		rows := m.snapshot.Rows
		if m.cfg.TopN > 0 && len(rows) > m.cfg.TopN {
			rows = rows[:m.cfg.TopN]
		}
		for _, row := range rows {
			teamName := trimLen(row.TeamName, 20)
			lines = append(lines,
				fmt.Sprintf("%-5d %-20s %-7.2f %-6d %s", row.Rank, teamName, row.TotalScore, row.SubmissionCount, row.LastStatus),
			)
		}
	}

	if len(m.deltas) > 0 {
		lines = append(lines, "", m.headerStyle.Render("Recent changes"))
		for _, d := range m.deltas {
			lines = append(lines,
				fmt.Sprintf("- %s | rank %d -> %d | %.2f -> %.2f | %s", shortID(d.TeamID), d.OldRank, d.NewRank, d.OldScore, d.NewScore, d.Message),
			)
		}
	}

	return m.borderStyle.Render(strings.Join(lines, "\n"))
}

func (m appModel) loadContestsCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		contests, err := client.GetContests()
		return contestsLoadedMsg{Contests: contests, Err: err}
	}
}

func (m appModel) loadLeaderboardCmd(contestID string, prevRows []model.LeaderboardRow) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		snapshot, deltas, err := collectSnapshot(client, contestID, prevRows)
		return leaderboardLoadedMsg{Snapshot: snapshot, Deltas: deltas, Err: err}
	}
}

func (m appModel) tickCmd() tea.Cmd {
	return tea.Tick(m.cfg.Interval, func(t time.Time) tea.Msg {
		return t
	})
}

func collectSnapshot(client *api.Client, contestID string, prevRows []model.LeaderboardRow) (model.Snapshot, []model.RowDelta, error) {
	submissions, err := client.GetSubmissions(contestID)
	if err != nil {
		return model.Snapshot{}, nil, fmt.Errorf("load submissions failed: %w", err)
	}

	teamNames := map[string]string{}
	scoresBySubmission := map[string][]model.Score{}
	seenTeam := map[string]bool{}

	for _, submission := range submissions {
		if !seenTeam[submission.TeamID] {
			team, teamErr := client.GetTeam(submission.TeamID)
			if teamErr == nil && team.Name != "" {
				teamNames[submission.TeamID] = team.Name
			}
			seenTeam[submission.TeamID] = true
		}

		scores, scoreErr := client.GetScores(submission.ID)
		if scoreErr != nil {
			return model.Snapshot{}, nil, fmt.Errorf("load scores for %s failed: %w", submission.ID, scoreErr)
		}
		scoresBySubmission[submission.ID] = scores
	}

	rows := leaderboard.Build(submissions, scoresBySubmission, teamNames)
	snapshot := model.Snapshot{
		Rows:            rows,
		GeneratedAt:     time.Now(),
		SubmissionTotal: len(submissions),
	}

	if len(prevRows) == 0 {
		return snapshot, nil, nil
	}
	return snapshot, leaderboard.Diff(prevRows, rows), nil
}

func trimLen(value string, max int) string {
	if len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func shortID(value string) string {
	if len(value) <= 8 {
		return value
	}
	return value[:8]
}
