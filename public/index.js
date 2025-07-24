// Analytics Dashboard JavaScript
class GoPayAnalytics {
    constructor() {
        this.providers = ['iyzico', 'stripe', 'ozanpay', 'paycell', 'papara', 'nkolay', 'paytr', 'payu'];
        this.trendsChart = null;
        this.distributionChart = null;
        this.currentFilters = {
            tenant_id: 'all',
            provider_id: 'all',
            environment: 'all',
            hours: '24'
        };
        this.authToken = localStorage.getItem('authToken');
        this.init();
    }

    getAuthHeaders() {
        const headers = {
            'Content-Type': 'application/json'
        };
        if (this.authToken) {
            headers['Authorization'] = `Bearer ${this.authToken}`;
        }
        return headers;
    }

    async authenticatedFetch(url, options = {}) {
        const response = await fetch(url, {
            ...options,
            headers: {
                ...this.getAuthHeaders(),
                ...options.headers
            }
        });

        // If unauthorized, redirect to login
        if (response.status === 401) {
            localStorage.removeItem('authToken');
            window.location.href = '/login';
            return null;
        }

        return response;
    }

    async init() {
        // Check authentication first
        const isAuthenticated = await this.checkAuthentication();
        if (!isAuthenticated) {
            return; // Will redirect to login
        }

        this.initializeFilters();
        this.initializeLogout();
        await this.loadFilterOptions();
        await this.loadDashboardData();
        this.initCharts();
        this.startRealTimeUpdates();
    }

