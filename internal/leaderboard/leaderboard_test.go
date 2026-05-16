package leaderboard

import (
	"coding-contest-client-cli/internal/model"
	"testing"
	"time"
)

func TestBuild(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	submissions := []model.Submission{
		{ID: "s1", TeamID: "team-1", Title: "A", Status: "submitted", UpdatedAt: now},
		{ID: "s2", TeamID: "team-2", Title: "B", Status: "judged", UpdatedAt: now.Add(1 * time.Hour)},
		{ID: "s3", TeamID: "team-1", Title: "C", Status: "judged", UpdatedAt: now.Add(2 * time.Hour)},
	}
	scores := map[string][]model.Score{
		"s1": {{Score: 10}},
		"s2": {{Score: 25}},
		"s3": {{Score: 5}},
	}
	names := map[string]string{"team-1": "Alpha", "team-2": "Beta"}

	rows := Build(submissions, scores, names)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].TeamName != "Beta" || rows[0].TotalScore != 25 {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
	if rows[1].TeamName != "Alpha" || rows[1].TotalScore != 15 {
		t.Fatalf("unexpected second row: %+v", rows[1])
	}
	if rows[0].Rank != 1 || rows[1].Rank != 2 {
		t.Fatalf("unexpected ranks: %+v", rows)
	}
}

func TestDiff(t *testing.T) {
	prev := []model.LeaderboardRow{
		{Rank: 1, TeamID: "team-1", TeamName: "Alpha", TotalScore: 10},
		{Rank: 2, TeamID: "team-2", TeamName: "Beta", TotalScore: 8},
	}
	curr := []model.LeaderboardRow{
		{Rank: 1, TeamID: "team-2", TeamName: "Beta", TotalScore: 12},
		{Rank: 2, TeamID: "team-1", TeamName: "Alpha", TotalScore: 10},
		{Rank: 3, TeamID: "team-3", TeamName: "Gamma", TotalScore: 7},
	}

	deltas := Diff(prev, curr)
	if len(deltas) != 3 {
		t.Fatalf("expected 3 deltas, got %d", len(deltas))
	}
	if deltas[0].TeamID != "team-2" || deltas[0].Message != "rank ↑" {
		t.Fatalf("unexpected delta[0]: %+v", deltas[0])
	}
	if deltas[2].TeamID != "team-3" || deltas[2].Message != "NEW team entered leaderboard" {
		t.Fatalf("unexpected delta[2]: %+v", deltas[2])
	}
}
