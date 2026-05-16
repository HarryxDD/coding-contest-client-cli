package api

import (
	"coding-contest-client-cli/internal/model"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetContestsPage(t *testing.T) {
	t.Parallel()

	var gotPath, gotQuery, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(model.PaginationResponse[model.Contest]{
			Data:        []model.Contest{{ID: "c1", Name: "Contest 1"}},
			Page:        2,
			TotalItems:  1,
			HasNextPage: false,
		})
	}))
	defer server.Close()

	client := New(server.URL, "/api", "token-123", time.Second)
	resp, err := client.GetContestsPage(2, 25, nil)
	if err != nil {
		t.Fatalf("GetContestsPage() error = %v", err)
	}
	if gotPath != "/api/contests" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if gotQuery != "limit=25&page=2" && gotQuery != "page=2&limit=25" {
		t.Fatalf("unexpected query %q", gotQuery)
	}
	if gotAuth != "Bearer token-123" {
		t.Fatalf("unexpected auth header %q", gotAuth)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "c1" {
		t.Fatalf("unexpected response %+v", resp)
	}
}

func TestGetSubmissionAndScores(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/contests/contest-1/submissions/sub-1":
			_ = json.NewEncoder(w).Encode(model.Submission{ID: "sub-1", TeamID: "team-1", ContestID: "contest-1", Title: "Submission", Status: "submitted"})
		case "/api/submissions/sub-1/scores":
			_ = json.NewEncoder(w).Encode(model.PaginationResponse[model.Score]{
				Data: []model.Score{{ID: "score-1", SubmissionID: "sub-1", Score: 9}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := New(server.URL, "/api", "token-123", time.Second)
	sub, err := client.GetSubmission("contest-1", "sub-1")
	if err != nil {
		t.Fatalf("GetSubmission() error = %v", err)
	}
	if sub.ID != "sub-1" || sub.TeamID != "team-1" {
		t.Fatalf("unexpected submission %+v", sub)
	}

	scores, err := client.GetScores("sub-1")
	if err != nil {
		t.Fatalf("GetScores() error = %v", err)
	}
	if len(scores) != 1 || scores[0].Score != 9 {
		t.Fatalf("unexpected scores %+v", scores)
	}
}
