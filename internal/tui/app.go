package tui

import (
	"coding-contest-client-cli/internal/api"
	"coding-contest-client-cli/internal/auth"
	"coding-contest-client-cli/internal/config"
	"coding-contest-client-cli/internal/leaderboard"
	"coding-contest-client-cli/internal/model"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen string

const (
	screenLogin            screen = "login"
	screenMainMenu         screen = "main-menu"
	screenContestList      screen = "contest-list"
	screenContestInfo      screen = "contest-info"
	screenLiveBoard        screen = "live-board"
	screenSubmissionList   screen = "submission-list"
	screenSubmissionDetail screen = "submission-detail"
	screenPATManagement    screen = "pat-management"
)

// Message types
type contestsLoadedMsg struct {
	Contests    []model.Contest
	Page        int
	TotalItems  int
	HasNextPage bool
	Err         error
}

type leaderboardLoadedMsg struct {
	Snapshot model.Snapshot
	Deltas   []model.RowDelta
	Err      error
}

type submissionsLoadedMsg struct {
	Submissions []model.Submission
	Err         error
}

type submissionDetailLoadedMsg struct {
	Detail *model.SubmissionDetail
	Err    error
}

type patsLoadedMsg struct {
	PATs []auth.PAT
	Err  error
}

type patCreatedMsg struct {
	Token string
	Err   error
}

type loginSuccessMsg struct {
	Token    string
	Username string
}

type loginFailedMsg struct {
	Err error
}

type patsDeletedMsg struct {
	Err error
}

// App model holds all state
type appModel struct {
	// Config
	cfg    config.Config
	client *api.Client

	// Session
	sessionUsername string
	sessionToken    string

	// UI state
	currentScreen screen
	width         int
	height        int

	// Navigation
	menuCursor int

	// Contest list state
	contests         []model.Contest
	filteredContests []model.Contest
	contestCursor    int
	// pagination
	contestsPage    int
	contestsLimit   int
	contestsTotal   int
	contestsHasNext bool

	// Selected items
	selected     *model.Contest
	selectedTeam *model.LeaderboardRow
	selectedPAT  *auth.PAT

	// Leaderboard state
	loading           bool
	errorText         string
	watching          bool
	leaderboardCursor int
	prevRows          []model.LeaderboardRow
	snapshot          model.Snapshot
	deltas            []model.RowDelta

	// Submissions state
	submissions      []model.Submission
	submissionCursor int
	submissionDetail *model.SubmissionDetail

	// PAT state
	pats          []auth.PAT
	patCursor     int
	createPATMode bool
	patName       string
	patExpDays    string

	// Login state
	loginMode string // "", "username", "password"
	loginUser string
	loginPass string

	// Styles
	titleStyle   lipgloss.Style
	headerStyle  lipgloss.Style
	activeStyle  lipgloss.Style
	mutedStyle   lipgloss.Style
	errorStyle   lipgloss.Style
	borderStyle  lipgloss.Style
	hintStyle    lipgloss.Style
	successStyle lipgloss.Style
	boxStyle     lipgloss.Style
}

func NewApp(cfg config.Config, client *api.Client) tea.Model {
	app := appModel{
		cfg:    cfg,
		client: client,

		currentScreen: screenLogin,
		loading:       true,
		watching:      false,
		loginMode:     "username",
		contestsPage:  1,
		contestsLimit: 10,

		titleStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")),
		headerStyle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117")),
		activeStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("62")).Padding(0, 1),
		mutedStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		errorStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		borderStyle:  lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("63")).Padding(1, 2),
		hintStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("110")),
		successStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		boxStyle:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(1, 2),
	}

	// Try to load existing session
	if session, err := auth.LoadSession(); err == nil && session != nil {
		app.sessionToken = session.Token
		app.sessionUsername = session.Username
		cfg.Token = session.Token
		app.cfg = cfg
		// Update client with token
		app.client = api.New(cfg.BaseURL, cfg.APIPrefix, session.Token, cfg.Timeout)
		app.currentScreen = screenMainMenu
	}

	return app
}

