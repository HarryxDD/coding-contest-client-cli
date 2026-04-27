package watch

import (
	"coding-contest-client-cli/internal/api"
	"coding-contest-client-cli/internal/leaderboard"
	"coding-contest-client-cli/internal/model"
	"context"
	"fmt"
	"time"
)

type Renderer interface {
	Render(snapshot model.Snapshot, deltas []model.RowDelta, topN int)
	RenderError(err error)
}

type Watcher struct {
	client    *api.Client
	renderer  Renderer
	contestID string
	interval  time.Duration
	topN      int
}

func New(client *api.Client, renderer Renderer, contestID string, interval time.Duration, topN int) *Watcher {
	return &Watcher{
		client:    client,
		renderer:  renderer,
		contestID: contestID,
		interval:  interval,
		topN:      topN,
	}
}

func (w *Watcher) Run(ctx context.Context) error {
	var prev model.Snapshot
	hasPrev := false

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// immediate first render
	if snap, err := w.collect(); err != nil {
		w.renderer.RenderError(err)
	} else {
		w.renderer.Render(snap, nil, w.topN)
		prev = snap
		hasPrev = true
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			snap, err := w.collect()
			if err != nil {
				w.renderer.RenderError(err)
				continue
			}

			var deltas []model.RowDelta
			if hasPrev {
				deltas = leaderboard.Diff(prev.Rows, snap.Rows)
			}

			w.renderer.Render(snap, deltas, w.topN)
			prev = snap
			hasPrev = true
		}
	}
}

func (w *Watcher) collect() (model.Snapshot, error) {
	submissions, err := w.client.GetSubmissions(w.contestID)
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("get submission failed: %w", err)
	}

	teamNames := map[string]string{}
	scoresBySubmission := map[string][]model.Score{}

	seenTeam := map[string]bool{}
	for _, sub := range submissions {
		if !seenTeam[sub.TeamID] {
			team, terr := w.client.GetTeam(sub.TeamID)
			if terr == nil && team.Name != "" {
				teamNames[sub.TeamID] = team.Name
			}
			seenTeam[sub.TeamID] = true
		}

		scores, serr := w.client.GetScores(sub.ID)
		if serr != nil {
			return model.Snapshot{}, fmt.Errorf("get scores for submission %s failed: %w", sub.ID, serr)
		}
		scoresBySubmission[sub.ID] = scores
	}

	rows := leaderboard.Build(submissions, scoresBySubmission, teamNames)

	return model.Snapshot{
		Rows:            rows,
		GeneratedAt:     time.Now(),
		SubmissionTotal: len(submissions),
	}, nil
}
