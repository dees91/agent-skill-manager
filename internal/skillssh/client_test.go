package skillssh

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGetPageNormalizesLeaderboardAndCachesIt(t *testing.T) {
	fixed := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/skills/hot/0" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{"skills":[{"source":"demo/skills","skillId":"alpha:beta","name":"Alpha","installs":42,"installsYesterday":3,"change":2}],"page":0,"total":1,"hasMore":false}`))
	}))
	defer server.Close()

	client := testClient(t, server, fixed)
	page, err := client.GetPage(context.Background(), ViewHot, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Skills) != 1 || page.Skills[0].ID != "demo/skills/alpha:beta" || page.Skills[0].InstallURL != "https://github.com/demo/skills" {
		t.Fatalf("page = %#v", page)
	}
	if page.Skills[0].Change != 2 || page.FetchedAt != fixed || page.Offline || page.FromCache {
		t.Fatalf("page metadata = %#v", page)
	}
	if _, err := os.Stat(client.cache.path); err != nil {
		t.Fatalf("cache not written: %v", err)
	}
}

func TestGetPageUsesFreshCacheAndFallsBackOffline(t *testing.T) {
	fixed := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if requests > 1 {
			http.Error(writer, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = writer.Write([]byte(`{"skills":[{"source":"demo/skills","skillId":"alpha","name":"alpha","installs":9}],"page":0,"total":1,"hasMore":false}`))
	}))
	defer server.Close()
	client := testClient(t, server, fixed)

	if _, err := client.GetPage(context.Background(), ViewAllTime, 0, true); err != nil {
		t.Fatal(err)
	}
	cached, err := client.GetPage(context.Background(), ViewAllTime, 0, false)
	if err != nil || !cached.FromCache || cached.Offline || requests != 1 {
		t.Fatalf("fresh cache = %#v err=%v requests=%d", cached, err, requests)
	}
	client.now = func() time.Time { return fixed.Add(2 * time.Hour) }
	offline, err := client.GetPage(context.Background(), ViewAllTime, 0, false)
	if err != nil || !offline.FromCache || !offline.Offline || requests != 2 {
		t.Fatalf("offline cache = %#v err=%v requests=%d", offline, err, requests)
	}
}

func TestSearchAndCachedOfflineSearch(t *testing.T) {
	fixed := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/search" {
			if request.URL.Query().Get("q") != "react native" || request.URL.Query().Get("limit") != "100" {
				t.Fatalf("query = %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"skills":[{"id":"expo/skills/react-native","skillId":"react-native","name":"React Native","installs":5,"source":"expo/skills"}],"count":1,"searchType":"semantic"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"skills":[{"source":"demo/skills","skillId":"cached-react","name":"Cached React","installs":7}],"page":0,"total":1,"hasMore":false}`))
	}))
	client := testClient(t, server, fixed)
	page, err := client.Search(context.Background(), "react native")
	if err != nil || page.SearchType != "semantic" || page.Skills[0].ID != "expo/skills/react-native" {
		t.Fatalf("search = %#v err=%v", page, err)
	}
	if _, err := client.GetPage(context.Background(), ViewAllTime, 0, true); err != nil {
		t.Fatal(err)
	}
	server.Close()
	cachedSearch, err := client.Search(context.Background(), "react native")
	if err != nil || !cachedSearch.Offline || cachedSearch.SearchType != "cached-search" || len(cachedSearch.Skills) != 1 {
		t.Fatalf("cached search = %#v err=%v", cachedSearch, err)
	}
	offline, err := client.Search(context.Background(), "cached")
	if err != nil || !offline.Offline || offline.SearchType != "local-cache" || len(offline.Skills) != 1 {
		t.Fatalf("offline search = %#v err=%v", offline, err)
	}
}