func (m appModel) Init() tea.Cmd {
	if m.currentScreen == screenLogin {
		return nil
	}
	return m.loadContestsCmd()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		return m, nil

	case tea.KeyMsg:
		if typed.String() == "q" || typed.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.currentScreen {
		case screenLogin:
			return m.updateLogin(typed)
		case screenMainMenu:
			return m.updateMainMenu(typed)
		case screenContestList:
			return m.updateContestList(typed)
		case screenContestInfo:
			return m.updateContestInfo(typed)
		case screenLiveBoard:
			return m.updateLiveBoard(typed)
		case screenSubmissionList:
			return m.updateSubmissionList(typed)
		case screenSubmissionDetail:
			return m.updateSubmissionDetail(typed)
		case screenPATManagement:
			return m.updatePATManagement(typed)
		}

	case contestsLoadedMsg:
		m.loading = false
		if typed.Err != nil {
			m.errorText = typed.Err.Error()
			m.contests = nil
			m.contestCursor = 0
		} else {
			m.contests = typed.Contests
			m.filteredContests = typed.Contests
			m.errorText = ""
			m.contestCursor = 0
			m.contestsPage = typed.Page
			m.contestsTotal = typed.TotalItems
			m.contestsHasNext = typed.HasNextPage
		}
		return m, nil

	case leaderboardLoadedMsg:
		m.loading = false
		if typed.Err != nil {
			m.errorText = typed.Err.Error()
		} else {
			m.snapshot = typed.Snapshot
			m.deltas = typed.Deltas
			m.prevRows = typed.Snapshot.Rows
			displayLimit := len(m.snapshot.Rows)
			if m.cfg.TopN > 0 && m.cfg.TopN < displayLimit {
				displayLimit = m.cfg.TopN
			}
			if len(m.snapshot.Rows) > 0 {
				matched := false
				if m.selectedTeam != nil {
					for i := range m.snapshot.Rows {
						if i < displayLimit && m.snapshot.Rows[i].TeamID == m.selectedTeam.TeamID {
							m.leaderboardCursor = i
							m.selectedTeam = &m.snapshot.Rows[i]
							matched = true
							break
						}
					}
				}
				if !matched {
					if m.leaderboardCursor >= displayLimit {
						m.leaderboardCursor = 0
					}
					m.selectedTeam = &m.snapshot.Rows[m.leaderboardCursor]
				}
			} else {
				m.leaderboardCursor = 0
				m.selectedTeam = nil
			}
			m.errorText = ""
		}
		// normal contest list key handling
		return m, nil

	case submissionsLoadedMsg:
		m.loading = false
		if typed.Err != nil {
			m.errorText = typed.Err.Error()
			m.submissions = nil
		} else {
			m.submissions = typed.Submissions
			m.submissionCursor = 0
			m.errorText = ""
		}
		return m, nil

	case submissionDetailLoadedMsg:
		m.loading = false
		if typed.Err != nil {
			m.errorText = typed.Err.Error()
			m.submissionDetail = nil
		} else {
			m.submissionDetail = typed.Detail
			m.errorText = ""
		}
		return m, nil

	case patsLoadedMsg:
		m.loading = false
		if typed.Err != nil {
			m.errorText = typed.Err.Error()
			m.pats = nil
		} else {
			m.pats = typed.PATs
			m.patCursor = 0
			m.errorText = ""
		}
		return m, nil

	case loginSuccessMsg:
		m.sessionToken = typed.Token
		m.sessionUsername = typed.Username
		m.cfg.Token = typed.Token
		m.client = api.New(m.cfg.BaseURL, m.cfg.APIPrefix, typed.Token, m.cfg.Timeout)
		m.loginUser = ""
		m.loginPass = ""
		m.currentScreen = screenMainMenu
		m.loading = false
		m.errorText = ""
		return m, m.loadContestsCmd()

	case loginFailedMsg:
		m.loading = false
		if typed.Err != nil {
			m.errorText = typed.Err.Error()
		} else {
			m.errorText = "login failed"
		}
		return m, nil

	case patCreatedMsg:
		m.loading = false
		if typed.Err != nil {
			m.errorText = typed.Err.Error()
			return m, nil
		}
		m.errorText = "PAT created and copied to clipboard"
		// reload PATs to show the new token entry (token itself is only returned at creation)
		return m, m.loadPATsCmd()

	case patsDeletedMsg:
		m.loading = false
		if typed.Err != nil {
			m.errorText = typed.Err.Error()
			return m, nil
		}
		m.errorText = "PAT deleted"
		// reload list to reflect deletion
		return m, m.loadPATsCmd()

	case time.Time:
		if m.currentScreen == screenLiveBoard && m.watching {
			return m, m.loadLeaderboardCmd(m.selected.ID, m.prevRows)
		}
	}

	return m, nil
}

func (m appModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading terminal...\n"
	}

	var content string
	switch m.currentScreen {
	case screenLogin:
		content = m.renderLogin()
	case screenMainMenu:
		content = m.renderMainMenu()
	case screenContestList:
		content = m.renderContestList()
	case screenContestInfo:
		content = m.renderContestInfo()
	case screenLiveBoard:
		content = m.renderLiveBoard()
	case screenSubmissionList:
		content = m.renderSubmissionList()
	case screenSubmissionDetail:
		content = m.renderSubmissionDetail()
	case screenPATManagement:
		content = m.renderPATManagement()
	default:
		content = "Unknown screen"
	}

	return content
}

