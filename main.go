package main

import (
	"bytes"
	"context"
	"flag"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"text/template"
	"time"

	"github.com/thisislawatts/home-dashboard/todoist"

	"github.com/anthonynsimon/bild/effect"
	"github.com/anthonynsimon/bild/transform"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// CacheEntry holds cached data and its expiration
type CacheEntry struct {
	Response []byte
	Expires  time.Time
	Status   int
	Header   http.Header
}

const (
	defaultPort      = "8080"
	cacheTTL         = 60 * time.Second
	templateFilePath = "src/template/index.html"
)

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

	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
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

		todoistClient := todoist.NewTodoistClient(token)

		filterQuery := "today & #Home 🏡"
		filteredTasks, err := todoistClient.GetFilteredTasks(filterQuery)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to get filtered tasks: %v", err)
			return
		}
		completedToday, err := todoistClient.GetCompletedTasks()
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to get completed tasks: %v", err)
			return
		}

		data := struct {
			FilteredTasks  []todoist.TodoistItem
			CompletedToday []todoist.TodoistItem
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
		// Log memory usage at start
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		log.Printf("Memory before request: Alloc=%d MiB, Sys=%d MiB", m.Alloc/1024/1024, m.Sys/1024/1024)

		// Chrome flags optimized for Synology NAS
		allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("disable-dev-shm-usage", true),                  // Avoid shared memory issues on NAS devices
			chromedp.Flag("no-sandbox", true),                             // Disable Chrome sandbox to reduce resource overhead
			chromedp.Flag("disable-gpu", true),                            // Force software rendering (more stable on NAS)
			chromedp.Flag("disable-software-rasterizer", false),           // Enable software rasterizer for better compatibility
			chromedp.Flag("disable-background-timer-throttling", true),    // Prevent background throttling
			chromedp.Flag("disable-backgrounding-occluded-windows", true), // Keep windows active
			chromedp.Flag("disable-renderer-backgrounding", true),         // Prevent renderer from going to background
			chromedp.Flag("memory-pressure-off", true),                    // Disable memory pressure handling
			chromedp.Flag("max_old_space_size", "512"),                    // Limit V8 memory usage to 512MB
			chromedp.Flag("single-process", true),                         // Run in single process mode to reduce overhead
			chromedp.Flag("disable-javascript", true),                     // Disable JavaScript execution
		)...)
		defer func() {
			cancel()     // Ensure Chrome process is terminated
			runtime.GC() // Force garbage collection

			// Log memory usage after cleanup
			runtime.ReadMemStats(&m)
			log.Printf("Memory after cleanup: Alloc=%d MiB, Sys=%d MiB", m.Alloc/1024/1024, m.Sys/1024/1024)
		}()

		ctx, cancel := chromedp.NewContext(allocCtx)
		defer cancel()

		// Add timeout for Synology resource constraints
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		var pngBuf []byte
		url := "http://localhost:" + *port + "/"
		if err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			chromedp.WaitReady("body"),
			chromedp.Sleep(2*time.Second), // Longer wait for slower Synology CPU
			chromedp.EmulateViewport(600, 800, chromedp.EmulateScale(2.0)),
			chromedp.FullScreenshot(&pngBuf, 100),
		); err != nil {
			c.String(http.StatusInternalServerError, "Failed to render image: %v", err)
			return
		}

		img, _, err := image.Decode(bytes.NewReader(pngBuf))
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to decode screenshot: %v", err)
			return
		}

		resized := transform.Resize(img, 600, 800, transform.Lanczos)
		gray := effect.Grayscale(resized)

		c.Header("Content-Type", "image/png")

		// Stream PNG directly to response using a pipe
		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			if err := png.Encode(pw, gray); err != nil {
				log.Printf("Failed to encode PNG: %v", err)
			}
		}()

		c.DataFromReader(http.StatusOK, -1, "image/png", pr, nil)

		log.Println("Image generated with bild pipeline")
	})

	r.Run(":" + *port)
}
