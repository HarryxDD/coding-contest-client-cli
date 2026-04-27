package api

import (
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

func (c *Client) GetSubmissions(contestID string) ([]model.Submission, error) {
	endpoint := fmt.Sprintf("/contests/%s/submissions", contestID)
	return fetchAllPages[model.Submission](c, endpoint)
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

func fetchAllPages[T any](c *Client, endpoint string) ([]T, error) {
	all := make([]T, 0, 50)
	page := 1

	for {
		query := url.Values{}
		query.Set("page", fmt.Sprintf("%d", page))
		query.Set("limit", "50")

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
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	u.Path = path.Join(u.Path, c.apiPrefix, endpoint)
	u.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("api error %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response failed: %w; body=%s", err, string(body))
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