    async checkAuthentication() {
        const token = localStorage.getItem('authToken');
        
        if (!token) {
            this.redirectToLogin();
            return false;
        }

        try {
            const response = await fetch('/v1/auth/validate', {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                }
            });

            if (!response.ok) {
                this.redirectToLogin();
                return false;
            }

            const data = await response.json();
            
            if (!data.success) {
                this.redirectToLogin();
                return false;
            }

            // Check if tenant_id is 1
            if (data.data.tenant_id !== "1") {
                alert('Access denied. Only tenant with ID 1 can access the dashboard.');
                this.redirectToLogin();
                return false;
            }

            // Update the token in case it was refreshed
            this.authToken = token;
            
            // Hide loading screen and show dashboard
            this.showDashboard();
            return true;

        } catch (error) {
            console.error('Authentication check failed:', error);
            this.redirectToLogin();
            return false;
        }
    }

    showDashboard() {
        const loadingScreen = document.getElementById('authLoadingScreen');
        const dashboardContainer = document.getElementById('dashboardContainer');
        
        if (loadingScreen) {
            loadingScreen.style.display = 'none';
        }
        if (dashboardContainer) {
            dashboardContainer.style.display = 'block';
        }
    }

    redirectToLogin() {
        localStorage.removeItem('authToken');
        window.location.href = '/login';
    }

    initializeLogout() {
        const logoutBtn = document.getElementById('logoutBtn');
        if (logoutBtn) {
            logoutBtn.addEventListener('click', () => {
                this.logout();
            });
        }
    }

    logout() {
        // Clear stored tokens
        localStorage.removeItem('authToken');
        
        // Redirect to login page
        window.location.href = '/login';
    }

    initializeFilters() {
        // Add event listeners to filter controls
        const tenantFilter = document.getElementById('tenantFilter');
        const providerFilter = document.getElementById('providerFilter');
        const environmentFilter = document.getElementById('environmentFilter');
        const hoursFilter = document.getElementById('hoursFilter');
        const refreshButton = document.getElementById('refreshButton');

        if (tenantFilter) {
            tenantFilter.addEventListener('change', (e) => {
                this.currentFilters.tenant_id = e.target.value;
                this.onFiltersChanged();
            });
        }

        if (providerFilter) {
            providerFilter.addEventListener('change', (e) => {
                this.currentFilters.provider_id = e.target.value;
                this.onFiltersChanged();
            });
        }

        if (environmentFilter) {
            environmentFilter.addEventListener('change', (e) => {
                this.currentFilters.environment = e.target.value;
                this.onFiltersChanged();
            });
        }

        if (hoursFilter) {
            hoursFilter.addEventListener('change', (e) => {
                this.currentFilters.hours = e.target.value;
                this.onFiltersChanged();
            });
        }

        if (refreshButton) {
            refreshButton.addEventListener('click', () => {
                this.refreshDashboard();
            });
        }
    }

    async loadFilterOptions() {
        try {
            // Load tenants from API
            await this.loadTenantOptions();
            
            // Load providers from API
            await this.loadProviderOptions();
        } catch (error) {
            console.error('Error loading filter options:', error);
        }
    }

    async loadTenantOptions() {
        try {
            const response = await this.authenticatedFetch('/v1/analytics/tenants');
            if (response && response.ok) {
                const data = await response.json();
                if (data.success && data.data) {
                    const tenantFilter = document.getElementById('tenantFilter');
                    if (tenantFilter) {
                        // Clear existing options except "All Tenants"
                        tenantFilter.innerHTML = '<option value="all">All Tenants</option>';
                        
                        // Add dynamic tenant options
                        data.data.forEach(tenant => {
                            const option = document.createElement('option');
                            option.value = tenant.id;
                            option.textContent = `${tenant.name} (ID: ${tenant.id})`;
                            tenantFilter.appendChild(option);
                        });
                    }
                }
            }
        } catch (error) {
            console.error('Error loading tenant options:', error);
            // Keep default options if API fails
        }
    }

    async loadProviderOptions() {
        try {
            const response = await this.authenticatedFetch('/v1/analytics/providers/list');
            if (response && response.ok) {
                const data = await response.json();
                if (data.success && data.data) {
                    const providerFilter = document.getElementById('providerFilter');
                    if (providerFilter) {
                        // Clear existing options except "All Providers"
                        providerFilter.innerHTML = '<option value="all">All Providers</option>';
                        
                        // Add dynamic provider options
                        data.data.forEach(provider => {
                            const option = document.createElement('option');
                            option.value = provider.id;
                            option.textContent = provider.name;
                            providerFilter.appendChild(option);
                        });
                    }
                }
            }
        } catch (error) {
            console.error('Error loading provider options:', error);
            // Keep default options if API fails
        }
    }

    buildFilterParams() {
        const params = new URLSearchParams();
        
        Object.entries(this.currentFilters).forEach(([key, value]) => {
            if (value && value !== 'all') {
                params.append(key, value);
            }
        });

        return params.toString();
    }

    async onFiltersChanged() {
        // Show loading state
        this.showLoadingState();
        
        // Reload data with new filters
        await this.loadDashboardData();
        await this.initCharts();
        
        // Hide loading state
        this.hideLoadingState();
    }

    async refreshDashboard() {
        const refreshButton = document.getElementById('refreshButton');
        if (refreshButton) {
            refreshButton.classList.add('loading');
            refreshButton.disabled = true;
        }

        try {
            await this.loadDashboardData();
            await this.initCharts();
            await this.loadProviderStatus();
            await this.loadRecentActivity();
        } finally {
            if (refreshButton) {
                refreshButton.classList.remove('loading');
                refreshButton.disabled = false;
            }
        }
    }

    showLoadingState() {
        // Add loading class to main stats
        document.querySelectorAll('.stat-value').forEach(el => {
            el.style.opacity = '0.5';
        });
    }

    hideLoadingState() {
        // Remove loading class from main stats
        document.querySelectorAll('.stat-value').forEach(el => {
            el.style.opacity = '1';
        });
    }

    async loadDashboardData() {
        try {
            // Load dashboard stats from analytics API with filters
            const filterParams = this.buildFilterParams();
            const dashboardResponse = await this.authenticatedFetch(`/v1/analytics/dashboard?${filterParams}`);
            
            if (dashboardResponse && dashboardResponse.ok) {
                const dashboardData = await dashboardResponse.json();
                if (dashboardData.success) {
                    this.updateStats(dashboardData.data);
                }
            }

            // Load provider status and recent activity with filters
            await this.loadProviderStatus();
            await this.loadRecentActivity();

        } catch (error) {
            console.error('Error loading dashboard data:', error);
            // Fallback to placeholder data (FAKE DATA for demo)
            this.updateStats({
                totalPayments: Math.floor(Math.random() * 10000) + 5000,
                successRate: (95 + Math.random() * 5).toFixed(2),
                totalVolume: (Math.random() * 1000000).toFixed(2),
                avgResponseTime: (200 + Math.random() * 100).toFixed(2),
                totalPaymentsChange: "+12.5% from yesterday",
                successRateChange: "+0.8% from yesterday", 
                totalVolumeChange: "+18.2% from yesterday",
                avgResponseChange: "-15ms from yesterday",
                activeTenants: 3,
                activeProviders: 5,
                environment: this.currentFilters.environment
            });
        }
    }

    updateStats(stats) {
        document.getElementById('totalPayments').textContent = stats.totalPayments.toLocaleString();
        document.getElementById('successRate').textContent = parseFloat(stats.successRate).toFixed(2) + '%';
        document.getElementById('totalVolume').textContent = '₺' + parseFloat(stats.totalVolume).toLocaleString();
        document.getElementById('avgResponseTime').textContent = parseFloat(stats.avgResponseTime).toFixed(2) + 'ms';

        // Update change indicators
        document.getElementById('totalPaymentsChange').textContent = stats.totalPaymentsChange || '+12.5% from yesterday';
        document.getElementById('successRateChange').textContent = stats.successRateChange || '+0.8% from yesterday';
        document.getElementById('totalVolumeChange').textContent = stats.totalVolumeChange || '+18.2% from yesterday';
        document.getElementById('avgResponseTimeChange').textContent = stats.avgResponseChange || '-15ms from yesterday';

        // Update title to show current filter context
        this.updateDashboardTitle(stats);
    }

    updateDashboardTitle(stats) {
        const subtitle = document.querySelector('.header-subtitle');
        if (subtitle) {
            let context = 'Multi-Tenant Payment Analytics Dashboard';
            
            if (this.currentFilters.tenant_id !== 'all') {
                context += ` - Tenant ${this.currentFilters.tenant_id}`;
            }
            
            if (this.currentFilters.provider_id !== 'all') {
                context += ` - ${this.currentFilters.provider_id}`;
            }
            
            if (this.currentFilters.environment !== 'all') {
                context += ` - ${this.currentFilters.environment}`;
            }
            
            subtitle.textContent = context;
        }
    }

    async initCharts() {
        // Destroy existing charts if they exist
        if (this.trendsChart) {
            this.trendsChart.destroy();
            this.trendsChart = null;
        }
        if (this.distributionChart) {
            this.distributionChart.destroy();
            this.distributionChart = null;
        }

        // Payment Trends Chart
        const trendsCtx = document.getElementById('paymentTrendsChart').getContext('2d');
        
        try {
            const filterParams = this.buildFilterParams();
            const response = await this.authenticatedFetch(`/v1/analytics/trends?${filterParams}`);
            
            if (response && response.ok) {
                const data = await response.json();
                if (data.success && data.data.datasets) {
                    this.trendsChart = new Chart(trendsCtx, {
                        type: 'line',
                        data: {
                            labels: data.data.labels,
                            datasets: data.data.datasets.map(dataset => ({
                                ...dataset,
                                tension: 0.4
                            }))
                        },
                        options: {
                            responsive: true,
                            maintainAspectRatio: false,
                            scales: {
                                y: {
                                    beginAtZero: true
                                }
                            }
                        }
                    });
                } else {
                    // Fallback to static data
                    this.createFallbackTrendsChart(trendsCtx);
                }
            } else {
                this.createFallbackTrendsChart(trendsCtx);
            }
        } catch (error) {
            console.error('Error loading trends data:', error);
            this.createFallbackTrendsChart(trendsCtx);
        }

        // Provider Distribution Chart
        const distributionCtx = document.getElementById('providerDistributionChart').getContext('2d');
        
        try {
            const filterParams = this.buildFilterParams();
            const response = await this.authenticatedFetch(`/v1/analytics/providers?${filterParams}`);
            
            if (response && response.ok) {
                const data = await response.json();
                if (data.success && data.data) {
                    const providers = data.data.slice(0, 5); // Top 5 providers
                    this.distributionChart = new Chart(distributionCtx, {
                        type: 'doughnut',
                        data: {
                            labels: providers.map(p => p.name),
                            datasets: [{
                                data: providers.map(p => p.transactions),
                                backgroundColor: [
                                    '#667eea',
                                    '#764ba2',
                                    '#f093fb',
                                    '#f5576c',
                                    '#4facfe'
                                ]
                            }]
                        },
                        options: {
                            responsive: true,
                            maintainAspectRatio: false,
                            plugins: {
                                legend: {
                                    position: 'bottom'
                                }
                            }
                        }
                    });
                } else {
                    this.createFallbackDistributionChart(distributionCtx);
                }
            } else {
                this.createFallbackDistributionChart(distributionCtx);
            }
        } catch (error) {
            console.error('Error loading provider distribution:', error);
            this.createFallbackDistributionChart(distributionCtx);
        }
    }

    createFallbackTrendsChart(ctx) {
        const trendsData = this.generateTrendData();
        this.trendsChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: trendsData.labels,
                datasets: [{
                    label: 'Successful Payments',
                    data: trendsData.success,
                    borderColor: '#10B981',
                    backgroundColor: 'rgba(16, 185, 129, 0.1)',
                    tension: 0.4
                }, {
                    label: 'Failed Payments',
                    data: trendsData.failed,
                    borderColor: '#EF4444',
                    backgroundColor: 'rgba(239, 68, 68, 0.1)',
                    tension: 0.4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true
                    }
                }
            }
        });
    }

    createFallbackDistributionChart(ctx) {
        // Filter providers based on current filter
        let providers = ['İyzico', 'Stripe', 'OzanPay', 'Paycell', 'Others'];
        let data = [35, 25, 20, 15, 5];
        
        if (this.currentFilters.provider_id !== 'all') {
            providers = [this.currentFilters.provider_id];
            data = [100];
        }
        
        this.distributionChart = new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: providers,
                datasets: [{
                    data: data,
                    backgroundColor: [
                        '#667eea',
                        '#764ba2',
                        '#f093fb',
                        '#f5576c',
                        '#4facfe'
                    ]
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'bottom'
                    }
                }
            }
        });
    }

    generateTrendData() {
        const labels = [];
        const success = [];
        const failed = [];
        const hours = parseInt(this.currentFilters.hours);
        
        for (let i = hours - 1; i >= 0; i--) {
            labels.push(i === 0 ? 'Now' : `${i}h ago`);
            success.push(Math.floor(Math.random() * 100) + 50);
            failed.push(Math.floor(Math.random() * 10) + 2);
        }
        
        return { labels, success, failed };
    }

    async loadProviderStatus() {
        const statusContainer = document.getElementById('providerStatus');
        
        try {
            const filterParams = this.buildFilterParams();
            const response = await this.authenticatedFetch(`/v1/analytics/providers?${filterParams}`);
            
            if (response && response.ok) {
                const data = await response.json();
                if (data.success) {
                    const providers = data.data;
                    statusContainer.innerHTML = providers.map(provider => `
                        <div class="provider-item">
                            <div class="provider-info">
                                <div class="provider-status ${provider.status}"></div>
                                <span class="provider-name">${provider.name}</span>
                                ${provider.tenantCount ? `<span style="font-size: 0.8rem; opacity: 0.7; margin-left: 8px;">(${provider.tenantCount} tenants)</span>` : ''}
                            </div>
                            <div style="text-align: right;">
                                <div class="provider-response">${provider.responseTime}</div>
                                ${provider.environment && provider.environment !== 'all' ? `<div style="font-size: 0.7rem; opacity: 0.6;">${provider.environment}</div>` : ''}
                            </div>
                        </div>
                    `).join('');
                    return;
                }
            }
        } catch (error) {
            console.error('Error loading provider status:', error);
        }

        // Fallback to static data
        let providers = [
            { name: 'İyzico', status: 'online', responseTime: '145ms', tenantCount: 3, environment: 'production' },
            { name: 'Stripe', status: 'online', responseTime: '89ms', tenantCount: 5, environment: 'production' },
            { name: 'OzanPay', status: 'online', responseTime: '203ms', tenantCount: 2, environment: 'production' },
            { name: 'Paycell', status: 'degraded', responseTime: '456ms', tenantCount: 1, environment: 'sandbox' },
            { name: 'Papara', status: 'online', responseTime: '167ms', tenantCount: 4, environment: 'production' }
        ];

        // Filter providers if specific provider is selected
        if (this.currentFilters.provider_id !== 'all') {
            providers = providers.filter(p => 
                p.name.toLowerCase().includes(this.currentFilters.provider_id.toLowerCase())
            );
        }

        statusContainer.innerHTML = providers.map(provider => `
            <div class="provider-item">
                <div class="provider-info">
                    <div class="provider-status ${provider.status}"></div>
                    <span class="provider-name">${provider.name}</span>
                    <span style="font-size: 0.8rem; opacity: 0.7; margin-left: 8px;">(${provider.tenantCount} tenants)</span>
                </div>
                <div style="text-align: right;">
                    <div class="provider-response">${provider.responseTime}</div>
                    <div style="font-size: 0.7rem; opacity: 0.6;">${provider.environment}</div>
                </div>
            </div>
        `).join('');
    }

    async loadRecentActivity() {
        const activityContainer = document.getElementById('recentActivity');
        
        try {
            const filterParams = this.buildFilterParams();
            const response = await this.authenticatedFetch(`/v1/analytics/activity?limit=50&${filterParams}`);
            
            if (response && response.ok) {
                const data = await response.json();
                if (data.success) {
                    const activities = data.data;
                    activityContainer.innerHTML = activities.map((activity, index) => `
                        <div class="activity-item clickable-activity" data-activity-index="${index}">
                            <div class="activity-info">
                                <div class="activity-icon" style="background: ${
                                    activity.status === 'success' ? '#dcfce7; color: #16a34a' :
                                    activity.status === 'failed' ? '#fecaca; color: #dc2626' :
                                    '#dbeafe; color: #2563eb'
                                };">
                                    ${activity.type === 'payment' ? '💳' : '↩️'}
                                </div>
                                <div class="activity-details">
                                    <h4>${activity.provider} ${activity.type} <span style="font-size: 0.8rem; opacity: 0.6;">🔍 Click for details</span></h4>
                                    <p>${activity.amount}</p>
                                    ${activity.endpoint ? `<p style="font-size: 0.75rem; opacity: 0.8; color: #667eea; font-family: monospace;">${activity.endpoint}</p>` : ''}
                                    ${activity.tenantId ? `<p style="font-size: 0.8rem; opacity: 0.7;">Tenant: ${activity.tenantId} | ${activity.env || 'production'}</p>` : ''}
                                </div>
                            </div>
                            <span class="activity-time">${activity.time}</span>
                        </div>
                    `).join('');

                    // Store activities data for modal access
                    this.currentActivities = activities;

                    // Add click event listeners to activities
                    this.addActivityClickListeners();
                    return;
                }
            }
        } catch (error) {
            console.error('Error loading recent activity:', error);
        }

        // Fallback to static data - Generate 50 demo activities
        let activities = [];
        const providers = ['İyzico', 'Stripe', 'OzanPay', 'Paycell', 'Papara', 'Nkolay', 'Paytr', 'Payu'];
        const types = ['payment', 'refund'];
        const statuses = ['success', 'success', 'success', 'failed', 'processed'];
        const environments = ['production', 'production', 'sandbox'];
        
        for (let i = 0; i < 50; i++) {
            const randomProvider = providers[Math.floor(Math.random() * providers.length)];
            const randomType = i % 10 === 0 ? 'refund' : 'payment'; // 10% refunds
            const randomStatus = statuses[Math.floor(Math.random() * statuses.length)];
            const randomEnv = environments[Math.floor(Math.random() * environments.length)];
            const randomAmount = (Math.random() * 1000 + 50).toFixed(2);
            const randomTime = Math.floor(Math.random() * 1440); // 0-1440 minutes
            
            // Generate realistic endpoints based on provider and type
            const endpoints = {
                payment: ['/payment', '/pay', '/charge', '/process', '/transaction'],
                refund: ['/refund', '/cancel', '/reverse', '/void']
            };
            const randomEndpoint = endpoints[randomType][Math.floor(Math.random() * endpoints[randomType].length)];
            
            let timeStr;
            if (randomTime < 60) {
                timeStr = `${randomTime} min ago`;
            } else if (randomTime < 1440) {
                timeStr = `${Math.floor(randomTime / 60)}h ${randomTime % 60}m ago`;
            } else {
                timeStr = `${Math.floor(randomTime / 1440)} days ago`;
            }
            
            activities.push({
                type: randomType,
                provider: randomProvider,
                amount: `₺${randomAmount}`,
                status: randomStatus,
                time: timeStr,
                tenantId: Math.floor(Math.random() * 3) + 1,
                env: randomEnv,
                id: `demo_${Date.now()}_${i}`,
                endpoint: randomEndpoint
            });
        }

        // Filter activities based on current filters
        if (this.currentFilters.tenant_id !== 'all') {
            activities = activities.filter(a => a.tenantId === this.currentFilters.tenant_id);
        }
        
        if (this.currentFilters.provider_id !== 'all') {
            activities = activities.filter(a => 
                a.provider.toLowerCase().includes(this.currentFilters.provider_id.toLowerCase())
            );
        }
        
        if (this.currentFilters.environment !== 'all') {
            activities = activities.filter(a => a.env === this.currentFilters.environment);
        }

        // Add fake request/response data for demo if not present
        activities.forEach((activity, index) => {
            if (!activity.request) {
                activity.request = JSON.stringify({
                    payment_id: activity.id || "demo_payment_123",
                    amount: parseFloat(activity.amount.replace('₺', '')),
                    currency: "TRY",
                    provider: activity.provider.toLowerCase(),
                    method: "POST",
                    endpoint: activity.endpoint || "/payment"
                }, null, 2);
            }
            
            if (!activity.response) {
                activity.response = JSON.stringify({
                    success: activity.status === 'success',
                    payment_id: activity.id || "demo_payment_123",
                    status: activity.status,
                    message: activity.status === 'success' ? "Payment processed successfully" : "Payment failed",
                    timestamp: new Date().toISOString(),
                    endpoint: activity.endpoint || "/payment"
                }, null, 2);
            }
        });

        // Add fake request/response data for fallback activities
        activities.forEach((activity, index) => {
            if (!activity.request) {
                activity.request = JSON.stringify({
                    payment_id: activity.id || "demo_payment_123",
                    amount: parseFloat(activity.amount.replace('₺', '')),
                    currency: "TRY",
                    provider: activity.provider.toLowerCase(),
                    method: "POST",
                    endpoint: activity.endpoint || "/payment"
                }, null, 2);
            }
            
            if (!activity.response) {
                activity.response = JSON.stringify({
                    success: activity.status === 'success',
                    payment_id: activity.id || "demo_payment_123",
                    status: activity.status,
                    message: activity.status === 'success' ? "Payment processed successfully" : "Payment failed",
                    timestamp: new Date().toISOString(),
                    endpoint: activity.endpoint || "/payment"
                }, null, 2);
            }
        });

        activityContainer.innerHTML = activities.map((activity, index) => `
            <div class="activity-item clickable-activity" data-activity-index="${index}">
                <div class="activity-info">
                    <div class="activity-icon" style="background: ${
                        activity.status === 'success' ? '#dcfce7; color: #16a34a' :
                        activity.status === 'failed' ? '#fecaca; color: #dc2626' :
                        '#dbeafe; color: #2563eb'
                    };">
                        ${activity.type === 'payment' ? '💳' : '↩️'}
                    </div>
                    <div class="activity-details">
                        <h4>${activity.provider} ${activity.type} <span style="font-size: 0.8rem; opacity: 0.6;">🔍 Click for details</span></h4>
                        <p>${activity.amount}</p>
                        ${activity.endpoint ? `<p style="font-size: 0.75rem; opacity: 0.8; color: #667eea; font-family: monospace;">${activity.endpoint}</p>` : ''}
                        <p style="font-size: 0.8rem; opacity: 0.7;">Tenant: ${activity.tenantId} | ${activity.env}</p>
                    </div>
                </div>
                <span class="activity-time">${activity.time}</span>
            </div>
        `).join('');

        // Store activities data for modal access
        this.currentActivities = activities;

        // Add click event listeners to activities
        this.addActivityClickListeners();
    }

    startRealTimeUpdates() {
        // Update dashboard every 30 seconds
        setInterval(() => {
            this.loadDashboardData();
        }, 30000);
    }

    addActivityClickListeners() {
        const activityItems = document.querySelectorAll('.clickable-activity');
        activityItems.forEach(item => {
            item.addEventListener('click', (e) => {
                const activityIndex = parseInt(e.currentTarget.getAttribute('data-activity-index'));
                if (this.currentActivities && this.currentActivities[activityIndex]) {
                    this.showActivityModal(this.currentActivities[activityIndex]);
                }
            });
            
            // Add hover effect
            item.style.cursor = 'pointer';
            item.addEventListener('mouseenter', () => {
                item.style.backgroundColor = '#f8fafc';
            });
            item.addEventListener('mouseleave', () => {
                item.style.backgroundColor = '';
            });
        });
    }

    showActivityModal(activity) {
        const modal = document.getElementById('activityModal');
        
        // Populate activity details
        document.getElementById('modalProvider').textContent = activity.provider;
        document.getElementById('modalType').textContent = activity.type;
        document.getElementById('modalAmount').textContent = activity.amount;
        document.getElementById('modalStatus').textContent = activity.status;
        document.getElementById('modalPaymentId').textContent = activity.id;
        document.getElementById('modalEndpoint').textContent = activity.endpoint || 'N/A';
        document.getElementById('modalTime').textContent = activity.time;
        
        // Format and display JSON data
        this.displayJsonData('requestJson', activity.request);
        this.displayJsonData('responseJson', activity.response);
        
        // Show modal
        modal.style.display = 'flex';
        
        // Add event listeners for closing modal
        this.addModalCloseListeners();
    }

    displayJsonData(elementId, jsonString) {
        const element = document.getElementById(elementId);
        
        if (!jsonString || jsonString.trim() === '') {
            element.innerHTML = '<span style="color: #6b7280; font-style: italic;">No data available</span>';
            return;
        }

        try {
            // Try to parse and format JSON
            const jsonObj = JSON.parse(jsonString);
            const formattedJson = this.formatJsonWithSyntaxHighlighting(jsonObj);
            element.innerHTML = formattedJson;
        } catch (e) {
            // If not valid JSON, display as plain text
            element.innerHTML = this.escapeHtml(jsonString);
        }
    }

    formatJsonWithSyntaxHighlighting(obj) {
        const jsonString = JSON.stringify(obj, null, 2);
        
        return jsonString
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/("(?:[^"\\]|\\.)*")\s*:/g, '<span class="json-key">$1</span>:')
            .replace(/:\s*("(?:[^"\\]|\\.)*")/g, ': <span class="json-string">$1</span>')
            .replace(/:\s*(-?\d+\.?\d*)/g, ': <span class="json-number">$1</span>')
            .replace(/:\s*(true|false)/g, ': <span class="json-boolean">$1</span>')
            .replace(/:\s*(null)/g, ': <span class="json-null">$1</span>');
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    addModalCloseListeners() {
        const modal = document.getElementById('activityModal');
        const modalClose = document.getElementById('modalClose');
        
        // Close on X button click
        modalClose.onclick = () => {
            modal.style.display = 'none';
        };
        
        // Close on background click
        modal.onclick = (e) => {
            if (e.target === modal) {
                modal.style.display = 'none';
            }
        };
        
        // Close on Escape key
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && modal.style.display === 'flex') {
                modal.style.display = 'none';
            }
        });
    }

    // Cleanup method to destroy charts
    cleanup() {
        if (this.trendsChart) {
            this.trendsChart.destroy();
            this.trendsChart = null;
        }
        if (this.distributionChart) {
            this.distributionChart.destroy();
            this.distributionChart = null;
        }
    }
}