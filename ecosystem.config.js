module.exports = {
  apps: [{
    name: 'subdomain-tool',
    script: './certificate-transparency-subdomains',
    instances: 1,
    autorestart: true,
    watch: false,
    max_memory_restart: '1G'
  }]
};