package api

import (
	"bytes"
	"coding-contest-client-cli/internal/model"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type Client struct {
	baseURL   string
	apiPrefix string
	token     string
	http      *http.Client
}

func New(baseURL, apiPrefix, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiPrefix: ensureLeadingSlash(apiPrefix),
		token:     token,
		http:      &http.Client{Timeout: timeout},
	}
}

func (c *Client) GetContests() ([]model.Contest, error) {
	return fetchAllPages[model.Contest](c, "/contests")
}

func (c *Client) GetContestsPage(page, limit int, filters map[string]string) (model.PaginationResponse[model.Contest], error) {
	query := url.Values{}
	query.Set("page", fmt.Sprintf("%d", page))
	query.Set("limit", fmt.Sprintf("%d", limit))
	if filters != nil {
		b, err := json.Marshal(filters)
		if err == nil {
			query.Set("filters", string(b))
		}
	}

	var resp model.PaginationResponse[model.Contest]
	if err := c.getJSON("/contests", query, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func (c *Client) GetSubmissions(contestID string) ([]model.Submission, error) {
	endpoint := fmt.Sprintf("/contests/%s/submissions", contestID)
	return fetchAllPages[model.Submission](c, endpoint)
}

func (c *Client) GetSubmissionsByTeam(contestID, teamID string) ([]model.Submission, error) {
	endpoint := fmt.Sprintf("/contests/%s/submissions", contestID)
	filters := map[string]string{
		"teamId": teamID,
	}
	return fetchAllPagesWithFilters[model.Submission](c, endpoint, filters)
}

func (c *Client) GetSubmission(contestID, submissionID string) (model.Submission, error) {
	var sub model.Submission
	err := c.getJSON(fmt.Sprintf("/contests/%s/submissions/%s", contestID, submissionID), nil, &sub)
	return sub, err
}

func (c *Client) GetScores(submissionID string) ([]model.Score, error) {
	endpoint := fmt.Sprintf("/submissions/%s/scores", submissionID)
	return fetchAllPages[model.Score](c, endpoint)
}

func (c *Client) GetTeam(teamID string) (model.Team, error) {
	var team model.Team
	err := c.getJSON(fmt.Sprintf("/teams/%s", teamID), nil, &team)
	return team, err
}

func (c *Client) UpdateContest(contestID string, payload map[string]any) (model.Contest, error) {
	var contest model.Contest
	endpoint := fmt.Sprintf("/contests/%s", contestID)
	err := c.patchJSON(endpoint, payload, &contest)
	return contest, err
}

func fetchAllPages[T any](c *Client, endpoint string) ([]T, error) {
	return fetchAllPagesWithFilters[T](c, endpoint, nil)
}

func fetchAllPagesWithFilters[T any](c *Client, endpoint string, filters map[string]string) ([]T, error) {
	all := make([]T, 0, 50)
	page := 1

	for {
		query := url.Values{}
		query.Set("page", fmt.Sprintf("%d", page))
		query.Set("limit", "50")
		if filters != nil {
			b, err := json.Marshal(filters)
			if err == nil {
				query.Set("filters", string(b))
			}
		}

		var resp model.PaginationResponse[T]
		if err := c.getJSON(endpoint, query, &resp); err != nil {
			return nil, err
		}

		all = append(all, resp.Data...)
		if !resp.HasNextPage {
			break
		}
		page++
	}

	return all, nil
}

func (c *Client) getJSON(endpoint string, query url.Values, out any) error {
	return c.requestJSON(http.MethodGet, endpoint, query, nil, out)
}

func (c *Client) patchJSON(endpoint string, body any, out any) error {
	return c.requestJSON(http.MethodPatch, endpoint, nil, body, out)
}

func (c *Client) requestJSON(method, endpoint string, query url.Values, body any, out any) error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	u.Path = path.Join(u.Path, c.apiPrefix, endpoint)
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body failed: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, u.String(), reader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("api error %d: %s", res.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response failed: %w; body=%s", err, string(respBody))
	}

	return nil
}

func ensureLeadingSlash(v string) string {
	if v == "" {
		return "/"
	}
	if strings.HasPrefix(v, "/") {
		return strings.TrimRight(v, "/")
	}
	return "/" + strings.TrimRight(v, "/")
}
