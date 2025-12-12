package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var db *sql.DB

// URL represents a shortened URL entry
type URL struct {
	ID          int       `json:"id"`
	ShortCode   string    `json:"short_code"`
	OriginalURL string    `json:"original_url"`
	Clicks      int       `json:"clicks"`
	CreatedAt   time.Time `json:"created_at"`
}

// ShortenRequest represents the request body for creating a short URL
type ShortenRequest struct {
	URL string `json:"url" binding:"required"`
}

// ShortenResponse represents the response after creating a short URL
type ShortenResponse struct {
	ShortURL    string `json:"short_url"`
	ShortCode   string `json:"short_code"`
	OriginalURL string `json:"original_url"`
}

// StatsResponse represents URL statistics
type StatsResponse struct {
	ShortCode   string    `json:"short_code"`
	OriginalURL string    `json:"original_url"`
	Clicks      int       `json:"clicks"`
	CreatedAt   time.Time `json:"created_at"`
}

func main() {
	// Connect to database with retry logic
	connectDB()
	defer db.Close()

	// Set Gin mode
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Set up router
	r := gin.Default()

	// Enable CORS
	r.Use(corsMiddleware())

	// API Routes
	api := r.Group("/api")
	{
		api.POST("/shorten", createShortURL)
		api.GET("/stats/:code", getStats)
		api.GET("/urls", listURLs)
		api.GET("/health", healthCheck)
	}

	// Root route - serve frontend
	r.GET("/", homeHandler)

	// Redirect route (catch-all for short codes)
	r.GET("/:code", redirectToURL)

	// Get port from environment
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Shorty is running on http://localhost:%s", port)
	r.Run(":" + port)
}

// connectDB establishes database connection with retry logic
func connectDB() {
	var err error
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://myuser:mypassword@localhost:5432/shortener_db?sslmode=disable"
	}

	// Retry connection up to 10 times (useful for Docker startup)
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", databaseURL)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Println("✓ Connected to database")
				return
			}
		}
		log.Printf("Waiting for database... (attempt %d/10)", i+1)
		time.Sleep(2 * time.Second)
	}

	log.Fatal("Failed to connect to database:", err)
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// generateShortCode creates a random 6-character code
func generateShortCode() (string, error) {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Use URL-safe base64 and take first 6 characters
	code := base64.URLEncoding.EncodeToString(bytes)
	code = strings.NewReplacer("+", "", "/", "", "=", "").Replace(code)
	if len(code) > 6 {
		code = code[:6]
	}
	return code, nil
}

// buildShortURL constructs the full short URL
func buildShortURL(c *gin.Context, code string) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + "/" + code
}

// createShortURL handles POST /api/shorten
func createShortURL(c *gin.Context) {
	var req ShortenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	// Add protocol if missing
	originalURL := req.URL
	if !strings.HasPrefix(originalURL, "http://") && !strings.HasPrefix(originalURL, "https://") {
		originalURL = "https://" + originalURL
	}

	// Check if URL already exists
	var existingCode string
	err := db.QueryRow("SELECT short_code FROM urls WHERE original_url = $1", originalURL).Scan(&existingCode)
	if err == nil {
		// URL already exists, return existing short code
		c.JSON(http.StatusOK, ShortenResponse{
			ShortURL:    buildShortURL(c, existingCode),
			ShortCode:   existingCode,
			OriginalURL: originalURL,
		})
		return
	}

	// Generate new short code
	shortCode, err := generateShortCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate short code"})
		return
	}

	// Insert into database
	_, err = db.Exec(
		"INSERT INTO urls (short_code, original_url, clicks, created_at) VALUES ($1, $2, 0, NOW())",
		shortCode, originalURL,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save URL"})
		return
	}

	c.JSON(http.StatusCreated, ShortenResponse{
		ShortURL:    buildShortURL(c, shortCode),
		ShortCode:   shortCode,
		OriginalURL: originalURL,
	})
}

// redirectToURL handles GET /:code
func redirectToURL(c *gin.Context) {
	code := c.Param("code")

	// Skip if it looks like a file request
	if strings.Contains(code, ".") {
		c.Status(http.StatusNotFound)
		return
	}

	var originalURL string
	err := db.QueryRow("SELECT original_url FROM urls WHERE short_code = $1", code).Scan(&originalURL)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Short URL not found"})
		return
	}

	// Increment click count asynchronously
	go db.Exec("UPDATE urls SET clicks = clicks + 1 WHERE short_code = $1", code)

	c.Redirect(http.StatusMovedPermanently, originalURL)
}

// getStats handles GET /api/stats/:code
func getStats(c *gin.Context) {
	code := c.Param("code")

	var stats StatsResponse
	err := db.QueryRow(
		"SELECT short_code, original_url, clicks, created_at FROM urls WHERE short_code = $1",
		code,
	).Scan(&stats.ShortCode, &stats.OriginalURL, &stats.Clicks, &stats.CreatedAt)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// listURLs handles GET /api/urls
func listURLs(c *gin.Context) {
	rows, err := db.Query("SELECT id, short_code, original_url, clicks, created_at FROM urls ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch URLs"})
		return
	}
	defer rows.Close()

	urls := []URL{}
	for rows.Next() {
		var u URL
		if err := rows.Scan(&u.ID, &u.ShortCode, &u.OriginalURL, &u.Clicks, &u.CreatedAt); err != nil {
			continue
		}
		urls = append(urls, u)
	}

	c.JSON(http.StatusOK, urls)
}