// ============ Login Screen ============

func (m appModel) updateLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		if m.loginMode == "username" {
			m.loginMode = "password"
		} else {
			m.loginMode = "username"
		}
		return m, nil

	case "enter":
		if m.loginMode == "username" && m.loginUser != "" {
			m.loginMode = "password"
			return m, nil
		}
		if m.loginMode == "password" && m.loginPass != "" && m.loginUser != "" {
			m.loading = true
			m.errorText = ""
			return m, m.loginCmd(m.loginUser, m.loginPass)
		}
		return m, nil

	case "backspace":
		if m.loginMode == "username" && len(m.loginUser) > 0 {
			m.loginUser = m.loginUser[:len(m.loginUser)-1]
		} else if m.loginMode == "password" && len(m.loginPass) > 0 {
			m.loginPass = m.loginPass[:len(m.loginPass)-1]
		}
		return m, nil

	default:
		if len(msg.String()) == 1 && msg.String() >= " " && msg.String() < "\x7f" {
			if m.loginMode == "username" {
				m.loginUser += msg.String()
			} else if m.loginMode == "password" {
				m.loginPass += msg.String()
			}
		}
		return m, nil
	}
}

func (m appModel) renderLogin() string {
	lines := []string{
		"",
		m.titleStyle.Render("Coding Contest Watcher"),
		"",
		m.headerStyle.Render("Login to your account"),
		"",
	}

	// Username input
	userDisplay := m.loginUser
	if m.loginMode == "username" {
		userDisplay = m.activeStyle.Render(userDisplay + " ")
	}
	lines = append(lines, fmt.Sprintf("Username: %s", userDisplay))

	// Password input
	passDisplay := strings.Repeat("•", len(m.loginPass))
	if m.loginMode == "password" {
		passDisplay = m.activeStyle.Render(passDisplay + " ")
	}
	lines = append(lines, fmt.Sprintf("Password: %s", passDisplay))

	lines = append(lines, "")
	lines = append(lines, m.hintStyle.Render("Tab = switch field, Enter = submit"))

	if m.loading {
		lines = append(lines, "", m.mutedStyle.Render("Logging in..."))
	}

	if m.errorText != "" {
		lines = append(lines, "", m.errorStyle.Render("Error: "+m.errorText))
	}

	return m.centerContent(strings.Join(lines, "\n"), m.width, m.height)
}

// ============ Main Menu Screen ============

func (m appModel) updateMainMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.menuCursor < 3 {
			m.menuCursor++
		}
		return m, nil
	case "k", "up":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
		return m, nil
	case "enter":
		switch m.menuCursor {
		case 0:
			m.currentScreen = screenContestList
			m.loading = true
			return m, m.loadContestsCmd()
		case 1:
			m.currentScreen = screenPATManagement
			m.loading = true
			return m, m.loadPATsCmd()
		case 2:
			m.errorText = ""
			m.sessionToken = ""
			m.sessionUsername = ""
			auth.ClearSession()
			m.currentScreen = screenLogin
			m.loginUser = ""
			m.loginPass = ""
			m.loginMode = "username"
			return m, nil
		case 3:
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m appModel) renderMainMenu() string {
	lines := []string{
		m.titleStyle.Render("Main Menu"),
		"",
		fmt.Sprintf("Welcome, %s!", m.sessionUsername),
		"",
	}

	menuItems := []string{
		"Browse Contests",
		"Manage PATs",
		"Logout",
		"Quit",
	}

	for i, item := range menuItems {
		prefix := "  "
		display := item
		if i == m.menuCursor {
			prefix = "▶ "
			display = m.activeStyle.Render(item)
		}
		lines = append(lines, prefix+display)
	}

	lines = append(lines, "")
	lines = append(lines, m.hintStyle.Render("j/k = navigate, Enter = select, q = quit"))

	return m.centerContent(strings.Join(lines, "\n"), m.width, m.height)
}

// ============ Contest List Keys ============

func (m appModel) updateContestList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if len(m.filteredContests) > 0 && m.contestCursor < len(m.filteredContests)-1 {
			m.contestCursor++
		}
		return m, nil

	case "k", "up":
		if m.contestCursor > 0 {
			m.contestCursor--
		}
		return m, nil

	case "enter":
		if len(m.filteredContests) == 0 {
			return m, nil
		}
		selected := m.filteredContests[m.contestCursor]
		m.selected = &selected
		m.currentScreen = screenContestInfo
		m.errorText = ""
		return m, nil

	case "r":
		m.loading = true
		m.errorText = ""
		return m, m.loadContestsCmd()

	case "b", "esc":
		m.currentScreen = screenMainMenu
		return m, nil

	case "n":
		if m.contestsHasNext {
			m.contestsPage++
			m.loading = true
			m.errorText = ""
			return m, m.loadContestsCmd()
		}
		return m, nil

	case "p":
		if m.contestsPage > 1 {
			m.contestsPage--
			m.loading = true
			m.errorText = ""
			return m, m.loadContestsCmd()
		}
		return m, nil
	}

	return m, nil
}

