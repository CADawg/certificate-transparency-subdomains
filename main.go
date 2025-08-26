package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

const indexHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>Subdomain Discovery Tool</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .container { max-width: 800px; margin: 0 auto; }
        input[type="text"] { padding: 10px; width: 300px; margin-right: 10px; }
        button { padding: 10px 20px; background: #007cba; color: white; border: none; cursor: pointer; }
        button:hover { background: #005a87; }
        .results { margin-top: 30px; }
        .subdomain { padding: 5px; margin: 2px 0; background: #f5f5f5; border-radius: 3px; }
        .error { color: red; padding: 10px; background: #ffe6e6; border-radius: 3px; }
        .loading { color: #007cba; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Subdomain Discovery Tool</h1>
        <p>Enter a domain to discover subdomains using Certificate Transparency logs and DNS enumeration.</p>
        
        <form onsubmit="searchSubdomains(event)">
            <input type="text" id="domain" placeholder="example.com" required>
            <button type="submit">Search Subdomains</button>
        </form>
        
        <div id="results" class="results"></div>
    </div>

    <script>
        async function searchSubdomains(event) {
            event.preventDefault();
            const domain = document.getElementById('domain').value.trim();
            const resultsDiv = document.getElementById('results');
            
            resultsDiv.innerHTML = '<div class="loading">Searching for subdomains...</div>';
            
            try {
                const response = await fetch('/api/search', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ domain: domain })
                });
                
                const data = await response.json();
                
                if (data.error) {
                    resultsDiv.innerHTML = '<div class="error">' + data.error + '</div>';
                    return;
                }
                
                if (data.subdomains.length === 0) {
                    resultsDiv.innerHTML = '<div>No subdomains found for ' + domain + '</div>';
                    return;
                }
                
                let html = '<h3>Found ' + data.subdomains.length + ' subdomains for ' + domain + ':</h3>';
                data.subdomains.forEach(function(result) {
                    html += '<div class="subdomain"><strong>' + result.subdomain + '</strong> <small>(' + result.source + ')</small></div>';
                });
                
                resultsDiv.innerHTML = html;
            } catch (error) {
                resultsDiv.innerHTML = '<div class="error">Error: ' + error.message + '</div>';
            }
        }
    </script>
</body>
</html>
`

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/", indexHandler).Methods("GET")
	r.HandleFunc("/api/search", searchHandler).Methods("POST")

	fmt.Println("Starting subdomain discovery server on :8080")
	fmt.Println("Visit http://localhost:9382 to use the tool")

	log.Fatal(http.ListenAndServe(":9382", r))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, indexHTML)
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
