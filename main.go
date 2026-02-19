package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/t3rm1n4l/go-mega"
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
	opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		log.Fatal(err)
	}

	opt.TLSConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	rdb = redis.NewClient(opt)

	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Redis connection failed:", err)
	}

	log.Println("✅ Redis connected")
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

	// 🔥 Cache check
	cache, err := rdb.Get(ctx, url).Result()
	if err == nil {
		size, _ := strconv.ParseInt(cache, 10, 64)
		json.NewEncoder(w).Encode(Response{
			Status: "success",
			Size:   size,
		})
		return
	}

	m := mega.New()

	node, err := m.NewFileFromURL(url)
	if err != nil {
		json.NewEncoder(w).Encode(Response{
			Status: "error",
			Error:  "Invalid MEGA link",
		})
		return
	}

	if node.GetSize() > maxSize {
		json.NewEncoder(w).Encode(Response{
			Status: "error",
			Error:  "File exceeds 5GB limit",
		})
		return
	}

	// Cache for 1 hour
	rdb.Set(ctx, url, strconv.FormatInt(node.GetSize(), 10), time.Hour)

	json.NewEncoder(w).Encode(Response{
		Status: "success",
		Name:   node.GetName(),
		Size:   node.GetSize(),
	})
}

func main() {
	initRedis()

	http.HandleFunc("/api", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("🚀 Server running on", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
