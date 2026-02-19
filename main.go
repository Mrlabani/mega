package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const maxSize int64 = 5 * 1024 * 1024 * 1024 // 5GB

var ctx = context.Background()
var rdb *redis.Client

type Response struct {
	Status string `json:"status"`
	Name   string `json:"name,omitempty"`
	Size   int64  `json:"size,omitempty"`
	Error  string `json:"error,omitempty"`
}

func initRedis() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Fatal("REDIS_URL not set")
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatal("Invalid Redis URL:", err)
	}

	// 🔥 Force TLS for rediss://
	opt.TLSConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	rdb = redis.NewClient(opt)

	// Test connection
	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Redis connection failed:", err)
	}

	fmt.Println("✅ Connected to Redis")
}

func getMegaInfo(url string) (string, int64, error) {

	// 🔥 Check cache first
	cache, err := rdb.Get(ctx, url).Result()
	if err == nil {
		parts := strings.Split(cache, "|")
		size, _ := strconv.ParseInt(parts[1], 10, 64)
		return parts[0], size, nil
	}

	cmd := exec.Command("mega-get", "--info", url)
	out, err := cmd.Output()
	if err != nil {
		return "", 0, err
	}

	lines := strings.Split(string(out), "\n")
	var name string
	var size int64

	for _, line := range lines {
		if strings.Contains(line, "name:") {
			name = strings.TrimSpace(strings.Split(line, ":")[1])
		}
		if strings.Contains(line, "size:") {
			fields := strings.Fields(line)
			size, _ = strconv.ParseInt(fields[1], 10, 64)
		}
	}

	// 🔥 Cache for 1 hour
	rdb.Set(ctx, url, fmt.Sprintf("%s|%d", name, size), time.Hour)

	return name, size, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")

	if url == "" {
		json.NewEncoder(w).Encode(Response{
			Status: "error",
			Error:  "Missing URL",
		})
		return
	}

	name, size, err := getMegaInfo(url)
	if err != nil {
		json.NewEncoder(w).Encode(Response{
			Status: "error",
			Error:  "Invalid MEGA link",
		})
		return
	}

	if size > maxSize {
		json.NewEncoder(w).Encode(Response{
			Status: "error",
			Error:  "File exceeds 5GB",
		})
		return
	}

	json.NewEncoder(w).Encode(Response{
		Status: "success",
		Name:   name,
		Size:   size,
	})
}

func main() {
	initRedis()

	http.HandleFunc("/api", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("🚀 Server running on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
