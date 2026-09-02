package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModerationDecisionPublishesOnlySafeAssets(t *testing.T) {
	q := moderationQueue{}
	for _, tc := range []struct{ label, want string }{{"safe", "publish"}, {"nsfw", "review"}, {"unknown", "review"}} {
		if got := q.decide("asset-1", tc.label); got != tc.want {
			t.Fatalf("label %q: got %q want %q", tc.label, got, tc.want)
		}
	}
}

func TestEnsureBucketAcceptsExistingBucket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"ALREADY_EXISTS"}}`))
	}))
	defer server.Close()

	previous := client
	client = &infraiClient{base: server.URL, key: "test", http: server.Client()}
	defer func() { client = previous }()

	if err := ensureBucket(); err != nil {
		t.Fatalf("existing bucket should not prevent startup: %v", err)
	}
}
