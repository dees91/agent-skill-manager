package skillssh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultBaseURL  = "https://www.skills.sh"
	maxResponse     = 5 << 20
	pageFreshness   = time.Minute
	detailFreshness = 5 * time.Minute
)

var safeSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,199}$`)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client reads the experimental anonymous skills.sh catalog surface.
type Client struct {
	baseURL string
	http    httpDoer
	cache   cacheStore
	cacheMu sync.Mutex
	now     func() time.Time
}

// New returns a production catalog client with bounded network behavior.
func New(cachePath string) *Client {
	return &Client{
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 12 * time.Second},
		cache:   cacheStore{path: cachePath},
		now:     time.Now,
	}
}

// GetPage returns one leaderboard page, falling back to cached data on failure.
func (c *Client) GetPage(ctx context.Context, view View, page int, force bool) (Page, error) {
	if !validView(view) {
		return Page{}, fmt.Errorf("unsupported skills.sh view %q", view)
	}
	if page < 0 {
		return Page{}, fmt.Errorf("skills.sh page must be non-negative")
	}
	document, cacheErr := c.loadCache()
	key := pageCacheKey(view, page)
	cached, found := document.Pages[key]
	if cacheErr == nil && found && !force && c.now().Sub(cached.FetchedAt) < pageFreshness {
		cached.FromCache = true
		cached.Offline = false
		return cached, nil
	}

	fetched, err := c.fetchPage(ctx, view, page)
	if err == nil {
		if saveErr := c.persistPage(key, fetched); saveErr != nil {
			fetched.Warning = saveErr.Error()
		}
		return fetched, nil
	}
	if found {
		cached.Offline = true
		cached.FromCache = true
		cached.Warning = "skills.sh is unavailable; showing cached catalog data"
		return cached, nil
	}
	if cacheErr != nil {
		return Page{}, fmt.Errorf("skills.sh request failed: %w; cached catalog unavailable: %v", err, cacheErr)
	}
	return Page{}, fmt.Errorf("skills.sh request failed and no cached page is available: %w", err)
}

// Search queries skills.sh and falls back to matching all cached leaderboard rows.
func (c *Client) Search(ctx context.Context, query string) (Page, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return Page{}, fmt.Errorf("search query must contain at least 2 characters")
	}
	if len([]rune(query)) > 200 {
		return Page{}, fmt.Errorf("search query must contain at most 200 characters")
	}
	document, cacheErr := c.loadCache()
	key := searchCacheKey(query)
	result, err := c.fetchSearch(ctx, query)
	if err == nil {
		if saveErr := c.persistSearch(key, result); saveErr != nil {
			result.Warning = saveErr.Error()
		}
		return result, nil
	}
	if cacheErr != nil {
		return Page{}, fmt.Errorf("skills.sh search failed: %w; cached catalog unavailable: %v", err, cacheErr)
	}
	if cached, found := document.Searches[key]; found {
		cached.Offline = true
		cached.FromCache = true
		cached.SearchType = "cached-search"
		cached.Warning = "skills.sh is unavailable; showing cached search results"
		return cached, nil
	}
	normalized := strings.ToLower(query)
	byID := map[string]Skill{}
	for _, cached := range document.Pages {
		for _, skill := range cached.Skills {
			if strings.Contains(strings.ToLower(skill.Name), normalized) || strings.Contains(strings.ToLower(skill.Source), normalized) {
				byID[skill.ID] = skill
			}
		}
	}
	skilled := make([]Skill, 0, len(byID))
	for _, skill := range byID {
		skilled = append(skilled, skill)
	}
	sort.SliceStable(skilled, func(i, j int) bool {
		if skilled[i].Installs != skilled[j].Installs {
			return skilled[i].Installs > skilled[j].Installs
		}
		return skilled[i].ID < skilled[j].ID
	})
	if len(skilled) > 100 {
		skilled = skilled[:100]
	}
	return Page{View: View("search"), Skills: skilled, Total: len(skilled), FetchedAt: newestPageTime(document), Offline: true, FromCache: true, SearchType: "local-cache", Warning: "skills.sh is unavailable; search is limited to cached rankings"}, nil
}

// GetDetail downloads a bounded skill snapshot and caches only parsed display metadata.
func (c *Client) GetDetail(ctx context.Context, skill Skill, force bool) (Detail, error) {
	if err := validateSkill(skill); err != nil {
		return Detail{}, err
	}
	document, cacheErr := c.loadCache()
	cached, found := document.Details[skill.ID]
	if cacheErr == nil && found && !force && c.now().Sub(cached.FetchedAt) < detailFreshness {
		cached.Skill = skill
		cached.FromCache = true
		cached.Offline = false
		return cached, nil
	}
	detail, err := c.fetchDetail(ctx, skill)
	if err == nil {
		if saveErr := c.persistDetail(skill.ID, detail); saveErr != nil {
			detail.Warning = saveErr.Error()
		}
		return detail, nil
	}
	if found {
		cached.Skill = skill
		cached.Offline = true
		cached.FromCache = true
		cached.Warning = "skills.sh is unavailable; showing cached skill details"
		return cached, nil
	}
	return Detail{}, fmt.Errorf("load skills.sh details: %w", err)
}

func (c *Client) fetchPage(ctx context.Context, view View, pageNumber int) (Page, error) {
	var response struct {
		Skills  []remoteSkill `json:"skills"`
		Page    int           `json:"page"`
		Total   int           `json:"total"`
		HasMore bool          `json:"hasMore"`
	}
	endpoint := fmt.Sprintf("/api/skills/%s/%d", view, pageNumber)
	if err := c.getJSON(ctx, endpoint, &response); err != nil {
		return Page{}, err
	}
	if response.Page != pageNumber || response.Total < 0 || len(response.Skills) > 200 {
		return Page{}, fmt.Errorf("invalid skills.sh leaderboard response")
	}
	skilled, err := normalizeSkills(response.Skills)
	if err != nil {
		return Page{}, err
	}
	return Page{View: view, Page: response.Page, Total: response.Total, HasMore: response.HasMore, Skills: skilled, FetchedAt: c.now().UTC()}, nil
}

func (c *Client) fetchSearch(ctx context.Context, query string) (Page, error) {
	var response struct {
		Skills     []remoteSkill `json:"skills"`
		Count      int           `json:"count"`
		SearchType string        `json:"searchType"`
	}
	endpoint := "/api/search?q=" + url.QueryEscape(query) + "&limit=100"
	if err := c.getJSON(ctx, endpoint, &response); err != nil {
		return Page{}, err
	}
	if len(response.Skills) > 100 || response.Count < 0 {
		return Page{}, fmt.Errorf("invalid skills.sh search response")
	}
	skilled, err := normalizeSkills(response.Skills)
	if err != nil {
		return Page{}, err
	}
	return Page{View: View("search"), Skills: skilled, Total: response.Count, FetchedAt: c.now().UTC(), SearchType: response.SearchType}, nil
}

func (c *Client) fetchDetail(ctx context.Context, skill Skill) (Detail, error) {
	parts := strings.Split(skill.Source, "/")
	segments := append(parts, skill.SkillID)
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	var response struct {
		Files []struct {
			Path     string `json:"path"`
			Contents string `json:"contents"`
		} `json:"files"`
	}
	if err := c.getJSON(ctx, "/api/download/"+strings.Join(segments, "/"), &response); err != nil {
		return Detail{}, err
	}
	description := ""
	for _, file := range response.Files {
		if path.Clean(file.Path) == "SKILL.md" {
			description = frontmatterDescription(file.Contents)
			break
		}
	}
	return Detail{Skill: skill, Description: description, FetchedAt: c.now().UTC(), AuditStatus: "external-only"}, nil

}

func (c *Client) getJSON(ctx context.Context, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Skill-Manager/experimental-skills-sh")
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("skills.sh returned HTTP %d", response.StatusCode)
	}
	limited := io.LimitReader(response.Body, maxResponse+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(data) > maxResponse {
		return fmt.Errorf("skills.sh response exceeds %d bytes", maxResponse)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode skills.sh response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("decode skills.sh response: unexpected trailing data")
	}
	return nil
}

func (c *Client) loadCache() (cacheDocument, error) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	return c.cache.load()
}

func (c *Client) persistPage(key string, page Page) error {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	document, err := c.cache.load()
	if err != nil {
		document = emptyCache()
	}
	document.Pages[key] = page
	pruneOldestPages(document.Pages, 100)
	return c.cache.save(document)
}

func (c *Client) persistSearch(key string, page Page) error {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	document, err := c.cache.load()
	if err != nil {
		document = emptyCache()
	}
	document.Searches[key] = page
	pruneOldestPages(document.Searches, 50)
	return c.cache.save(document)
}

func (c *Client) persistDetail(key string, detail Detail) error {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	document, err := c.cache.load()
	if err != nil {
		document = emptyCache()
	}
	document.Details[key] = detail
	pruneOldestDetails(document.Details, 200)
	return c.cache.save(document)
}

type remoteSkill struct {
	ID                string  `json:"id"`
	SkillID           string  `json:"skillId"`
	Name              string  `json:"name"`
	Source            string  `json:"source"`
	Installs          int64   `json:"installs"`
	WeeklyInstalls    []int64 `json:"weeklyInstalls"`
	InstallsYesterday int64   `json:"installsYesterday"`
	Change            int64   `json:"change"`
}

func normalizeSkills(remote []remoteSkill) ([]Skill, error) {
	result := make([]Skill, 0, len(remote))
	seen := make(map[string]struct{}, len(remote))
	for _, item := range remote {
		skillID := strings.TrimSpace(item.SkillID)
		if skillID == "" && item.ID != "" {
			skillID = path.Base(item.ID)
		}
		skill := Skill{ID: strings.TrimSpace(item.ID), SkillID: skillID, Name: strings.TrimSpace(item.Name), Source: strings.TrimSpace(item.Source), Installs: item.Installs, WeeklyInstalls: item.WeeklyInstalls, InstallsYesterday: item.InstallsYesterday, Change: item.Change}
		if skill.ID == "" {
			skill.ID = skill.Source + "/" + skill.SkillID
		}
		if skill.Name == "" {
			skill.Name = skill.SkillID
		}
		canonicalizeSkill(&skill)
		if err := validateSkill(skill); err != nil {
			return nil, fmt.Errorf("invalid skills.sh skill: %w", err)
		}
		if _, exists := seen[skill.ID]; exists {
			return nil, fmt.Errorf("invalid skills.sh response: duplicate skill ID %q", skill.ID)
		}
		seen[skill.ID] = struct{}{}
		result = append(result, skill)
	}
	return result, nil
}

func validateSkill(skill Skill) error {
	if !safeSegment.MatchString(skill.SkillID) || skill.SkillID == "." || skill.SkillID == ".." {
		return fmt.Errorf("unsafe skill identifier %q", skill.SkillID)
	}
	if strings.TrimSpace(skill.Name) == "" || len(skill.Name) > 300 {
		return fmt.Errorf("invalid skill name")
	}
	parts := strings.Split(skill.Source, "/")
	if len(parts) == 0 || len(parts) > 2 {
		return fmt.Errorf("unsafe skill source %q", skill.Source)
	}
	for _, part := range parts {
		if !safeSegment.MatchString(part) || part == "." || part == ".." {
			return fmt.Errorf("unsafe skill source %q", skill.Source)
		}
	}
	if skill.ID != skill.Source+"/"+skill.SkillID {
		return fmt.Errorf("skill ID does not match source and slug")
	}
	if skill.Installs < 0 || skill.InstallsYesterday < 0 {
		return fmt.Errorf("invalid install count")
	}
	for _, installs := range skill.WeeklyInstalls {
		if installs < 0 {
			return fmt.Errorf("invalid weekly install count")
		}
	}
	return nil
}

func isGitHubSource(source string) bool {
	parts := strings.Split(source, "/")
	return len(parts) == 2 && safeSegment.MatchString(parts[0]) && safeSegment.MatchString(parts[1])
}

func canonicalizeSkill(skill *Skill) {
	if isGitHubSource(skill.Source) {
		skill.SourceType = "github"
		skill.InstallURL = "https://github.com/" + skill.Source
		skill.URL = defaultBaseURL + "/" + skill.Source + "/" + url.PathEscape(skill.SkillID)
		return
	}
	skill.SourceType = "well-known"
	skill.InstallURL = ""
	skill.URL = defaultBaseURL + "/site/" + url.PathEscape(skill.Source) + "/" + url.PathEscape(skill.SkillID)
}

func validView(view View) bool { return view == ViewAllTime || view == ViewTrending || view == ViewHot }

func newestPageTime(document cacheDocument) time.Time {
	var newest time.Time
	for _, page := range document.Pages {
		if page.FetchedAt.After(newest) {
			newest = page.FetchedAt
		}
	}
	return newest
}

func frontmatterDescription(contents string) string {
	contents = strings.ReplaceAll(contents, "\r\n", "\n")
	lines := strings.Split(contents, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	closing := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			closing = index
			break
		}
	}
	if closing < 0 {
		return ""
	}
	frontmatterBytes := []byte(strings.Join(lines[1:closing], "\n"))
	if len(frontmatterBytes) > 64<<10 {
		return ""
	}
	var frontmatter struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal(frontmatterBytes, &frontmatter); err != nil {
		return ""
	}
	return truncateRunes(strings.TrimSpace(frontmatter.Description), 4000)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
