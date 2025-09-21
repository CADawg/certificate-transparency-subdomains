class SubdomainDiscovery {
    constructor() {
        this.form = document.getElementById('searchForm');
        this.domainInput = document.getElementById('domain');
        this.searchBtn = document.getElementById('searchBtn');
        this.resultsDiv = document.getElementById('results');
        this.btnText = document.querySelector('.btn-text');
        this.btnSpinner = document.querySelector('.btn-spinner');

        this.currentEventSource = null;
        this.foundSubdomains = new Set();
        this.isSearching = false;

        this.init();
    }

    init() {
        this.form.addEventListener('submit', (e) => this.handleSubmit(e));
        this.domainInput.addEventListener('input', () => this.validateInput());
    }

    validateInput() {
        const domain = this.domainInput.value.trim();
        const isValid = domain && /^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]*\.([a-zA-Z]{2,}|[a-zA-Z]{2,}\.[a-zA-Z]{2,})$/.test(domain);
        this.searchBtn.disabled = !isValid || this.isSearching;
    }

    async handleSubmit(event) {
        event.preventDefault();

        if (this.isSearching) {
            this.stopSearch();
            return;
        }

        const domain = this.domainInput.value.trim();
        if (!domain) return;

        this.startSearch(domain);
    }

    startSearch(domain) {
        this.isSearching = true;
        this.foundSubdomains.clear();
        this.updateUI();
        this.showLoadingState(domain);

        // Close any existing connection
        if (this.currentEventSource) {
            this.currentEventSource.close();
        }

        // Create new EventSource for streaming
        this.currentEventSource = new EventSource('/api/stream', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ domain: domain })
        });

        // Since EventSource doesn't support POST directly, we'll use fetch with streaming
        this.streamResults(domain);
    }

    async streamResults(domain) {
        try {
            const response = await fetch('/api/stream', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Accept': 'text/event-stream'
                },
                body: JSON.stringify({ domain: domain })
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const reader = response.body.getReader();
            const decoder = new TextDecoder();
            let buffer = '';

            while (true) {
                const { done, value } = await reader.read();

                if (done) break;

                buffer += decoder.decode(value, { stream: true });
                const lines = buffer.split('\n');
                buffer = lines.pop(); // Keep incomplete line in buffer

                for (const line of lines) {
                    this.processStreamLine(line);
                }
            }

        } catch (error) {
            this.showError(`Connection error: ${error.message}`);
        } finally {
            this.stopSearch();
        }
    }

    processStreamLine(line) {
        if (line.startsWith('data: ')) {
            const data = line.substring(6);

            if (data === '{"message": "Search completed"}') {
                this.showCompletionState();
                return;
            }

            try {
                const result = JSON.parse(data);
                if (result.subdomain && result.source) {
                    this.addSubdomain(result);
                }
            } catch (e) {
                console.warn('Failed to parse stream data:', data);
            }
        } else if (line.startsWith('event: complete')) {
            this.showCompletionState();
        }
    }

    addSubdomain(result) {
        if (this.foundSubdomains.has(result.subdomain)) {
            return;
        }

        this.foundSubdomains.add(result.subdomain);

        // Create results container if it doesn't exist
        if (!document.querySelector('.results-container')) {
            this.createResultsContainer();
        }

        const container = document.querySelector('.results-container');
        const item = this.createSubdomainItem(result);
        container.appendChild(item);

        // Update count
        this.updateCount();

        // Scroll to show new item
        item.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }

    createResultsContainer() {
        this.resultsDiv.innerHTML = `
            <div class="status-bar">
                <div class="status-text">
                    <span class="search-status">Searching...</span>
                    <div class="progress-indicator">
                        <div class="progress-dot"></div>
                        <div class="progress-dot"></div>
                        <div class="progress-dot"></div>
                    </div>
                </div>
                <div class="status-count">0 found</div>
            </div>
            <div class="results-container"></div>
        `;
    }

    createSubdomainItem(result) {
        const item = document.createElement('div');
        item.className = 'subdomain-item';

        const sourceClass = result.source === 'Certificate Transparency' ? 'source-ct' : 'source-dns';
        const sourceText = result.source === 'Certificate Transparency' ? 'CT Logs' : 'DNS Enum';

        item.innerHTML = `
            <div class="subdomain-name">${this.escapeHtml(result.subdomain)}</div>
            <div class="subdomain-source ${sourceClass}">${sourceText}</div>
        `;

        return item;
    }

    updateCount() {
        const countElement = document.querySelector('.status-count');
        if (countElement) {
            const count = this.foundSubdomains.size;
            countElement.textContent = `${count} found`;
        }
    }

    showLoadingState(domain) {
        this.resultsDiv.innerHTML = `
            <div class="loading-message">
                🔍 Searching for subdomains of <strong>${this.escapeHtml(domain)}</strong>...
                <div style="margin-top: 1rem;">
                    <div class="progress-indicator">
                        <div class="progress-dot"></div>
                        <div class="progress-dot"></div>
                        <div class="progress-dot"></div>
                    </div>
                </div>
            </div>
        `;
    }

    showCompletionState() {
        const statusText = document.querySelector('.search-status');
        const progressIndicator = document.querySelector('.progress-indicator');

        if (statusText) {
            statusText.textContent = '✅ Search completed';
        }

        if (progressIndicator) {
            progressIndicator.style.display = 'none';
        }

        if (this.foundSubdomains.size === 0) {
            this.resultsDiv.innerHTML = `
                <div class="loading-message">
                    😔 No subdomains found for this domain.
                    <div style="margin-top: 0.5rem; font-weight: normal; color: var(--text-secondary);">
                        Try a different domain or check if the domain exists.
                    </div>
                </div>
            `;
        }
    }

    showError(message) {
        this.resultsDiv.innerHTML = `
            <div class="error-message">
                <strong>❌ Error:</strong> ${this.escapeHtml(message)}
            </div>
        `;
    }

    stopSearch() {
        this.isSearching = false;

        if (this.currentEventSource) {
            this.currentEventSource.close();
            this.currentEventSource = null;
        }

        this.updateUI();
    }

    updateUI() {
        if (this.isSearching) {
            this.btnText.textContent = 'Stop Search';
            this.btnSpinner.style.display = 'none';
            this.searchBtn.style.background = 'var(--error-color)';
        } else {
            this.btnText.textContent = 'Search Subdomains';
            this.btnSpinner.style.display = 'none';
            this.searchBtn.style.background = 'var(--primary-color)';
        }

        this.validateInput();
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize the application when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new SubdomainDiscovery();
});