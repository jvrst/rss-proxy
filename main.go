package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// Config holds named profiles loaded from config.json.
type Config struct {
	Upstream string              `json:"upstream"`
	Profiles map[string][]string `json:"profiles"`
}

var config Config

func loadConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(&config)
}

type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title         string `xml:"title"`
	Link          string `xml:"link"`
	Description   string `xml:"description"`
	LastBuildDate string `xml:"lastBuildDate"`
	Items         []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
	Link        string `xml:"link"`
}

// itemMatchesServices returns true if the item relates to any of the given services.
// Matching is case-insensitive and checks the "Affected components" line
// and any "ServiceName: ..." prefix in the description.
func itemMatchesServices(item Item, services []string) bool {
	desc := strings.ToLower(item.Description)
	for _, svc := range services {
		svcLower := strings.ToLower(strings.TrimSpace(svc))
		if svcLower == "" {
			continue
		}
		// Check "⚠️ Affected components: ..." line
		if idx := strings.Index(desc, "affected components:"); idx != -1 {
			components := desc[idx+len("affected components:"):]
			if nl := strings.Index(components, "\n"); nl != -1 {
				components = components[:nl]
			}
			for _, comp := range strings.Split(components, ",") {
				if strings.TrimSpace(comp) == svcLower {
					return true
				}
			}
		}
		// Check "ServiceName: ..." prefix on any description line
		for _, line := range strings.Split(desc, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), svcLower+":") {
				return true
			}
		}
	}
	return false
}

func fetchAndFilter(services []string) ([]byte, error) {
	resp, err := http.Get(config.Upstream)
	if err != nil {
		return nil, fmt.Errorf("fetching upstream: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading upstream body: %w", err)
	}

	var feed RSS
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parsing RSS: %w", err)
	}

	// No filter → return full feed as-is
	if len(services) == 0 {
		return body, nil
	}

	filtered := make([]Item, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		if itemMatchesServices(item, services) {
			filtered = append(filtered, item)
		}
	}
	feed.Channel.Items = filtered

	out, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling RSS: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}

// resolveServices expands profile names and raw service names into a deduplicated list.
func resolveServices(profileParam, servicesParam string) ([]string, error) {
	seen := map[string]struct{}{}
	var result []string

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, s)
		}
	}

	for _, name := range strings.Split(profileParam, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		services, ok := config.Profiles[name]
		if !ok {
			return nil, fmt.Errorf("unknown profile %q", name)
		}
		for _, s := range services {
			add(s)
		}
	}

	for _, s := range strings.Split(servicesParam, ",") {
		add(s)
	}

	return result, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	profileParam := r.URL.Query().Get("profile")
	servicesParam := r.URL.Query().Get("services")

	services, err := resolveServices(profileParam, servicesParam)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := fetchAndFilter(services)
	if err != nil {
		log.Printf("error: %v", err)
		http.Error(w, "failed to fetch feed", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write(data)
}

func main() {
	configPath := os.Getenv("CONFIG")
	if configPath == "" {
		configPath = "config.json"
	}
	if err := loadConfig(configPath); err != nil {
		log.Fatalf("loading config: %v", err)
	}
	if config.Upstream == "" {
		log.Fatal("config: missing upstream URL")
	}
	log.Printf("loaded %d profile(s): %s", len(config.Profiles), profileNames())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/feed", handler)

	log.Printf("RSS proxy listening on :%s", port)
	log.Printf("Usage: /feed?profile=vehicles  or  /feed?services=Tesla,Volvo  or  both")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func profileNames() string {
	names := make([]string, 0, len(config.Profiles))
	for k := range config.Profiles {
		names = append(names, k)
	}
	return strings.Join(names, ", ")
}
