package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
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

// CacheEntry holds cached data and its expiration
type CacheEntry struct {
	Response []byte
	Expires  time.Time
	Status   int
	Header   http.Header
}

// CachingHTTPClient wraps http.Client and caches responses for 60 seconds
type CachingHTTPClient struct {
	Client *http.Client
	Cache  map[string]CacheEntry
	Mutex  sync.Mutex
	TTL    time.Duration
}

func NewCachingHTTPClient(ttl time.Duration) *CachingHTTPClient {
	return &CachingHTTPClient{
		Client: &http.Client{},
		Cache:  make(map[string]CacheEntry),
		TTL:    ttl,
	}
}

func (c *CachingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	cacheKey := req.Method + ":" + req.URL.String()
	c.Mutex.Lock()
	entry, found := c.Cache[cacheKey]
	if found && time.Now().Before(entry.Expires) {
		c.Mutex.Unlock()
		return &http.Response{
			StatusCode: entry.Status,
			Body:       ioutil.NopCloser(bytes.NewReader(entry.Response)),
			Header:     entry.Header.Clone(),
		}, nil
	}
	c.Mutex.Unlock()

	resp, err := c.Client.Do(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	c.Mutex.Lock()
	c.Cache[cacheKey] = CacheEntry{
		Response: body,
		Expires:  time.Now().Add(c.TTL),
		Status:   resp.StatusCode,
		Header:   resp.Header.Clone(),
	}
	c.Mutex.Unlock()

	return &http.Response{
		StatusCode: resp.StatusCode,
		Body:       ioutil.NopCloser(bytes.NewReader(body)),
		Header:     resp.Header.Clone(),
	}, nil
}

var cachingClient = NewCachingHTTPClient(60 * time.Second)

func main() {
	// Parse port from command-line flag
	port := flag.String("port", "8080", "HTTP server port")
	flag.Parse()

	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	token := os.Getenv("TODOIST_API_TOKEN")
	if token == "" {
		log.Fatal("TODOIST_API_TOKEN not set in environment")
	}

	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		filterQuery := "today & #Home 🏡"
		filteredTasks := getFilteredTasks(token, filterQuery)
		completedToday := getCompletedTasks(token)

		tmplBytes, err := ioutil.ReadFile("src/index.html")
		if err != nil {
			c.String(500, "Failed to read template: %v", err)
			return
		}
		tmpl, err := template.New("index").Parse(string(tmplBytes))
		if err != nil {
			c.String(500, "Failed to parse template: %v", err)
			return
		}

		data := struct {
			FilteredTasks  []TodoistItem
			CompletedToday []TodoistItem
		}{
			FilteredTasks:  filteredTasks,
			CompletedToday: completedToday,
		}

		c.Header("Content-Type", "text/html; charset=utf-8")
		err = tmpl.Execute(c.Writer, data)
		if err != nil {
			c.String(500, "Failed to execute template: %v", err)
		}
	})

	r.Run(":" + *port)
}

// urlQueryEscape escapes a string for use in a URL query.
func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}

// getFilteredTasks fetches filtered tasks using the caching client
func getFilteredTasks(token, filterQuery string) []TodoistItem {
	url := "https://api.todoist.com/api/v1/tasks/filter?query=" + url.QueryEscape(filterQuery)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := cachingClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	var respData TodoistResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil
	}
	return respData.Results
}

// getCompletedTasks fetches completed tasks for today using the caching client
func getCompletedTasks(token string) []TodoistItem {
	url := "https://api.todoist.com/api/v1/tasks/completed?project_id=6WQPPXjR3wvf9cjJ"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := cachingClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	var respData TodoistCompletedResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil
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