// healthCheck handles GET /api/health
func healthCheck(c *gin.Context) {
	err := db.Ping()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": "Database connection failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// homeHandler serves the frontend
func homeHandler(c *gin.Context) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shorty - URL Shortener</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); 
            min-height: 100vh; 
            display: flex; 
            align-items: center; 
            justify-content: center;
            padding: 20px;
        }
        .container { 
            background: white; 
            padding: 40px; 
            border-radius: 16px; 
            box-shadow: 0 20px 60px rgba(0,0,0,0.3); 
            max-width: 500px; 
            width: 100%; 
        }
        h1 { color: #333; margin-bottom: 8px; font-size: 2.5em; }
        .subtitle { color: #666; margin-bottom: 30px; }
        .input-group { display: flex; gap: 10px; margin-bottom: 20px; flex-wrap: wrap; }
        input[type="text"] { 
            flex: 1; 
            min-width: 200px;
            padding: 14px 18px; 
            border: 2px solid #e0e0e0; 
            border-radius: 8px; 
            font-size: 16px; 
            transition: border-color 0.3s; 
        }
        input[type="text"]:focus { outline: none; border-color: #667eea; }
        button { 
            padding: 14px 28px; 
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); 
            color: white; 
            border: none; 
            border-radius: 8px; 
            font-size: 16px; 
            cursor: pointer; 
            transition: transform 0.2s, box-shadow 0.2s; 
        }
        button:hover { transform: translateY(-2px); box-shadow: 0 4px 12px rgba(102,126,234,0.4); }
        button:disabled { opacity: 0.7; cursor: not-allowed; transform: none; }
        .result { 
            background: #f0fdf4; 
            border: 1px solid #86efac;
            padding: 20px; 
            border-radius: 8px; 
            margin-top: 20px; 
            display: none; 
        }
        .result.show { display: block; animation: fadeIn 0.3s ease; }
        .result.error { background: #fef2f2; border-color: #fca5a5; }
        .result a { color: #667eea; font-weight: bold; word-break: break-all; font-size: 18px; }
        .result .original { color: #666; font-size: 14px; margin-top: 8px; word-break: break-all; }
        .copy-btn { 
            margin-top: 12px; 
            padding: 8px 16px; 
            font-size: 14px; 
            background: #667eea; 
        }
        .stats { margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; }
        .stats h3 { color: #333; margin-bottom: 15px; }
        .api-info { font-size: 14px; color: #666; line-height: 1.8; }
        .api-info code { background: #f0f0f0; padding: 2px 8px; border-radius: 4px; font-family: monospace; }
        @keyframes fadeIn { from { opacity: 0; transform: translateY(-10px); } to { opacity: 1; transform: translateY(0); } }
    </style>
</head>
<body>
    <div class="container">
        <h1>✂️ Shorty</h1>
        <p class="subtitle">Fast & simple URL shortener</p>
        <div class="input-group">
            <input type="text" id="urlInput" placeholder="Paste your long URL here..." onkeypress="if(event.key==='Enter')shortenURL()" />
            <button onclick="shortenURL()" id="shortenBtn">Shorten</button>
        </div>
        <div class="result" id="result"></div>
        <div class="stats">
            <h3>📡 API Endpoints</h3>
            <div class="api-info">
                <p><code>POST /api/shorten</code> — Create short URL</p>
                <p><code>GET /api/stats/{code}</code> — Get URL statistics</p>
                <p><code>GET /api/urls</code> — List all URLs</p>
                <p><code>GET /{code}</code> — Redirect to original</p>
            </div>
        </div>
    </div>
    <script>
        async function shortenURL() {
            const input = document.getElementById('urlInput');
            const result = document.getElementById('result');
            const btn = document.getElementById('shortenBtn');
            const url = input.value.trim();
            
            if (!url) {
                showResult('Please enter a URL', true);
                return;
            }
            
            btn.disabled = true;
            btn.textContent = 'Shortening...';
            
            try {
                const response = await fetch('/api/shorten', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ url: url })
                });
                
                const data = await response.json();
                
                if (response.ok) {
                    showResult(` + "`" + `
                        <a href="${data.short_url}" target="_blank">${data.short_url}</a>
                        <p class="original">Original: ${data.original_url}</p>
                        <button class="copy-btn" onclick="copyToClipboard('${data.short_url}')">📋 Copy to Clipboard</button>
                    ` + "`" + `);
                    input.value = '';
                } else {
                    showResult(data.error || 'Something went wrong', true);
                }
            } catch (error) {
                showResult('Failed to connect to server', true);
            }
            
            btn.disabled = false;
            btn.textContent = 'Shorten';
        }
        
        function showResult(content, isError = false) {
            const result = document.getElementById('result');
            result.innerHTML = content;
            result.className = 'result show' + (isError ? ' error' : '');
        }
        
        function copyToClipboard(text) {
            navigator.clipboard.writeText(text).then(() => {
                const btn = document.querySelector('.copy-btn');
                btn.textContent = '✓ Copied!';
                setTimeout(() => btn.textContent = '📋 Copy to Clipboard', 2000);
            });
        }
    </script>
</body>
</html>`
	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html)
}
