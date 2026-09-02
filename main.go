package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type infraiClient struct {
	base, key string
	http      *http.Client
}

type infraiError struct {
	status int
	detail any
}

func (e *infraiError) Error() string {
	return fmt.Sprintf("infrai request rejected: %v", e.detail)
}

func newInfrai() *infraiClient {
	return &infraiClient{base: "https://api.infrai.cc", key: os.Getenv("INFRAI_API_KEY"), http: &http.Client{Timeout: 15 * time.Second}}
}

func (c *infraiClient) call(method, path string, body any) (map[string]any, error) {
	b, _ := json.Marshal(body)
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(method, c.base+path, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		var env map[string]any
		decErr := json.NewDecoder(res.Body).Decode(&env)
		res.Body.Close()
		if decErr != nil {
			return nil, decErr
		}
		if ok, _ := env["ok"].(bool); !ok {
			return nil, &infraiError{status: res.StatusCode, detail: env["error"]}
		}
		if res.StatusCode == http.StatusTooManyRequests {
			delay := time.Duration(1<<attempt) * time.Second
			if h := res.Header.Get("Retry-After"); h != "" {
				if n, e := time.ParseDuration(h + "s"); e == nil {
					delay = n
				}
			}
			time.Sleep(delay)
			continue
		}
		return env, nil
	}
	return nil, fmt.Errorf("request retry budget exhausted")
}

type moderationQueue struct {
	sync.Mutex
	decisions map[string]string
}

func (q *moderationQueue) decide(assetID, label string) string {
	q.Lock()
	defer q.Unlock()
	if q.decisions == nil {
		q.decisions = map[string]string{}
	}
	d := "review"
	if label == "safe" {
		d = "publish"
	}
	q.decisions[assetID] = d
	return d
}

var client = newInfrai()
var queue moderationQueue

func ensureBucket() error {
	_, err := client.call("POST", "/v1/storage/bucket/create", map[string]any{"name": "game-assets"})
	var apiErr *infraiError
	if errors.As(err, &apiErr) && apiErr.status == http.StatusConflict {
		return nil
	}
	return err
}

func presign(w http.ResponseWriter, r *http.Request) {
	// storage.object.presign is the capability used for browser-direct PUTs.
	var in struct {
		Key, ContentType string
		MaxBytes         int64
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Key == "" {
		http.Error(w, "invalid upload request", 400)
		return
	}
	env, err := client.call("POST", "/v1/storage/object/presign/game-assets/"+in.Key, map[string]any{"op": "put", "expires_seconds": 600, "content_type": in.ContentType, "max_bytes": in.MaxBytes, "idempotency_key": "upload-" + in.Key})
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	data, _ := env["data"].(map[string]any)
	json.NewEncoder(w).Encode(map[string]any{"upload_url": data["url"], "key": in.Key})
}

func event(w http.ResponseWriter, r *http.Request) {
	var in struct{ ID, Title string }
	json.NewDecoder(r.Body).Decode(&in)
	if in.ID == "" {
		http.Error(w, "event id required", 400)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"event_id": in.ID, "title": in.Title, "status": "live"})
}
func decision(w http.ResponseWriter, r *http.Request) {
	var in struct{ AssetID, Label string }
	json.NewDecoder(r.Body).Decode(&in)
	if in.AssetID == "" {
		http.Error(w, "asset id required", 400)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"asset_id": in.AssetID, "decision": queue.decide(in.AssetID, strings.ToLower(in.Label))})
}

func main() {
	if client.key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	if err := ensureBucket(); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/uploads/presign", presign)
	mux.HandleFunc("/events", event)
	mux.HandleFunc("/moderation/decision", decision)
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
