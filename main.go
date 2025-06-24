package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/h2non/bimg"
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
// Use sync.RWMutex for concurrent reads
type CachingHTTPClient struct {
	Client *http.Client
	Cache  map[string]CacheEntry
	Mutex  sync.RWMutex
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
	c.Mutex.RLock()
	entry, found := c.Cache[cacheKey]
	c.Mutex.RUnlock()
	if found && time.Now().Before(entry.Expires) {
		return &http.Response{
			StatusCode: entry.Status,
			Body:       io.NopCloser(bytes.NewReader(entry.Response)),
			Header:     entry.Header.Clone(),
		}, nil
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
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
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     resp.Header.Clone(),
	}, nil
}

const (
	defaultPort      = "8080"
	cacheTTL         = 60 * time.Second
	templateFilePath = "src/index.html"
)

var cachingClient = NewCachingHTTPClient(cacheTTL)

func main() {
	port := flag.String("port", defaultPort, "HTTP server port")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	token := os.Getenv("TODOIST_API_TOKEN")
	if token == "" {
		log.Fatal("TODOIST_API_TOKEN not set in environment")
		os.Exit(1)
	}

	tmplBytes, err := os.ReadFile(templateFilePath)
	if err != nil {
		log.Fatalf("Failed to read template: %v", err)
		os.Exit(1)
	}
	tmpl, err := template.New("index").Parse(string(tmplBytes))
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
		os.Exit(1)
	}

	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		filterQuery := "today & #Home 🏡"
		filteredTasks, err := getFilteredTasks(token, filterQuery)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to get filtered tasks: %v", err)
			return
		}
		completedToday, err := getCompletedTasks(token)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to get completed tasks: %v", err)
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
		if err := tmpl.Execute(c.Writer, data); err != nil {
			c.String(http.StatusInternalServerError, "Failed to execute template: %v", err)
		}
	})

	r.GET("/image", func(c *gin.Context) {
		ctx, cancel := chromedp.NewContext(context.Background())
		defer cancel()

		var pngBuf []byte
		url := "http://localhost:" + *port + "/"
		if err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			chromedp.EmulateViewport(600, 800),
			chromedp.CaptureScreenshot(&pngBuf),
		); err != nil {
			c.String(http.StatusInternalServerError, "Failed to render image: %v", err)
			return
		}

		gray, err := bimg.NewImage(pngBuf).Colourspace(bimg.InterpretationBW)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to grayscale image: %v", err)
			return
		}

		c.Header("Content-Type", "image/png")
		c.Writer.Write(gray)

	})

	r.Run(":" + *port)
}

// getFilteredTasks fetches filtered tasks using the caching client
func getFilteredTasks(token, filterQuery string) ([]TodoistItem, error) {
	url := "https://api.todoist.com/api/v1/tasks/filter?query=" + url.QueryEscape(filterQuery)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
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

// getCompletedTasks fetches completed tasks for today using the caching client
func getCompletedTasks(token string) ([]TodoistItem, error) {
	url := "https://api.todoist.com/api/v1/tasks/completed?project_id=6WQPPXjR3wvf9cjJ"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token)
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