func (m appModel) renderContestList() string {
	contentHeight := m.height - 6
	if contentHeight < 3 {
		contentHeight = 3
	}

	lines := []string{
		m.titleStyle.Render("Contests"),
	}

	// Pagination bar
	paginationBar := fmt.Sprintf("Page: %d | Limit: %d | ", m.contestsPage, m.contestsLimit)
	if m.contestsTotal > 0 {
		paginationBar = fmt.Sprintf("%s Total: %d | ", paginationBar, m.contestsTotal)
	}
	paginationBar = paginationBar + "Press 'n'/'p' to navigate pages"
	lines = append(lines, m.mutedStyle.Render(paginationBar))

	lines = append(lines, "")

	if m.loading {
		lines = append(lines, m.mutedStyle.Render("Loading contests..."))
		return m.formatContent(strings.Join(lines, "\n"), m.width, contentHeight)
	}

	if m.errorText != "" {
		lines = append(lines, m.errorStyle.Render("Error: "+m.errorText))
		lines = append(lines, "")
	}

	if len(m.filteredContests) == 0 {
		lines = append(lines, m.mutedStyle.Render("No contests available."))
		return m.formatContent(strings.Join(lines, "\n"), m.width, contentHeight)
	}

	// Show contests with limited height
	shownCount := len(m.filteredContests)
	if shownCount > contentHeight-3 {
		shownCount = contentHeight - 3
	}

	startIdx := m.contestCursor - (shownCount / 2)
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > len(m.filteredContests)-shownCount {
		startIdx = len(m.filteredContests) - shownCount
		if startIdx < 0 {
			startIdx = 0
		}
	}

	for i := startIdx; i < startIdx+shownCount && i < len(m.filteredContests); i++ {
		contest := m.filteredContests[i]
		prefix := "  "
		status := "●"
		if !contest.IsActive {
			status = m.mutedStyle.Render("●")
		}

		label := fmt.Sprintf("%s %s", status, contest.Name)
		if i == m.contestCursor {
			prefix = "▶ "
			label = m.activeStyle.Render(label)
		}
		lines = append(lines, prefix+label)
	}

	lines = append(lines, "")
	lines = append(lines, m.hintStyle.Render("j/k=nav, Enter=view, r=refresh, n/p=page, b=back, q=quit"))

	return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
}

// ============ Contest Info Screen ============

func (m appModel) updateContestInfo(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	}
	return m, nil
}

func (m appModel) renderContestInfo() string {
	if m.selected == nil {
		return m.errorStyle.Render("No contest selected.")
	}

	contest := m.selected
	status := m.mutedStyle.Render("inactive")
	if contest.IsActive {
		status = m.successStyle.Render("active")
	}

	lines := []string{
		m.titleStyle.Render("Contest Details"),
		"",
		m.headerStyle.Render(contest.Name),
		fmt.Sprintf("ID: %s", shortID(contest.ID)),
		fmt.Sprintf("Status: %s", status),
	}

	if contest.Description != nil && *contest.Description != "" {
		lines = append(lines, fmt.Sprintf("Description: %s", *contest.Description))
	}

	if contest.Reward != nil && *contest.Reward != "" {
		lines = append(lines, fmt.Sprintf("Reward: %s", *contest.Reward))
	}

	lines = append(lines,
		fmt.Sprintf("Max Team Size: %d", contest.MaxTeamSize),
	)

	if contest.MaxTeams != nil {
		lines = append(lines, fmt.Sprintf("Max Teams: %d", *contest.MaxTeams))
	}

	lines = append(lines,
		fmt.Sprintf("Start: %s", contest.StartDate.Format("2006-01-02 15:04")),
		fmt.Sprintf("End: %s", contest.EndDate.Format("2006-01-02 15:04")),
	)

	if m.errorText != "" {
		lines = append(lines, "", m.errorStyle.Render("Error: "+m.errorText))
	}

	lines = append(lines, "")
	lines = append(lines, m.hintStyle.Render("Enter/w = watch live, b/esc = back, q = quit"))

	return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
}

// ============ Live Leaderboard Screen ============

