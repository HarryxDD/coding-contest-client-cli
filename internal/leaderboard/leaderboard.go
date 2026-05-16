package leaderboard

import (
	"coding-contest-client-cli/internal/model"
	"sort"
	"time"
)

type teamAgg struct {
	teamID           string
	teamName         string
	totalScore       float64
	submissionCount  int
	lastStatus       string
	lastSubmissionAt int64
}

func Build(
	submissions []model.Submission,
	scoresBySubmission map[string][]model.Score,
	teamNames map[string]string,
) []model.LeaderboardRow {
	aggs := map[string]*teamAgg{}

	for _, s := range submissions {
		agg, ok := aggs[s.TeamID]
		if !ok {
			agg = &teamAgg{
				teamID:   s.TeamID,
				teamName: fallbackTeamName(s.TeamID, teamNames),
			}
			aggs[s.TeamID] = agg
		}

		agg.submissionCount++
		agg.lastStatus = s.Status

		if ts := s.UpdatedAt.Unix(); ts > agg.lastSubmissionAt {
			agg.lastSubmissionAt = ts
		}

		for _, sc := range scoresBySubmission[s.ID] {
			agg.totalScore += sc.Score
		}
	}

	rows := make([]model.LeaderboardRow, 0, len(aggs))
	for _, a := range aggs {
		rows = append(rows, model.LeaderboardRow{
			TeamID:           a.teamID,
			TeamName:         a.teamName,
			TotalScore:       a.totalScore,
			SubmissionCount:  a.submissionCount,
			LastStatus:       a.lastStatus,
			LastSubmissionAt: unixToTime(a.lastSubmissionAt),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalScore == rows[j].TotalScore {
			return rows[i].TeamName < rows[j].TeamName
		}
		return rows[i].TotalScore > rows[j].TotalScore
	})

	for i := range rows {
		rows[i].Rank = i + 1
	}

	return rows
}

func Diff(prev, curr []model.LeaderboardRow) []model.RowDelta {
	prevMap := map[string]model.LeaderboardRow{}
	for _, r := range prev {
		prevMap[r.TeamID] = r
	}

	deltas := make([]model.RowDelta, 0)

	for _, now := range curr {
		old, existed := prevMap[now.TeamID]
		if !existed {
			deltas = append(deltas, model.RowDelta{
				TeamID:   now.TeamID,
				Message:  "NEW team entered leaderboard",
				OldRank:  0,
				NewRank:  now.Rank,
				OldScore: 0,
				NewScore: now.TotalScore,
			})
			continue
		}

		if old.Rank != now.Rank || old.TotalScore != now.TotalScore {
			msg := "updated"
			if now.Rank < old.Rank {
				msg = "rank ↑"
			} else if now.Rank > old.Rank {
				msg = "rank ↓"
			}
			deltas = append(deltas, model.RowDelta{
				TeamID:   now.TeamID,
				Message:  msg,
				OldRank:  old.Rank,
				NewRank:  now.Rank,
				OldScore: old.TotalScore,
				NewScore: now.TotalScore,
			})
		}
	}

	sort.Slice(deltas, func(i, j int) bool {
		return deltas[i].NewRank < deltas[j].NewRank
	})

	return deltas
}

func fallbackTeamName(teamID string, names map[string]string) string {
	if n, ok := names[teamID]; ok && n != "" {
		return n
	}
	if len(teamID) >= 8 {
		return "team-" + teamID[:8]
	}
	return "team-" + teamID
}

func unixToTime(ts int64) time.Time {
	if ts <= 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0).UTC()
}
