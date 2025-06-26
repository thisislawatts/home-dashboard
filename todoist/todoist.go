package todoist

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

// Deadline represents the deadline object in a Todoist task.
type Deadline struct {
	Date      string  `json:"date"`
	Datetime  *string `json:"datetime,omitempty"`
	Recurring bool    `json:"recurring"`
	String    string  `json:"string"`
	Timezone  *string `json:"timezone,omitempty"`
}

// Duration represents the duration object in a Todoist task.
type Duration struct {
	Amount int    `json:"amount"`
	Unit   string `json:"unit"`
}

// Due represents the due object in a Todoist task.
type Due struct {
	Date      string  `json:"date"`
	Datetime  *string `json:"datetime,omitempty"`
	Recurring bool    `json:"recurring"`
	String    string  `json:"string"`
	Timezone  *string `json:"timezone,omitempty"`
}

// TodoistItem represents a single task item in Todoist.
type TodoistItem struct {
	UserID         string    `json:"user_id"`
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	SectionID      *string   `json:"section_id"`
	ParentID       *string   `json:"parent_id"`
	AddedByUID     *string   `json:"added_by_uid"`
	AssignedByUID  *string   `json:"assigned_by_uid"`
	ResponsibleUID *string   `json:"responsible_uid"`
	Labels         []string  `json:"labels"`
	Deadline       *Deadline `json:"deadline"`
	Duration       *Duration `json:"duration"`
	Checked        bool      `json:"checked"`
	IsDeleted      bool      `json:"is_deleted"`
	AddedAt        *string   `json:"added_at"`
	CompletedAt    *string   `json:"completed_at"`
	UpdatedAt      *string   `json:"updated_at"`
	Due            *Due      `json:"due"`
	Priority       int       `json:"priority"`
	ChildOrder     int       `json:"child_order"`
	Content        string    `json:"content"`
	Description    string    `json:"description"`
	NoteCount      int       `json:"note_count"`
	DayOrder       int       `json:"day_order"`
	IsCollapsed    bool      `json:"is_collapsed"`
}

// TodoistResponse represents the response structure from the Todoist API for filtered tasks.
type TodoistResponse struct {
	Results    []TodoistItem `json:"results"`
	NextCursor *string       `json:"next_cursor"`
}

type TodoistCompletedResponse struct {
	Items []TodoistItem `json:"items"`
}

var cachingClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: 10 * time.Second,
	},
}

// TodoistClient wraps the API key and exposes methods for Todoist API
type TodoistClient struct {
	APIKey string
}

func NewTodoistClient(apiKey string) *TodoistClient {
	return &TodoistClient{APIKey: apiKey}
}

func (c *TodoistClient) GetFilteredTasks(filterQuery string) ([]TodoistItem, error) {
	url := "https://api.todoist.com/api/v1/tasks/filter?query=" + url.QueryEscape(filterQuery)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+c.APIKey)
	resp, err := cachingClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	var respData TodoistResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, err
	}
	return respData.Results, nil
}

func (c *TodoistClient) GetCompletedTasks() ([]TodoistItem, error) {
	url := "https://api.todoist.com/api/v1/tasks/completed?project_id=6WQPPXjR3wvf9cjJ"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+c.APIKey)
	resp, err := cachingClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	var respData TodoistCompletedResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, err
	}
	today := time.Now().Format("2006-01-02")
	var completedToday []TodoistItem
	for _, item := range respData.Items {
		if item.CompletedAt != nil {
			if t, err := time.Parse(time.RFC3339Nano, *item.CompletedAt); err == nil {
				if t.Format("2006-01-02") == today {
					completedToday = append(completedToday, item)
				}
			}
		}
	}
	return completedToday, nil
}
