package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gorilla/mux"
)

type SubdomainResult struct {
	Subdomain string `json:"subdomain"`
	Source    string `json:"source"`
}

type SearchResponse struct {
	Domain     string            `json:"domain"`
	Subdomains []SubdomainResult `json:"subdomains"`
	Error      string            `json:"error,omitempty"`
}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/", indexHandler).Methods("GET")
	r.HandleFunc("/blog", blogHandler).Methods("GET")
	r.HandleFunc("/blog/", blogHandler).Methods("GET")
	r.HandleFunc("/blog/{slug}", articleHandler).Methods("GET")
	r.HandleFunc("/api/search", searchHandler).Methods("POST")
	r.HandleFunc("/api/stream", streamHandler).Methods("POST")

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	fmt.Println("Starting subdomain discovery server on :9382")
	fmt.Println("Visit http://localhost:9382 to use the tool")

	log.Fatal(http.ListenAndServe(":9382", r))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join("static", "index.html"))
}

func blogHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join("static", "blog.html"))
}

func articleHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	slug := vars["slug"]

	articlePath := filepath.Join("static", "articles", slug+".html")
	if _, err := filepath.Abs(articlePath); err != nil {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, articlePath)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	domain := strings.TrimSpace(req.Domain)
	if domain == "" {
		sendErrorResponse(w, "Domain is required")
		return
	}

	if !isValidDomain(domain) {
		sendErrorResponse(w, "Invalid domain format")
		return
	}

	subdomains := discoverSubdomains(domain)

	response := SearchResponse{
		Domain:     domain,
		Subdomains: subdomains,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func sendErrorResponse(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(SearchResponse{Error: message})
}

func isValidDomain(domain string) bool {
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]*\.([a-zA-Z]{2,}|[a-zA-Z]{2,}\.[a-zA-Z]{2,})$`)
	return domainRegex.MatchString(domain)
}

func streamHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	domain := strings.TrimSpace(req.Domain)
	if domain == "" {
		http.Error(w, "Domain is required", http.StatusBadRequest)
		return
	}

	if !isValidDomain(domain) {
		http.Error(w, "Invalid domain format", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	resultChan := make(chan SubdomainResult, 100)
	done := make(chan bool, 2)
	subdomainSet := make(map[string]bool)
	var mutex sync.Mutex

	go func() {
		ctSubdomains := getCTSubdomains(domain)
		for _, subdomain := range ctSubdomains {
			mutex.Lock()
			if !subdomainSet[subdomain] {
				subdomainSet[subdomain] = true
				resultChan <- SubdomainResult{
					Subdomain: subdomain,
					Source:    "Certificate Transparency",
				}
			}
			mutex.Unlock()
		}
		done <- true
	}()

	go func() {
		dnsSubdomains := getDNSSubdomainsStreaming(domain, resultChan, &subdomainSet, &mutex)
		_ = dnsSubdomains
		done <- true
	}()

	go func() {
		completedSources := 0
		for completedSources < 2 {
			<-done
			completedSources++
		}
		close(resultChan)
	}()

	for result := range resultChan {
		data, _ := json.Marshal(result)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	fmt.Fprintf(w, "event: complete\ndata: {\"message\": \"Search completed\"}\n\n")
	flusher.Flush()
}

func discoverSubdomains(domain string) []SubdomainResult {
	var results []SubdomainResult
	var mutex sync.Mutex
	var wg sync.WaitGroup

	subdomainSet := make(map[string]bool)

	wg.Add(2)

	go func() {
		defer wg.Done()
		ctSubdomains := getCTSubdomains(domain)
		mutex.Lock()
		for _, subdomain := range ctSubdomains {
			if !subdomainSet[subdomain] {
				subdomainSet[subdomain] = true
				results = append(results, SubdomainResult{
					Subdomain: subdomain,
					Source:    "Certificate Transparency",
				})
			}
		}
		mutex.Unlock()
	}()

	go func() {
		defer wg.Done()
		dnsSubdomains := getDNSSubdomains(domain)
		mutex.Lock()
		for _, subdomain := range dnsSubdomains {
			if !subdomainSet[subdomain] {
				subdomainSet[subdomain] = true
				results = append(results, SubdomainResult{
					Subdomain: subdomain,
					Source:    "DNS Enumeration",
				})
			}
		}
		mutex.Unlock()
	}()

	wg.Wait()

	return results
}
