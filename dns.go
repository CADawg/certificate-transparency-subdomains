package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

func getDNSSubdomains(domain string) []string {
	var subdomains []string

	commonSubdomains := []string{
		"www", "mail", "ftp", "admin", "api", "app", "blog", "cdn", "dev", "docs",
		"forum", "help", "m", "mobile", "news", "shop", "stage", "staging", "test",
		"webmail", "secure", "login", "cpanel", "whm", "mysql", "phpmyadmin",
		"ns1", "ns2", "ns3", "ns4", "mx", "mx1", "mx2", "smtp", "pop", "imap",
		"support", "portal", "assets", "static", "img", "images", "media", "files",
		"download", "downloads", "upload", "uploads", "backup", "backups",
		"dashboard", "control", "panel", "status", "stats", "analytics", "reports",
		"crm", "erp", "hr", "finance", "accounting", "billing", "payment", "payments",
		"store", "cart", "checkout", "order", "orders", "product", "products",
		"search", "find", "directory", "catalog", "inventory", "demo", "beta",
		"alpha", "preview", "old", "new", "v1", "v2", "v3", "vpn", "ssh", "sftp",
		"git", "svn", "repo", "repository", "ci", "build", "jenkins", "gitlab",
		"github", "bitbucket", "redmine", "jira", "wiki", "confluence", "sharepoint",
		"intranet", "extranet", "partner", "partners", "client", "clients", "customer",
		"customers", "vendor", "vendors", "supplier", "suppliers", "affiliate",
		"affiliates", "reseller", "resellers", "dealer", "dealers", "distributor",
		"distributors", "agent", "agents", "rep", "reps", "sales", "marketing",
		"promo", "promotion", "promotions", "campaign", "campaigns", "event", "events",
		"conference", "webinar", "training", "education", "learn", "learning",
		"course", "courses", "class", "classes", "school", "university", "college",
		"student", "students", "teacher", "teachers", "faculty", "staff", "employee",
		"employees", "member", "members", "user", "users", "guest", "guests",
		"public", "private", "internal", "external", "local", "remote", "cloud",
		"server", "servers", "host", "hosts", "node", "nodes", "cluster", "clusters",
		"db", "database", "databases", "data", "backup", "mirror", "cache", "proxy",
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * 2000,
			}
			return d.DialContext(ctx, network, address)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, subdomain := range commonSubdomains {
		fullDomain := fmt.Sprintf("%s.%s", subdomain, domain)

		_, err := resolver.LookupHost(ctx, fullDomain)
		if err == nil {
			subdomains = append(subdomains, fullDomain)
		}
	}

	txtRecords, err := resolver.LookupTXT(ctx, domain)
	if err == nil {
		for _, record := range txtRecords {
			if strings.Contains(strings.ToLower(record), "subdomain") {
				parts := strings.Fields(record)
				for _, part := range parts {
					if strings.Contains(part, "."+domain) {
						cleanDomain := strings.ToLower(strings.TrimSpace(part))
						if isValidSubdomain(cleanDomain, domain) {
							subdomains = append(subdomains, cleanDomain)
						}
					}
				}
			}
		}
	}

	return subdomains
}

func getDNSSubdomainsStreaming(domain string, resultChan chan<- SubdomainResult, subdomainSet *map[string]bool, mutex *sync.Mutex) []string {
	var subdomains []string

	commonSubdomains := []string{
		"www", "mail", "ftp", "admin", "api", "app", "blog", "cdn", "dev", "docs",
		"forum", "help", "m", "mobile", "news", "shop", "stage", "staging", "test",
		"webmail", "secure", "login", "cpanel", "whm", "mysql", "phpmyadmin",
		"ns1", "ns2", "ns3", "ns4", "mx", "mx1", "mx2", "smtp", "pop", "imap",
		"support", "portal", "assets", "static", "img", "images", "media", "files",
		"download", "downloads", "upload", "uploads", "backup", "backups",
		"dashboard", "control", "panel", "status", "stats", "analytics", "reports",
		"crm", "erp", "hr", "finance", "accounting", "billing", "payment", "payments",
		"store", "cart", "checkout", "order", "orders", "product", "products",
		"search", "find", "directory", "catalog", "inventory", "demo", "beta",
		"alpha", "preview", "old", "new", "v1", "v2", "v3", "vpn", "ssh", "sftp",
		"git", "svn", "repo", "repository", "ci", "build", "jenkins", "gitlab",
		"github", "bitbucket", "redmine", "jira", "wiki", "confluence", "sharepoint",
		"intranet", "extranet", "partner", "partners", "client", "clients", "customer",
		"customers", "vendor", "vendors", "supplier", "suppliers", "affiliate",
		"affiliates", "reseller", "resellers", "dealer", "dealers", "distributor",
		"distributors", "agent", "agents", "rep", "reps", "sales", "marketing",
		"promo", "promotion", "promotions", "campaign", "campaigns", "event", "events",
		"conference", "webinar", "training", "education", "learn", "learning",
		"course", "courses", "class", "classes", "school", "university", "college",
		"student", "students", "teacher", "teachers", "faculty", "staff", "employee",
		"employees", "member", "members", "user", "users", "guest", "guests",
		"public", "private", "internal", "external", "local", "remote", "cloud",
		"server", "servers", "host", "hosts", "node", "nodes", "cluster", "clusters",
		"db", "database", "databases", "data", "backup", "mirror", "cache", "proxy",
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * 1000,
			}
			return d.DialContext(ctx, network, address)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	semaphore := make(chan struct{}, 20)
	var wg sync.WaitGroup

	for _, subdomain := range commonSubdomains {
		wg.Add(1)
		go func(sub string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			fullDomain := fmt.Sprintf("%s.%s", sub, domain)

			_, err := resolver.LookupHost(ctx, fullDomain)
			if err == nil {
				mutex.Lock()
				if !(*subdomainSet)[fullDomain] {
					(*subdomainSet)[fullDomain] = true
					subdomains = append(subdomains, fullDomain)
					select {
					case resultChan <- SubdomainResult{
						Subdomain: fullDomain,
						Source:    "DNS Enumeration",
					}:
					default:
					}
				}
				mutex.Unlock()
			}
		}(subdomain)
	}

	wg.Wait()

	txtRecords, err := resolver.LookupTXT(ctx, domain)
	if err == nil {
		for _, record := range txtRecords {
			if strings.Contains(strings.ToLower(record), "subdomain") {
				parts := strings.Fields(record)
				for _, part := range parts {
					if strings.Contains(part, "."+domain) {
						cleanDomain := strings.ToLower(strings.TrimSpace(part))
						if isValidSubdomain(cleanDomain, domain) {
							mutex.Lock()
							if !(*subdomainSet)[cleanDomain] {
								(*subdomainSet)[cleanDomain] = true
								subdomains = append(subdomains, cleanDomain)
								select {
								case resultChan <- SubdomainResult{
									Subdomain: cleanDomain,
									Source:    "DNS Enumeration",
								}:
								default:
								}
							}
							mutex.Unlock()
						}
					}
				}
			}
		}
	}

	return subdomains
}

func isValidSubdomain(subdomain, baseDomain string) bool {
	if subdomain == "" || subdomain == baseDomain {
		return false
	}

	if !strings.HasSuffix(subdomain, "."+baseDomain) {
		return false
	}

	parts := strings.Split(subdomain, ".")
	if len(parts) <= len(strings.Split(baseDomain, ".")) {
		return false
	}

	return true
}
