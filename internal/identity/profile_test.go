package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestProfileFetcher(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		sub := map[string]string{"Bearer tok-alice": "alice", "Bearer tok-bob": "bob"}[r.Header.Get("Authorization")]
		if sub == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"sub": sub, "email": sub + "@example.test", "name": "Full " + sub,
			"display_name": "Disp " + sub, "preferred_username": sub, "picture": "https://idp/" + sub + ".png",
		})
	}))
	defer srv.Close()
	f := NewProfileFetcher(srv.URL)
	ctx := context.Background()

	p, err := f.Fetch(ctx, "alice", "tok-alice")
	if err != nil {
		t.Fatal(err)
	}
	want := Profile{Email: "alice@example.test", Name: "Disp alice", Username: "alice", Picture: "https://idp/alice.png"}
	if p != want {
		t.Fatalf("got %+v, want %+v", p, want)
	}

	if _, err := f.Fetch(ctx, "alice", "tok-alice"); err != nil || calls.Load() != 1 {
		t.Fatalf("expected cached result, calls=%d err=%v", calls.Load(), err)
	}

	if _, err := f.Fetch(ctx, "carol", "tok-bob"); err == nil {
		t.Fatal("expected subject mismatch error")
	}
	if _, err := f.Fetch(ctx, "carol", ""); err == nil {
		t.Fatal("expected error without access token")
	}
	if _, err := f.Fetch(ctx, "carol", "bad"); err == nil {
		t.Fatal("expected error on userinfo 401")
	}
}
