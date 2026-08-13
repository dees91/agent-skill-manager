// Package skillssh provides the isolated, best-effort skills.sh catalog adapter.
package skillssh

import "time"

// View is one supported skills.sh leaderboard.
type View string

const (
	ViewAllTime  View = "all-time"
	ViewTrending View = "trending"
	ViewHot      View = "hot"
)

// Skill is the normalized catalog summary shared by leaderboard and search.
type Skill struct {
	ID                string  `json:"id"`
	SkillID           string  `json:"skillId"`
	Name              string  `json:"name"`
	Source            string  `json:"source"`
	Installs          int64   `json:"installs"`
	WeeklyInstalls    []int64 `json:"weeklyInstalls,omitempty"`
	InstallsYesterday int64   `json:"installsYesterday,omitempty"`
	Change            int64   `json:"change,omitempty"`
	SourceType        string  `json:"sourceType"`
	InstallURL        string  `json:"installUrl,omitempty"`
	URL               string  `json:"url"`
}

// Page is one catalog result plus connection/cache evidence.
type Page struct {
	View       View      `json:"view"`
	Page       int       `json:"page"`
	Total      int       `json:"total"`
	HasMore    bool      `json:"hasMore"`
	Skills     []Skill   `json:"skills"`
	FetchedAt  time.Time `json:"fetchedAt"`
	Offline    bool      `json:"offline"`
	FromCache  bool      `json:"fromCache"`
	SearchType string    `json:"searchType,omitempty"`
	Warning    string    `json:"warning,omitempty"`
}

// Detail is the safe display projection parsed from the catalog snapshot.
type Detail struct {
	Skill       Skill     `json:"skill"`
	Description string    `json:"description,omitempty"`
	FetchedAt   time.Time `json:"fetchedAt"`
	Offline     bool      `json:"offline"`
	FromCache   bool      `json:"fromCache"`
	AuditStatus string    `json:"auditStatus"`
	Warning     string    `json:"warning,omitempty"`
}