func (m appModel) updateLiveBoard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "b", "esc":
		m.currentScreen = screenContestInfo
		m.watching = false
		m.leaderboardCursor = 0
		m.errorText = ""
		return m, nil
	case "j", "down":
		maxIdx := len(m.snapshot.Rows) - 1
		if m.cfg.TopN > 0 && m.cfg.TopN-1 < maxIdx {
			maxIdx = m.cfg.TopN - 1
		}
		if len(m.snapshot.Rows) > 0 && m.leaderboardCursor < maxIdx {
			m.leaderboardCursor++
			m.selectedTeam = &m.snapshot.Rows[m.leaderboardCursor]
		}
		return m, nil
	case "k", "up":
		if len(m.snapshot.Rows) > 0 && m.leaderboardCursor > 0 {
			m.leaderboardCursor--
			m.selectedTeam = &m.snapshot.Rows[m.leaderboardCursor]
		}
		return m, nil
	case "space":
		m.watching = !m.watching
		if m.watching {
			return m, m.loadLeaderboardCmd(m.selected.ID, m.prevRows)
		}
		return m, nil
	case "r":
		m.errorText = ""
		return m, m.loadLeaderboardCmd(m.selected.ID, m.prevRows)
	case "enter", "d":
		if len(m.snapshot.Rows) > 0 {
			m.selectedTeam = &m.snapshot.Rows[m.leaderboardCursor]
			m.currentScreen = screenSubmissionList
			m.loading = true
			m.submissionCursor = 0
			return m, m.loadSubmissionsCmd(m.selected.ID, m.selectedTeam.TeamID)
		}
		return m, nil
	}
	return m, nil
}

func (m appModel) renderLiveBoard() string {
	if m.selected == nil {
		return m.errorStyle.Render("No contest selected.")
	}

	contentHeight := m.height - 8

	watchState := m.mutedStyle.Render("paused")
	if m.watching {
		watchState = m.successStyle.Render("live")
	}

	lines := []string{
		m.titleStyle.Render("Live Leaderboard"),
		fmt.Sprintf("Contest: %s", m.selected.Name),
		fmt.Sprintf("Mode: %s (interval %s)", watchState, m.cfg.Interval.String()),
	}

	if m.loading {
		lines = append(lines, "", m.mutedStyle.Render("Loading leaderboard..."))
		return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
	}

	if m.errorText != "" {
		lines = append(lines, "", m.errorStyle.Render("Error: "+m.errorText))
		lines = append(lines, "")
	}

	lines = append(lines, "")

	// Show deltas if any
	if len(m.deltas) > 0 {
		lines = append(lines, m.successStyle.Render("✨ Recent changes:"))
		for _, delta := range m.deltas {
			if delta.NewRank > 0 && delta.OldRank > 0 {
				rankChange := fmt.Sprintf("%d→%d", delta.OldRank, delta.NewRank)
				lines = append(lines, fmt.Sprintf("  %s: %s (%+.1f pts)", delta.TeamID, rankChange, delta.NewScore-delta.OldScore))
			}
		}
		lines = append(lines, "")
	}

	// Leaderboard
	if len(m.snapshot.Rows) > 0 {
		lines = append(lines, m.headerStyle.Render(fmt.Sprintf("%-4s %-30s %10s %s", "Rank", "Team", "Score", "Status")))
		lines = append(lines, m.mutedStyle.Render(strings.Repeat("─", 60)))

		limit := m.cfg.TopN
		if limit == 0 || limit > len(m.snapshot.Rows) {
			limit = len(m.snapshot.Rows)
		}

		shown := 0
		for i := 0; i < limit && shown < contentHeight-5; i++ {
			row := m.snapshot.Rows[i]
			marker := "  "
			if i == m.leaderboardCursor {
				marker = "▶ "
			}
			line := fmt.Sprintf("%-4d %-30s %10.1f %s",
				i+1,
				trimLen(row.TeamName, 28),
				row.TotalScore,
				row.LastStatus,
			)
			if i == m.leaderboardCursor {
				line = m.activeStyle.Render(line)
			}
			lines = append(lines, marker+line)
			shown++
		}
	} else {
		lines = append(lines, m.mutedStyle.Render("No submissions yet."))
	}

	lines = append(lines, "")
	lines = append(lines, m.hintStyle.Render("j/k=select team, enter/d=drill-down, r=refresh, b=back, q=quit"))

	return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
}

// ============ Submission Management Screens ============

