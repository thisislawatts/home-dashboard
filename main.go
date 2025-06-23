package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"text/template"
	"time"

	"github.com/joho/godotenv"
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

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	token := os.Getenv("TODOIST_API_TOKEN")
	if token == "" {
		log.Fatal("TODOIST_API_TOKEN not set in environment")
	}

	filterQuery := "today & #Home 🏡"

	// Build the correct API v1 URL for getting tasks by filter query string
	url := "https://api.todoist.com/api/v1/tasks/filter?query=" + url.QueryEscape(filterQuery)

	// Fetch tasks from Todoist API
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatalf("Failed to fetch tasks: %s", resp.Status)
	}

	var respData TodoistResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		log.Fatal(err)
	}

	// Fetch completed tasks for today
	completedToday := fetchCompletedTasks(token)

	// Load HTML template from src/index.html
	tmplBytes, err := ioutil.ReadFile("src/index.html")
	if err != nil {
		log.Fatalf("Failed to read template: %v", err)
	}
	tmpl, err := template.New("index").Parse(string(tmplBytes))
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	// Prepare data for template
	data := struct {
		FilteredTasks  []TodoistItem
		CompletedToday []TodoistItem
	}{
		FilteredTasks:  respData.Results,
		CompletedToday: completedToday,
	}

	// Render template to dist/index.html
	f, err := os.Create("dist/index.html")
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer f.Close()
	if err := tmpl.Execute(f, data); err != nil {
		log.Fatalf("Failed to execute template: %v", err)
	}
}

// urlQueryEscape escapes a string for use in a URL query.
func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}

// fetchCompletedTasks returns a slice of TodoistItem completed today
func fetchCompletedTasks(token string) []TodoistItem {
	url := "https://api.todoist.com/api/v1/tasks/completed?project_id=6WQPPXjR3wvf9cjJ"
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatalf("Failed to fetch completed tasks: %s", resp.Status)
	}

	var respData TodoistCompletedResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		log.Fatal(err)
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
	return completedToday
}
