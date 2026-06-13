package csstricks_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/csstricks-cli/csstricks"
)

// rssXML wraps items in a minimal valid RSS 2.0 feed.
func rssXML(items string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
` + items + `
  </channel>
</rss>`
}

func singleItem(title, link, pubDate, creator, category, description string) string {
	return `<item>
  <title>` + title + `</title>
  <link>` + link + `</link>
  <pubDate>` + pubDate + `</pubDate>
  <dc:creator>` + creator + `</dc:creator>
  <category><![CDATA[` + category + `]]></category>
  <description><![CDATA[` + description + `]]></description>
</item>`
}

func newTestClient(ts *httptest.Server) *csstricks.Client {
	cfg := csstricks.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return csstricks.NewClient(cfg)
}

func TestLatestParsesTitle(t *testing.T) {
	xml := rssXML(singleItem(
		"A Complete Guide to Flexbox",
		"https://css-tricks.com/snippets/css/a-guide-to-flexbox/",
		"Mon, 15 Jan 2024 12:00:00 +0000",
		"Chris Coyier",
		"CSS",
		"<p>A comprehensive guide to CSS flexbox layout.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	articles, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("got %d articles, want 1", len(articles))
	}
	if articles[0].Title != "A Complete Guide to Flexbox" {
		t.Errorf("Title = %q", articles[0].Title)
	}
}

func TestLatestParsesURL(t *testing.T) {
	wantURL := "https://css-tricks.com/snippets/css/a-guide-to-grid/"
	xml := rssXML(singleItem(
		"A Complete Guide to CSS Grid",
		wantURL,
		"Fri, 12 Jan 2024 15:30:00 +0000",
		"Chris Coyier",
		"CSS",
		"<p>Everything you need to know about CSS Grid.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	articles, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if articles[0].URL != wantURL {
		t.Errorf("URL = %q, want %q", articles[0].URL, wantURL)
	}
}

func TestLatestParsesAuthor(t *testing.T) {
	xml := rssXML(singleItem(
		"CSS Custom Properties",
		"https://css-tricks.com/css-custom-properties/",
		"Mon, 01 Jan 2024 00:00:00 +0000",
		"Robin Rendle",
		"CSS",
		"<p>Custom properties are powerful.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	articles, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if articles[0].Author != "Robin Rendle" {
		t.Errorf("Author = %q, want %q", articles[0].Author, "Robin Rendle")
	}
}

func TestLatestParsesDate(t *testing.T) {
	xml := rssXML(singleItem(
		"CSS Grid Tips",
		"https://css-tricks.com/css-grid-tips/",
		"Thu, 07 Mar 2024 18:00:00 GMT",
		"",
		"",
		"<p>Details.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	articles, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if articles[0].Published != "2024-03-07" {
		t.Errorf("Published = %q, want %q", articles[0].Published, "2024-03-07")
	}
}

func TestLatestStripsSummaryHTML(t *testing.T) {
	xml := rssXML(singleItem(
		"HTML Stripping Test",
		"https://css-tricks.com/html/",
		"Sat, 20 Jan 2024 10:00:00 +0000",
		"",
		"",
		"<p>This is the <b>summary</b> text.</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	articles, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(articles[0].Summary, "<") || strings.Contains(articles[0].Summary, ">") {
		t.Errorf("Summary contains HTML tags: %q", articles[0].Summary)
	}
	if !strings.Contains(articles[0].Summary, "summary") {
		t.Errorf("Summary text missing: %q", articles[0].Summary)
	}
}

func TestLatestTruncatesSummary(t *testing.T) {
	long := strings.Repeat("x", 300)
	xml := rssXML(singleItem(
		"Long Post",
		"https://css-tricks.com/long/",
		"Mon, 01 Jan 2024 00:00:00 +0000",
		"",
		"",
		"<p>"+long+"</p>",
	))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	articles, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	runes := []rune(articles[0].Summary)
	if len(runes) > 150 {
		t.Errorf("Summary too long: %d runes", len(runes))
	}
	if !strings.HasSuffix(articles[0].Summary, "...") {
		t.Errorf("Summary missing ellipsis: %q", articles[0].Summary)
	}
}

func TestLatestRankOrder(t *testing.T) {
	items := singleItem("A", "https://css-tricks.com/a/", "Mon, 01 Jan 2024 00:00:00 +0000", "", "", "") +
		singleItem("B", "https://css-tricks.com/b/", "Tue, 02 Jan 2024 00:00:00 +0000", "", "", "") +
		singleItem("C", "https://css-tricks.com/c/", "Wed, 03 Jan 2024 00:00:00 +0000", "", "", "")
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	articles, err := newTestClient(ts).Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 3 {
		t.Fatalf("got %d articles, want 3", len(articles))
	}
	for i, a := range articles {
		if a.Rank != i+1 {
			t.Errorf("articles[%d].Rank = %d, want %d", i, a.Rank, i+1)
		}
	}
}

func TestLatestLimit(t *testing.T) {
	items := ""
	for i := 0; i < 5; i++ {
		items += singleItem("T", "https://css-tricks.com/t/", "Mon, 01 Jan 2024 00:00:00 +0000", "", "", "")
	}
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	articles, err := newTestClient(ts).Latest(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 2 {
		t.Errorf("got %d articles with limit=2, want 2", len(articles))
	}
}

func TestSearchFiltersByTitle(t *testing.T) {
	items := singleItem("Flexbox Tips", "https://css-tricks.com/flex/", "Mon, 01 Jan 2024 00:00:00 +0000", "", "", "All about flex layout.") +
		singleItem("CSS Grid Basics", "https://css-tricks.com/grid/", "Tue, 02 Jan 2024 00:00:00 +0000", "", "", "A grid post.")
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	articles, err := newTestClient(ts).Search(context.Background(), "flexbox", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("got %d articles, want 1", len(articles))
	}
	if articles[0].Title != "Flexbox Tips" {
		t.Errorf("Title = %q", articles[0].Title)
	}
}

func TestSearchFiltersBySummary(t *testing.T) {
	items := singleItem("Random Post", "https://css-tricks.com/rand/", "Mon, 01 Jan 2024 00:00:00 +0000", "", "", "This talks about animations.") +
		singleItem("Another Post", "https://css-tricks.com/other/", "Tue, 02 Jan 2024 00:00:00 +0000", "", "", "Nothing relevant here.")
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	articles, err := newTestClient(ts).Search(context.Background(), "animations", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("got %d articles, want 1", len(articles))
	}
}

func TestSearchReturnsEmpty(t *testing.T) {
	items := singleItem("CSS Tips", "https://css-tricks.com/css/", "Mon, 01 Jan 2024 00:00:00 +0000", "", "", "CSS is great.")
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	articles, err := newTestClient(ts).Search(context.Background(), "zyxwvutsrq", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 0 {
		t.Errorf("got %d articles, want 0", len(articles))
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	items := singleItem("Retry Test", "https://css-tricks.com/retry/", "Mon, 01 Jan 2024 00:00:00 +0000", "", "", "")
	xml := rssXML(items)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(xml))
	}))
	defer ts.Close()

	cfg := csstricks.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := csstricks.NewClient(cfg)

	start := time.Now()
	_, err := c.Latest(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestGetUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(rssXML("")))
	}))
	defer ts.Close()

	cfg := csstricks.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	c := csstricks.NewClient(cfg)
	_, _ = c.Latest(context.Background(), 0)

	if gotUA == "" {
		t.Error("request carried no User-Agent")
	}
}