func (m appModel) updateSubmissionList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "b", "esc":
		m.currentScreen = screenLiveBoard
		m.selectedTeam = nil
		m.errorText = ""
		return m, nil

	case "j", "down":
		if len(m.submissions) > 0 && m.submissionCursor < len(m.submissions)-1 {
			m.submissionCursor++
		}
		return m, nil

	case "k", "up":
		if m.submissionCursor > 0 {
			m.submissionCursor--
		}
		return m, nil

	case "enter":
		if len(m.submissions) == 0 {
			return m, nil
		}
		submission := m.submissions[m.submissionCursor]
		m.currentScreen = screenSubmissionDetail
		m.loading = true
		return m, m.loadSubmissionDetailCmd(m.selected.ID, submission.ID)
	}
	return m, nil
}

func (m appModel) renderSubmissionList() string {
	if m.selectedTeam == nil {
		return m.errorStyle.Render("No team selected.")
	}

	contentHeight := m.height - 7

	lines := []string{
		m.titleStyle.Render("Submissions"),
		fmt.Sprintf("Team: %s", m.selectedTeam.TeamName),
	}

	if m.loading {
		lines = append(lines, "", m.mutedStyle.Render("Loading submissions..."))
		return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
	}

	if m.errorText != "" {
		lines = append(lines, "", m.errorStyle.Render("Error: "+m.errorText))
	}

	if len(m.submissions) == 0 {
		lines = append(lines, "", m.mutedStyle.Render("No submissions."))
		return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
	}

	lines = append(lines, "")

	shown := 0
	for i := 0; i < len(m.submissions) && shown < contentHeight-3; i++ {
		sub := m.submissions[i]
		prefix := "  "
		label := fmt.Sprintf("%s (status: %s)", sub.Title, sub.Status)
		if i == m.submissionCursor {
			prefix = "▶ "
			label = m.activeStyle.Render(label)
		}
		lines = append(lines, prefix+label)
		shown++
	}

	lines = append(lines, "")
	lines = append(lines, m.hintStyle.Render("j/k=nav, Enter=details, b=back, q=quit"))

	return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
}

func (m appModel) updateSubmissionDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "b", "esc":
		m.currentScreen = screenSubmissionList
		m.submissionDetail = nil
		m.errorText = ""
		return m, nil
	}
	return m, nil
}

func (m appModel) renderSubmissionDetail() string {
	if m.submissionDetail == nil {
		return m.errorStyle.Render("No submission loaded.")
	}

	lines := []string{
		m.titleStyle.Render("Submission Details"),
		m.headerStyle.Render(m.submissionDetail.Title),
		fmt.Sprintf("ID: %s", shortID(m.submissionDetail.ID)),
		fmt.Sprintf("Status: %s", m.submissionDetail.Status),
		fmt.Sprintf("Total Score: %.1f", m.submissionDetail.TotalScore),
	}

	if m.submissionDetail.Description != nil {
		lines = append(lines, fmt.Sprintf("Description: %s", *m.submissionDetail.Description))
	}

	lines = append(lines,
		fmt.Sprintf("Submitted: %s", m.submissionDetail.SubmittedAt.Format("2006-01-02 15:04:05")),
	)

	if len(m.submissionDetail.Scores) > 0 {
		lines = append(lines, "", m.headerStyle.Render("Scores:"))
		for _, score := range m.submissionDetail.Scores {
			feedback := ""
			if score.Feedback != nil {
				feedback = fmt.Sprintf(" - %s", *score.Feedback)
			}
			lines = append(lines, fmt.Sprintf("  Judge: %.1f%s", score.Score, feedback))
		}
	}

	lines = append(lines, "")
	lines = append(lines, m.hintStyle.Render("b/esc=back, q=quit"))

	return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
}

// ============ PAT Management Screen ============

