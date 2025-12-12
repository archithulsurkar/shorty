# ✂️ Shorty

A fast and simple URL shortener built with Go (Gin framework) and PostgreSQL.

## Features

- 🚀 Fast URL shortening with random 6-character codes
- 📊 Click tracking and statistics
- 🔄 Automatic duplicate detection (same URL = same short code)
- 🎨 Clean, responsive web interface
- 🐳 Docker support for easy deployment
- 📡 RESTful API

## Quick Start

### Using Docker (Recommended)

```bash
# Clone the repository
git clone <your-repo-url>
cd shorty

# Start the application
docker-compose up --build

# Access at http://localhost:8080
```

### Local Development

1. **Start PostgreSQL** (or use Docker for just the database):
   ```bash
   docker-compose up db
   ```

2. **Set environment variables**:
   ```bash
   export DATABASE_URL="postgres://myuser:mypassword@localhost:5432/shortener_db?sslmode=disable"
   export APP_PORT=8080
   ```

3. **Run the application**:
   ```bash
   go mod tidy
   go run main.go
   ```

## API Endpoints

### Create Short URL
```bash
POST /api/shorten
Content-Type: application/json

{
  "url": "https://example.com/very/long/url"
}
```

**Response:**
```json
{
  "short_url": "http://localhost:8080/abc123",
  "short_code": "abc123",
  "original_url": "https://example.com/very/long/url"
}
```

### Get URL Statistics
```bash
GET /api/stats/{code}
```

**Response:**
```json
{
  "short_code": "abc123",
  "original_url": "https://example.com/very/long/url",
  "clicks": 42,
  "created_at": "2024-01-15T10:30:00Z"
}
```

### List All URLs
```bash
GET /api/urls
```

### Health Check
```bash
GET /api/health
```

### Redirect
```bash
GET /{code}
# Redirects to the original URL
```

## Configuration

Environment variables (configured in `.env`):

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_PORT` | Port for the web server | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | - |
| `POSTGRES_USER` | Database username | `myuser` |
| `POSTGRES_PASSWORD` | Database password | `mypassword` |
| `POSTGRES_DB` | Database name | `shortener_db` |
| `GIN_MODE` | Gin framework mode | `release` |

## Project Structure

```
shorty/
├── main.go              # Application entry point & handlers
├── go.mod               # Go module file
├── dockerfile           # Docker build configuration
├── docker-compose.yaml  # Docker Compose setup
├── .env                 # Environment variables
├── .gitignore          # Git ignore rules
├── sql/
│   └── init.sql        # Database schema
└── README.md           # This file
```

## Tech Stack

- **Backend**: Go 1.21 with [Gin](https://github.com/gin-gonic/gin) framework
- **Database**: PostgreSQL 15
- **Containerization**: Docker & Docker Compose

## License

MIT
