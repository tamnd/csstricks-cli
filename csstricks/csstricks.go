// Package csstricks is the library behind the cst command: the HTTP client,
// request shaping, and typed data models for CSS-Tricks (css-tricks.com).
//
// Data comes from the public RSS feed at css-tricks.com/feed/. No API key
// is required. The client sends a real User-Agent, paces requests, and
// retries 429/5xx with exponential backoff.
package csstricks

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://css-tricks.com",
		UserAgent: "cst/dev (+https://github.com/tamnd/csstricks-cli)",
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client fetches the CSS-Tricks RSS feed.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	rate       time.Duration
	retries    int
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client configured by cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		userAgent:  cfg.UserAgent,
		rate:       cfg.Rate,
		retries:    cfg.Retries,
	}
}

// Article is the record emitted for CSS-Tricks articles.
type Article struct {
	Rank       int    `json:"rank"`
	Title      string `json:"title"`
	Author     string `json:"author"`
	Published  string `json:"published"`
	Summary    string `json:"summary"`
	Categories string `json:"categories"`
	URL        string `json:"url"`
}

// Latest fetches up to limit articles from the RSS feed ranked by feed order.
// limit=0 returns all entries.
func (c *Client) Latest(ctx context.Context, limit int) ([]Article, error) {
	rawURL := c.baseURL + "/feed/"
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", rawURL, err)
	}
	items := feed.Channel.Items
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	out := make([]Article, len(items))
	for i, it := range items {
		out[i] = itemToArticle(it, i+1)
	}
	return out, nil
}

// Search fetches the full RSS feed and returns up to limit articles whose title,
// summary, or categories contain query (case-insensitive). limit=0 returns all matches.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Article, error) {
	all, err := c.Latest(ctx, 0)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []Article
	for _, a := range all {
		if strings.Contains(strings.ToLower(a.Title), q) ||
			strings.Contains(strings.ToLower(a.Summary), q) ||
			strings.Contains(strings.ToLower(a.Categories), q) {
			out = append(out, a)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// get fetches a URL with pacing and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/xml, application/rss+xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// RSS 2.0 wire types

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

// rssItem maps to each <item> in the feed.
// dc:creator is matched by local name "creator" via encoding/xml.
type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	PubDate     string   `xml:"pubDate"`
	Creator     string   `xml:"creator"`
	Description string   `xml:"description"`
	Categories  []string `xml:"category"`
}

func itemToArticle(it rssItem, rank int) Article {
	cats := strings.Join(it.Categories, ", ")
	return Article{
		Rank:       rank,
		Title:      strings.TrimSpace(it.Title),
		Author:     strings.TrimSpace(it.Creator),
		Published:  parseDate(it.PubDate),
		Summary:    stripAndTruncate(it.Description, 150),
		Categories: cats,
		URL:        strings.TrimSpace(it.Link),
	}
}

// parseDate parses an RSS pubDate and returns "2006-01-02". Falls back to
// the raw string on parse error.
func parseDate(s string) string {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, "Mon, 02 Jan 2006 15:04:05 GMT"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	return s
}

// stripAndTruncate strips HTML tags, decodes common entities, and truncates
// to maxChars runes, appending "..." if truncated.
func stripAndTruncate(html string, maxChars int) string {
	var b strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	out := b.String()
	out = strings.ReplaceAll(out, "&amp;", "&")
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", `"`)
	out = strings.ReplaceAll(out, "&#39;", "'")
	out = strings.ReplaceAll(out, "&apos;", "'")
	out = strings.TrimSpace(out)
	rs := []rune(out)
	if len(rs) > maxChars {
		return string(rs[:maxChars-3]) + "..."
	}
	return out
}