func (m appModel) updatePATManagement(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.createPATMode {
		switch msg.String() {
		case "esc":
			m.createPATMode = false
			m.patName = ""
			m.patExpDays = ""
			return m, nil

		case "tab":
			if m.patName != "" && m.patExpDays == "" {
				m.patExpDays = "0"
			}
			return m, nil

		case "enter":
			if m.patName != "" {
				m.createPATMode = false
				expDays := 0
				if m.patExpDays != "" {
					fmt.Sscanf(m.patExpDays, "%d", &expDays)
				}
				return m, m.createPATCmd(m.patName, expDays)
			}
			return m, nil

		case "backspace":
			if m.patExpDays != "" && len(m.patExpDays) > 0 {
				m.patExpDays = m.patExpDays[:len(m.patExpDays)-1]
			} else if m.patName != "" && m.patExpDays == "" {
				m.patName = m.patName[:len(m.patName)-1]
			}
			return m, nil

		default:
			if len(msg.String()) == 1 {
				if msg.String() >= "0" && msg.String() <= "9" && m.patExpDays != "" {
					m.patExpDays += msg.String()
				} else if m.patExpDays == "" && msg.String() >= " " && msg.String() < "\x7f" {
					m.patName += msg.String()
				}
			}
			return m, nil
		}
	}

	switch msg.String() {
	case "b", "esc":
		m.currentScreen = screenMainMenu
		m.errorText = ""
		return m, nil

	case "n":
		m.createPATMode = true
		m.patName = ""
		m.patExpDays = ""
		return m, nil

	case "d":
		if len(m.pats) > 0 && m.patCursor < len(m.pats) {
			pat := m.pats[m.patCursor]
			m.loading = true
			m.errorText = ""
			return m, m.deletePATCmd(pat.ID)
		}
		return m, nil

	case "j", "down":
		if len(m.pats) > 0 && m.patCursor < len(m.pats)-1 {
			m.patCursor++
		}
		return m, nil

	case "k", "up":
		if m.patCursor > 0 {
			m.patCursor--
		}
		return m, nil

	// case "c":
	// 	if len(m.pats) > 0 && m.patCursor < len(m.pats) {
	// 		token := m.pats[m.patCursor].Token
	// 		if token == "" {
	// 			m.errorText = "PAT has no token available"
	// 			return m, nil
	// 		}
	// 		if err := clipboard.WriteAll(token); err != nil {
	// 			// Fallback: instruct manual copy
	// 			m.errorText = fmt.Sprintf("Failed to copy to clipboard: %v — token: %s", err, token)
	// 			return m, nil
	// 		}
	// 		m.errorText = "PAT token copied to clipboard"
	// 	}
	// 	return m, nil

	case "r":
		m.loading = true
		m.errorText = ""
		return m, m.loadPATsCmd()
	}

	return m, nil
}

func (m appModel) renderPATManagement() string {
	contentHeight := m.height - 8

	lines := []string{
		m.titleStyle.Render("Personal Access Tokens"),
		"",
	}

	if m.createPATMode {
		lines = append(lines, m.headerStyle.Render("Create New PAT"),
			fmt.Sprintf("Name: %s", m.activeStyle.Render(m.patName+" ")),
			fmt.Sprintf("Expire in days (0=never): %s", m.activeStyle.Render(m.patExpDays+" ")),
			"",
			m.hintStyle.Render("Enter=create, Esc=cancel"),
		)
		return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
	}

	if m.loading {
		lines = append(lines, m.mutedStyle.Render("Loading PATs..."))
		return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
	}

	if m.errorText != "" {
		lines = append(lines, m.errorStyle.Render("Error: "+m.errorText))
		lines = append(lines, "")
	}

	if len(m.pats) == 0 {
		lines = append(lines, m.mutedStyle.Render("No PATs created yet."))
		lines = append(lines, "")
		lines = append(lines, m.hintStyle.Render("n=create, b=back, q=quit"))
		return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
	}

	lines = append(lines, m.headerStyle.Render("Your PATs"))
	lines = append(lines, m.hintStyle.Render("Newly created PAT will be copied to clipboard"))
	lines = append(lines, "")

	shown := 0
	for i := 0; i < len(m.pats) && shown < contentHeight-4; i++ {
		pat := m.pats[i]
		prefix := "  "
		expText := "never"
		if pat.ExpiresAt != nil {
			expText = pat.ExpiresAt.Format("2006-01-02")
		}

		state := "disabled"
		if pat.Enabled {
			state = "enabled"
		}
		permText := strings.Join(pat.Permissions, ", ")
		if permText == "" {
			permText = "no permissions"
		}
		label := fmt.Sprintf("%s [%s] (Exp: %s) - %s", shortID(pat.ID), state, expText, permText)
		if i == m.patCursor {
			prefix = "▶ "
			label = m.activeStyle.Render(label)
		}
		lines = append(lines, prefix+label)
		shown++
	}

	lines = append(lines, "")
	lines = append(lines, m.hintStyle.Render("j/k=nav, n=new, d=delete, r=refresh, b=back, q=quit"))

	return m.formatContent(strings.Join(lines, "\n"), m.width, m.height)
}

// ============ Command Functions ============

func (m appModel) loginCmd(username, password string) tea.Cmd {
	baseURL, apiPrefix := m.cfg.BaseURL, m.cfg.APIPrefix
	return func() tea.Msg {
		token, uname, err := auth.Login(baseURL, apiPrefix, username, password)
		if err != nil {
			return loginFailedMsg{Err: err}
		}

		// attempt to save session but don't fail login if save fails
		_ = auth.SaveSession(token, uname)

		return loginSuccessMsg{Token: token, Username: uname}
	}
}