func TestGetDetailParsesOnlySkillDescriptionAndCachesProjection(t *testing.T) {
	fixed := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/download/demo/skills/alpha" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{"files":[{"path":"SKILL.md","contents":"---\nname: alpha\ndescription: A safe display description.\n---\nDo dangerous things"},{"path":"script.sh","contents":"secret supporting file"}],"hash":"abc"}`))
	}))
	defer server.Close()
	client := testClient(t, server, fixed)
	skill := Skill{ID: "demo/skills/alpha", SkillID: "alpha", Name: "alpha", Source: "demo/skills", SourceType: "github"}
	detail, err := client.GetDetail(context.Background(), skill, true)
	if err != nil || detail.Description != "A safe display description." || detail.AuditStatus != "external-only" {
		t.Fatalf("detail = %#v err=%v", detail, err)
	}
	cache, err := os.ReadFile(client.cache.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cache), "dangerous") || strings.Contains(string(cache), "supporting file") {
		t.Fatalf("cache persisted raw skill files: %s", cache)
	}
}

func TestRejectsUnsafeOrOversizedResponses(t *testing.T) {
	for name, response := range map[string]string{
		"unsafe source": `{"skills":[{"source":"../escape/repo","skillId":"alpha","name":"alpha","installs":1}],"page":0,"total":1,"hasMore":false}`,
		"unsafe slug":   `{"skills":[{"source":"demo/skills","skillId":"../alpha","name":"alpha","installs":1}],"page":0,"total":1,"hasMore":false}`,
		"duplicate ID":  `{"skills":[{"source":"demo/skills","skillId":"alpha","name":"alpha","installs":1},{"source":"demo/skills","skillId":"alpha","name":"alpha","installs":1}],"page":0,"total":2,"hasMore":false}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { _, _ = writer.Write([]byte(response)) }))
			defer server.Close()
			client := testClient(t, server, time.Now())
			if _, err := client.GetPage(context.Background(), ViewAllTime, 0, true); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{"skills":[],"padding":"`+strings.Repeat("x", maxResponse)+`","page":0,"total":0,"hasMore":false}`)
	}))
	defer server.Close()
	client := testClient(t, server, time.Now())
	if _, err := client.GetPage(context.Background(), ViewAllTime, 0, true); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversize error = %v", err)
	}
}

func TestCacheRejectsUnknownVersion(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(cachePath, []byte(`{"version":99,"pages":{},"details":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (cacheStore{path: cachePath}).load(); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %v", err)
	}
}

func TestCacheRejectsUnsafeAndOversizedDocuments(t *testing.T) {
	t.Run("unsafe skill", func(t *testing.T) {
		cachePath := filepath.Join(t.TempDir(), "catalog.json")
		contents := `{"version":1,"pages":{"all-time:0":{"view":"all-time","page":0,"total":1,"skills":[{"id":"../escape/repo/alpha","skillId":"alpha","name":"alpha","source":"../escape/repo","installs":1}]}},"details":{}}`
		if err := os.WriteFile(cachePath, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := (cacheStore{path: cachePath}).load(); err == nil || !strings.Contains(err.Error(), "unsafe") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("oversized file", func(t *testing.T) {
		cachePath := filepath.Join(t.TempDir(), "catalog.json")
		file, err := os.Create(cachePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(maxCacheBytes + 1); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := (cacheStore{path: cachePath}).load(); err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestConcurrentCatalogWritesPreserveEveryPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		view := strings.Split(request.URL.Path, "/")[3]
		_, _ = fmt.Fprintf(writer, `{"skills":[{"source":"demo/skills","skillId":"%s","name":"%s","installs":1}],"page":0,"total":1,"hasMore":false}`, view, view)
	}))
	defer server.Close()
	client := testClient(t, server, time.Now())

	var group sync.WaitGroup
	errors := make(chan error, 3)
	for _, view := range []View{ViewAllTime, ViewTrending, ViewHot} {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := client.GetPage(context.Background(), view, 0, true)
			errors <- err
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	document, err := client.cache.load()
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Pages) != 3 {
		t.Fatalf("cached pages = %d, want 3", len(document.Pages))
	}
}

func testClient(t *testing.T, server *httptest.Server, now time.Time) *Client {
	t.Helper()
	return &Client{baseURL: server.URL, http: server.Client(), cache: cacheStore{path: filepath.Join(t.TempDir(), "cache", "catalog.json")}, now: func() time.Time { return now }}
}
