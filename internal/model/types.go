package model

import "time"

type PaginationResponse[T any] struct {
	Data        []T  `json:"data"`
	Page        int  `json:"page"`
	TotalItems  int  `json:"totalItems"`
	HasNextPage bool `json:"hasNextPage"`
}

type Submission struct {
	ID          string    `json:"id"`
	TeamID      string    `json:"teamId"`
	ContestID   string    `json:"contestId"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Status      string    `json:"status"`
	SubmittedAt time.Time `json:"submittedAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Score struct {
	ID           string    `json:"id"`
	SubmissionID string    `json:"submissionId"`
	JudgeID      string    `json:"judgeId"`
	CriteriaID   string    `json:"criteriaId"`
	Score        float64   `json:"score"`
	Feedback     *string   `json:"feedback"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Team struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ContestID string    `json:"contestId"`
	CreatedAt time.Time `json:"createdAt"`
}

type LeaderboardRow struct {
	Rank             int
	TeamID           string
	TeamName         string
	TotalScore       float64
	SubmissionCount  int
	LastStatus       string
	LastSubmissionAt time.Time
}

type Snapshot struct {
	Rows            []LeaderboardRow
	GeneratedAt     time.Time
	SubmissionTotal int
}

type RowDelta struct {
	TeamID   string
	Message  string
	OldRank  int
	NewRank  int
	OldScore float64
	NewScore float64
}