func (m appModel) loadContestsCmd() tea.Cmd {
	client := m.client
	page := m.contestsPage
	limit := m.contestsLimit
	return func() tea.Msg {
		resp, err := client.GetContestsPage(page, limit, nil)
		if err != nil {
			return contestsLoadedMsg{Contests: nil, Err: err}
		}
		return contestsLoadedMsg{Contests: resp.Data, Page: resp.Page, TotalItems: resp.TotalItems, HasNextPage: resp.HasNextPage, Err: nil}
	}
}

func (m appModel) loadLeaderboardCmd(contestID string, prevRows []model.LeaderboardRow) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		snapshot, deltas, err := collectSnapshot(client, contestID, prevRows)
		return leaderboardLoadedMsg{Snapshot: snapshot, Deltas: deltas, Err: err}
	}
}

func (m appModel) loadSubmissionsCmd(contestID, teamID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		submissions, err := client.GetSubmissions(contestID)
		if err != nil {
			return submissionsLoadedMsg{Submissions: nil, Err: err}
		}

		filtered := make([]model.Submission, 0, len(submissions))
		for _, sub := range submissions {
			if sub.TeamID == teamID {
				filtered = append(filtered, sub)
			}
		}

		return submissionsLoadedMsg{Submissions: filtered, Err: nil}
	}
}

func (m appModel) loadSubmissionDetailCmd(contestID, submissionID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		submission, err := client.GetSubmission(contestID, submissionID)
		if err != nil {
			return submissionDetailLoadedMsg{Detail: nil, Err: err}
		}

		scores, err := client.GetScores(submissionID)
		if err != nil {
			return submissionDetailLoadedMsg{Detail: nil, Err: err}
		}

		total := 0.0
		for _, score := range scores {
			total += score.Score
		}

		detail := &model.SubmissionDetail{
			ID:          submission.ID,
			TeamID:      submission.TeamID,
			ContestID:   submission.ContestID,
			Title:       submission.Title,
			Description: submission.Description,
			Status:      submission.Status,
			SubmittedAt: submission.SubmittedAt,
			UpdatedAt:   submission.UpdatedAt,
			Scores:      scores,
			TotalScore:  total,
		}

		return submissionDetailLoadedMsg{Detail: detail, Err: nil}
	}
}

func (m appModel) loadPATsCmd() tea.Cmd {
	token, baseURL, apiPrefix := m.sessionToken, m.cfg.BaseURL, m.cfg.APIPrefix
	return func() tea.Msg {
		pats, err := auth.GetPATs(baseURL, apiPrefix, token)
		return patsLoadedMsg{PATs: pats, Err: err}
	}
}

func (m appModel) createPATCmd(name string, expDays int) tea.Cmd {
	token, baseURL, apiPrefix := m.sessionToken, m.cfg.BaseURL, m.cfg.APIPrefix
	return func() tea.Msg {
		pat, err := auth.CreatePAT(baseURL, apiPrefix, token, name, expDays)
		if err != nil {
			return patCreatedMsg{Token: "", Err: err}
		}

		// try to copy token to clipboard; best-effort
		if pat != nil && pat.Token != "" {
			_ = clipboard.WriteAll(pat.Token)
		}

		return patCreatedMsg{Token: pat.Token, Err: nil}
	}
}

func (m appModel) deletePATCmd(patID string) tea.Cmd {
	token, baseURL, apiPrefix := m.sessionToken, m.cfg.BaseURL, m.cfg.APIPrefix
	return func() tea.Msg {
		err := auth.DeletePAT(baseURL, apiPrefix, token, patID)
		if err != nil {
			return patsDeletedMsg{Err: err}
		}

		// Reload PATs list
		pats, err := auth.GetPATs(baseURL, apiPrefix, token)
		return patsLoadedMsg{PATs: pats, Err: err}
	}
}

func (m appModel) tickCmd() tea.Cmd {
	return tea.Tick(m.cfg.Interval, func(t time.Time) tea.Msg {
		return t
	})
}

// ============ Helper Functions ============

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

// centerContent centers text in available width
func (m appModel) centerContent(content string, width, height int) string {
	lines := strings.Split(content, "\n")

	// Add top padding
	topPad := (height - len(lines)) / 2
	if topPad < 0 {
		topPad = 0
	}

	result := strings.Repeat("\n", topPad)

	for _, line := range lines {
		// Center each line
		padding := (width - lipgloss.Width(line)) / 2
		if padding < 0 {
			padding = 0
		}
		result += strings.Repeat(" ", padding) + line + "\n"
	}

	return result
}

// formatContent formats content with borders and proper sizing
func (m appModel) formatContent(content string, width, height int) string {
	boxWidth := width - 4
	if boxWidth < 20 {
		boxWidth = 20
	}

	styled := m.borderStyle.Width(boxWidth).Height(height).Render(content)
	return styled
}
