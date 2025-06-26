package main

import (
	"bytes"
	"context"
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/thisislawatts/home-dashboard/todoist"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/h2non/bimg"
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

		img, _, err := image.Decode(bytes.NewReader(gray))
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to decode grayscale image: %v", err)
			return
		}

		var palette color.Palette
		for i := 0; i < 256; i++ {
			palette = append(palette, color.Gray{uint8(i)})
		}

		palettedImg := image.NewPaletted(img.Bounds(), palette)
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				c := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
				palettedImg.SetColorIndex(x, y, uint8(c.Y))
			}
		}

		c.Header("Content-Type", "image/png")
		var buf bytes.Buffer
		if err := png.Encode(&buf, palettedImg); err != nil {
			c.String(http.StatusInternalServerError, "Failed to encode paletted PNG: %v", err)
			return
		}
		c.Writer.Write(buf.Bytes())

		log.Println("Dropping cache")
		bimg.VipsCacheDropAll()
	})

	r.Run(":" + *port)
}
