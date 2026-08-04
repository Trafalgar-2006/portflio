package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// WakaTime integration
//
// Polls the WakaTime API for coding-time summaries and language breakdown,
// shown on the admin dashboard. Entirely optional: with no WAKATIME_API_KEY
// set the worker never starts and the panel is omitted.
//
// Config:
//
//	WAKATIME_API_KEY  API key from wakatime.com/settings/api-key
//	WAKATIME_INTERVAL poll interval (default 30m)

type wakaState struct {
	mu      sync.RWMutex
	today   string
	week    string
	top     []views.LangStat
	lastErr error
	fetched time.Time
}

var waka wakaState

func (w *wakaState) snapshot() (string, string, []views.LangStat) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	top := make([]views.LangStat, len(w.top))
	copy(top, w.top)
	return w.today, w.week, top
}

// wakaSummary is the subset of the WakaTime summaries payload we read.
type wakaSummary struct {
	Data []struct {
		GrandTotal struct {
			Text string `json:"text"`
		} `json:"grand_total"`
		Languages []struct {
			Name    string  `json:"name"`
			Percent float64 `json:"percent"`
			Text    string  `json:"text"`
		} `json:"languages"`
	} `json:"data"`
	CumulativeTotal struct {
		Text string `json:"text"`
	} `json:"cumulative_total"`
}

// StartWakaTime launches the polling worker if an API key is configured.
func StartWakaTime(ctx context.Context) {
	key := os.Getenv("WAKATIME_API_KEY")
	if key == "" {
		return // optional feature, silently disabled
	}

	interval := 30 * time.Minute
	if v := os.Getenv("WAKATIME_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= time.Minute {
			interval = d
		}
	}
	log.Printf("WakaTime sync enabled (every %s)", interval)

	go func() {
		timer := time.NewTimer(8 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			if err := fetchWaka(ctx, key); err != nil {
				log.Printf("WakaTime: %v", err)
				waka.mu.Lock()
				waka.lastErr = err
				waka.mu.Unlock()
			}
			timer.Reset(interval)
		}
	}()
}

func fetchWaka(ctx context.Context, key string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(key))

	get := func(url string) (*wakaSummary, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", auth)
		req.Header.Set("User-Agent", "ssh-portfolio")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, errHTTP(resp.Status)
		}
		var out wakaSummary
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return nil, err
		}
		return &out, nil
	}

	today, err := get("https://wakatime.com/api/v1/users/current/summaries?range=Today")
	if err != nil {
		return err
	}
	week, err := get("https://wakatime.com/api/v1/users/current/summaries?range=Last%207%20Days")
	if err != nil {
		return err
	}

	todayText := "0 mins"
	if len(today.Data) > 0 && today.Data[0].GrandTotal.Text != "" {
		todayText = today.Data[0].GrandTotal.Text
	}
	weekText := week.CumulativeTotal.Text
	if weekText == "" {
		weekText = "0 mins"
	}

	// Language breakdown comes from the weekly window — a single day is too
	// noisy to be interesting.
	var top []views.LangStat
	if len(week.Data) > 0 {
		for i, l := range week.Data[0].Languages {
			if i >= 5 {
				break
			}
			top = append(top, views.LangStat{Name: l.Name, Percent: l.Percent, Text: l.Text})
		}
	}

	waka.mu.Lock()
	waka.today, waka.week, waka.top = todayText, weekText, top
	waka.lastErr, waka.fetched = nil, time.Now()
	waka.mu.Unlock()
	return nil
}

type errHTTP string

func (e errHTTP) Error() string { return "WakaTime API returned " + string(e) }
