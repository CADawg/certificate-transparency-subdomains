package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type CTEntry struct {
	Name string `json:"name_value"`
}

func getCTSubdomains(domain string) []string {
	var subdomains []string
	
	url := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", domain)
	
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("Error fetching CT logs: %v\n", err)
		return subdomains
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("CT API returned status: %d\n", resp.StatusCode)
		return subdomains
	}
	
	var entries []CTEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		fmt.Printf("Error decoding CT response: %v\n", err)
		return subdomains
	}
	
	seen := make(map[string]bool)
	domainPattern := regexp.MustCompile(`^([a-zA-Z0-9\-_\*\.]+\.` + regexp.QuoteMeta(domain) + `)$`)
	
	for _, entry := range entries {
		names := strings.Split(entry.Name, "\n")
		for _, name := range names {
			name = strings.TrimSpace(strings.ToLower(name))
			
			if name == "" || name == domain {
				continue
			}
			
			if strings.HasPrefix(name, "*.") {
				name = name[2:]
			}
			
			if domainPattern.MatchString(name) && !seen[name] {
				seen[name] = true
				subdomains = append(subdomains, name)
			}
		}
	}
	
	return subdomains
}