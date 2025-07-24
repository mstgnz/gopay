// Analytics Dashboard JavaScript
class GoPayAnalytics {
    constructor() {
        this.providers = ['iyzico', 'stripe', 'ozanpay', 'paycell', 'papara', 'nkolay', 'paytr', 'payu'];
        this.trendsChart = null;
        this.distributionChart = null;
        const now = new Date();
        this.currentFilters = {
            tenant_id: 'all',
            provider_id: 'all',
            environment: 'all',
            month: now.getMonth() + 1, // 1-12
            year: now.getFullYear()
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
        this.applyTenantRestrictions();
        await this.loadDashboardData();
        this.initCharts();
        this.updateSearchState();
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

            // Store user tenant info for filtering
            this.userTenantId = data.data.tenant_id;
            this.isAdmin = (data.data.tenant_id === "1");

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
        const monthFilter = document.getElementById('monthFilter');
        const yearFilter = document.getElementById('yearFilter');
        const refreshButton = document.getElementById('refreshButton');
        const paymentSearch = document.getElementById('paymentSearch');
        const searchButton = document.getElementById('searchButton');

        // Initialize month and year options
        this.initializeDateFilters();

        if (tenantFilter) {
            tenantFilter.addEventListener('change', (e) => {
                this.currentFilters.tenant_id = e.target.value;
                this.onFiltersChanged();
                this.updateSearchState();
            });
        }

        if (providerFilter) {
            providerFilter.addEventListener('change', (e) => {
                this.currentFilters.provider_id = e.target.value;
                this.onFiltersChanged();
                this.updateSearchState();
            });
        }

        if (environmentFilter) {
            environmentFilter.addEventListener('change', (e) => {
                this.currentFilters.environment = e.target.value;
                this.onFiltersChanged();
                this.updateSearchState();
            });
        }

        if (monthFilter) {
            monthFilter.addEventListener('change', (e) => {
                this.currentFilters.month = parseInt(e.target.value);
                this.onFiltersChanged();
                this.updateChartTitle();
            });
        }

        if (yearFilter) {
            yearFilter.addEventListener('change', (e) => {
                this.currentFilters.year = parseInt(e.target.value);
                this.onFiltersChanged();
                this.updateChartTitle();
            });
        }

        if (refreshButton) {
            refreshButton.addEventListener('click', () => {
                this.refreshDashboard();
            });
        }

        if (paymentSearch) {
            paymentSearch.addEventListener('keypress', (e) => {
                if (e.key === 'Enter' && !paymentSearch.disabled) {
                    this.searchPaymentById(paymentSearch.value.trim());
                }
            });
        }

        if (searchButton) {
            searchButton.addEventListener('click', () => {
                if (!searchButton.disabled && paymentSearch) {
                    this.searchPaymentById(paymentSearch.value.trim());
                }
            });
        }
    }

    initializeDateFilters() {
        const monthFilter = document.getElementById('monthFilter');
        const yearFilter = document.getElementById('yearFilter');
        
        if (monthFilter) {
            const months = [
                'January', 'February', 'March', 'April', 'May', 'June',
                'July', 'August', 'September', 'October', 'November', 'December'
            ];
            
            months.forEach((month, index) => {
                const option = document.createElement('option');
                option.value = index + 1;
                option.textContent = month;
                if (index + 1 === this.currentFilters.month) {
                    option.selected = true;
                }
                monthFilter.appendChild(option);
            });
        }
        
        if (yearFilter) {
            const currentYear = new Date().getFullYear();
            // Show last 3 years and next year
            for (let year = currentYear - 2; year <= currentYear + 1; year++) {
                const option = document.createElement('option');
                option.value = year;
                option.textContent = year;
                if (year === this.currentFilters.year) {
                    option.selected = true;
                }
                yearFilter.appendChild(option);
            }
        }
        
        // Update chart title initially
        this.updateChartTitle();
    }

    updateChartTitle() {
        const titleElement = document.getElementById('paymentTrendsTitle');
        if (titleElement) {
            const months = [
                'January', 'February', 'March', 'April', 'May', 'June',
                'July', 'August', 'September', 'October', 'November', 'December'
            ];
            const monthName = months[this.currentFilters.month - 1];
            titleElement.textContent = `📈 Payment Trends (${monthName} ${this.currentFilters.year})`;
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

    applyTenantRestrictions() {
        const tenantFilter = document.getElementById('tenantFilter');
        
        if (!this.isAdmin) {
            // Non-admin users: set their tenant and disable the filter
            if (tenantFilter) {
                // Set current tenant as selected
                this.currentFilters.tenant_id = this.userTenantId;
                tenantFilter.value = this.userTenantId;
                tenantFilter.disabled = true;
                
                // Add visual indicator
                const filterGroup = tenantFilter.closest('.filter-group');
                if (filterGroup) {
                    const label = filterGroup.querySelector('label');
                    if (label) {
                        label.innerHTML = `Tenant: <span style="color: #10b981; font-weight: 600;">Your Data</span>`;
                    }
                }
            }
            
            // Update dashboard subtitle
            const subtitle = document.querySelector('.header-subtitle');
            if (subtitle) {
                subtitle.textContent = `Payment Analytics Dashboard - Tenant ${this.userTenantId}`;
            }
        } else {
            // Admin users: show normal interface with access indicator
            const subtitle = document.querySelector('.header-subtitle');
            if (subtitle) {
                subtitle.innerHTML = `Multi-Tenant Payment Analytics Dashboard <span style="color: #f59e0b;">👑 Admin Access</span>`;
            }
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
            // Show zero values instead of fake data
            this.updateStats({
                totalPayments: 0,
                successRate: 0,
                totalVolume: 0,
                avgResponseTime: 0,
                totalPaymentsChange: "No data available",
                successRateChange: "No data available",
                totalVolumeChange: "No data available",
                avgResponseChange: "No data available",
                activeTenants: 0,
                activeProviders: 0,
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
        document.getElementById('totalPaymentsChange').textContent = stats.totalPaymentsChange || 'No data available';
        document.getElementById('successRateChange').textContent = stats.successRateChange || 'No data available';
        document.getElementById('totalVolumeChange').textContent = stats.totalVolumeChange || 'No data available';
        document.getElementById('avgResponseTimeChange').textContent = stats.avgResponseChange || 'No data available';

        // Update title to show current filter context
        this.updateDashboardTitle(stats);
    }

    updateDashboardTitle(stats) {
        const subtitle = document.querySelector('.header-subtitle');
        if (subtitle) {
            let context = 'Multi-Tenant Payment Analytics Dashboard';
            
            // Add admin indicator for admin users only
            if (this.isAdmin) {
                context += ' <span style="color: #f59e0b;">👑 Admin Access</span>';
                subtitle.innerHTML = context;
            } else {
                context += ` - Tenant ${this.userTenantId}`;
                subtitle.textContent = context;
            }
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
                    // Show empty chart with no data message
                    this.createEmptyTrendsChart(trendsCtx);
                }
            } else {
                this.createEmptyTrendsChart(trendsCtx);
            }
        } catch (error) {
            console.error('Error loading trends data:', error);
            this.createEmptyTrendsChart(trendsCtx);
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
                    this.createEmptyDistributionChart(distributionCtx);
                }
            } else {
                this.createEmptyDistributionChart(distributionCtx);
            }
        } catch (error) {
            console.error('Error loading provider distribution:', error);
            this.createEmptyDistributionChart(distributionCtx);
        }
    }

    createEmptyTrendsChart(ctx) {
        this.trendsChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: 'Successful Payments',
                    data: [],
                    borderColor: '#10B981',
                    backgroundColor: 'rgba(16, 185, 129, 0.1)',
                    tension: 0.4
                }, {
                    label: 'Failed Payments',
                    data: [],
                    borderColor: '#EF4444',
                    backgroundColor: 'rgba(239, 68, 68, 0.1)',
                    tension: 0.4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: true
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true
                    }
                }
            }
        });
    }

    createEmptyDistributionChart(ctx) {
        this.distributionChart = new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: [],
                datasets: [{
                    data: [],
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
                        position: 'bottom',
                        display: true
                    }
                }
            }
        });
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

        // Show message if no provider data available
        statusContainer.innerHTML = `
            <div style="text-align: center; padding: 20px; color: #6b7280; font-style: italic;">
                <p>No provider data available</p>
                <p style="font-size: 0.8rem; margin-top: 8px;">Check your connection or try refreshing</p>
            </div>
        `;
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

        // Show message if no activity data available
        activityContainer.innerHTML = `
            <div style="text-align: center; padding: 20px; color: #6b7280; font-style: italic;">
                <p>No recent activity data available</p>
                <p style="font-size: 0.8rem; margin-top: 8px;">Make some payments to see activity here</p>
            </div>
        `;

        // Clear stored activities
        this.currentActivities = [];
    }

    updateSearchState() {
        const paymentSearch = document.getElementById('paymentSearch');
        const searchButton = document.getElementById('searchButton');
        
        if (paymentSearch) {
            // Enable search only if tenant and provider are selected (not 'all')
            const canSearch = this.currentFilters.tenant_id !== 'all' && 
                             this.currentFilters.provider_id !== 'all';
            
            paymentSearch.disabled = !canSearch;
            paymentSearch.placeholder = canSearch ? 
                'Search by payment ID...' : 
                'Select tenant and provider first';
            
            if (searchButton) {
                searchButton.disabled = !canSearch;
            }
            
            if (!canSearch) {
                paymentSearch.value = '';
            }
        }
    }

    async searchPaymentById(paymentId) {
        if (!paymentId) {
            this.showErrorModal('Invalid Input', 'Please enter a payment ID to search.', null);
            return;
        }

        if (this.currentFilters.tenant_id === 'all' || this.currentFilters.provider_id === 'all') {
            this.showErrorModal('Missing Selection', 'Please select both tenant and provider before searching.', null);
            return;
        }

        try {
            const searchParams = new URLSearchParams({
                tenant_id: this.currentFilters.tenant_id,
                provider_id: this.currentFilters.provider_id,
                payment_id: paymentId
            });

            const response = await this.authenticatedFetch(`/v1/analytics/search?${searchParams}`);
            
            if (response && response.ok) {
                const data = await response.json();
                if (data.success && data.data) {
                    // Check if single payment or multiple payments
                    if (Array.isArray(data.data)) {
                        // Multiple payments - show search results modal
                        this.showSearchResultsModal(data.data, paymentId);
                    } else {
                        // Single payment - show activity modal  
                        this.showActivityModal(data.data);
                    }
                } else {
                    this.showErrorModal('Payment Not Found', data.message || 'No payment found with the given ID', null, () => {
                        // Retry callback
                        this.searchPaymentById(paymentId);
                    });
                }
            } else {
                // Handle different error status codes
                let errorMessage = 'Search failed';
                let errorDescription = 'Please try again.';
                
                if (response) {
                    try {
                        const errorData = await response.json();
                        errorMessage = errorData.message || 'Search failed';
                        errorDescription = errorData.error || 'Please try again.';
                        this.showErrorModal(errorMessage, errorDescription, response.status, () => {
                            this.searchPaymentById(paymentId);
                        });
                    } catch (e) {
                        this.showErrorModal(errorMessage, errorDescription, response.status, () => {
                            this.searchPaymentById(paymentId);
                        });
                    }
                } else {
                    this.showErrorModal('Network Error', 'Unable to connect to server. Please check your connection.', null, () => {
                        this.searchPaymentById(paymentId);
                    });
                }
            }
        } catch (error) {
            console.error('Error searching payment:', error);
            this.showErrorModal('Search Error', 'An unexpected error occurred while searching. Please try again.', null, () => {
                this.searchPaymentById(paymentId);
            });
        }
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

    showSearchResultsModal(payments, paymentId) {
        const modal = document.getElementById('searchResultsModal');
        
        // Populate search info
        document.getElementById('searchPaymentId').textContent = paymentId;
        document.getElementById('searchProvider').textContent = payments[0].provider;
        document.getElementById('searchTotalRequests').textContent = payments.length;
        
        // Populate endpoints list
        const endpointsList = document.getElementById('endpointsList');
        endpointsList.innerHTML = payments.map((payment, index) => `
            <div class="endpoint-item" data-payment-index="${index}">
                <div class="endpoint-info">
                    <div class="endpoint-path">${payment.endpoint || '/unknown'}</div>
                    <div class="endpoint-meta">${payment.time} • ${payment.amount}</div>
                </div>
                <div class="endpoint-status ${payment.status}">${payment.status}</div>
            </div>
        `).join('');
        
        // Store payments for selection
        this.currentSearchResults = payments;
        
        // Add click listeners to endpoints
        this.addEndpointClickListeners();
        
        // Show modal
        modal.style.display = 'flex';
        
        // Add close listeners
        this.addSearchModalCloseListeners();
    }

    addEndpointClickListeners() {
        const endpointItems = document.querySelectorAll('.endpoint-item');
        endpointItems.forEach(item => {
            item.addEventListener('click', (e) => {
                // Remove previous selection
                endpointItems.forEach(ei => ei.classList.remove('selected'));
                
                // Add selection to clicked item
                item.classList.add('selected');
                
                // Get payment data
                const paymentIndex = parseInt(item.getAttribute('data-payment-index'));
                const payment = this.currentSearchResults[paymentIndex];
                
                // Show selected request details
                this.showSelectedRequestDetails(payment);
            });
        });
    }

    showSelectedRequestDetails(payment) {
        const detailsSection = document.getElementById('selectedRequestDetails');
        
        // Populate details
        document.getElementById('selectedEndpoint').textContent = payment.endpoint || 'N/A';
        document.getElementById('selectedAmount').textContent = payment.amount;
        document.getElementById('selectedStatus').textContent = payment.status;
        document.getElementById('selectedTime').textContent = payment.time;
        
        // Show JSON data
        this.displayJsonData('selectedRequestJson', payment.request);
        this.displayJsonData('selectedResponseJson', payment.response);
        
        // Show details section
        detailsSection.style.display = 'block';
    }

    addSearchModalCloseListeners() {
        const modal = document.getElementById('searchResultsModal');
        const modalClose = document.getElementById('searchModalClose');
        
        // Close on X button click
        modalClose.onclick = () => {
            modal.style.display = 'none';
            document.getElementById('selectedRequestDetails').style.display = 'none';
        };
        
        // Close on background click
        modal.onclick = (e) => {
            if (e.target === modal) {
                modal.style.display = 'none';
                document.getElementById('selectedRequestDetails').style.display = 'none';
            }
        };
        
        // Close on Escape key
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && modal.style.display === 'flex') {
                modal.style.display = 'none';
                document.getElementById('selectedRequestDetails').style.display = 'none';
            }
        });
    }

    showErrorModal(title, description, errorCode = null, retryCallback = null) {
        const modal = document.getElementById('errorModal');
        const titleElement = document.getElementById('errorModalTitle');
        const messageElement = document.getElementById('errorMessage');
        const descriptionElement = document.getElementById('errorDescription');
        const errorCodeElement = document.getElementById('errorCode');
        const errorCodeValue = document.getElementById('errorCodeValue');
        const retryBtn = document.getElementById('errorRetryBtn');
        
        // Set content
        titleElement.textContent = `⚠️ ${title}`;
        messageElement.textContent = title;
        descriptionElement.textContent = description;
        
        // Show/hide error code
        if (errorCode) {
            errorCodeValue.textContent = errorCode;
            errorCodeElement.style.display = 'block';
        } else {
            errorCodeElement.style.display = 'none';
        }
        
        // Handle retry button
        if (retryCallback) {
            retryBtn.style.display = 'block';
            retryBtn.onclick = () => {
                modal.style.display = 'none';
                retryCallback();
            };
        } else {
            retryBtn.style.display = 'none';
        }
        
        // Show modal
        modal.style.display = 'flex';
        
        // Add close listeners
        this.addErrorModalCloseListeners();
    }

    addErrorModalCloseListeners() {
        const modal = document.getElementById('errorModal');
        const modalClose = document.getElementById('errorModalClose');
        const closeBtn = document.getElementById('errorCloseBtn');
        
        // Close on X button click
        modalClose.onclick = () => {
            modal.style.display = 'none';
        };
        
        // Close on Close button click
        closeBtn.onclick = () => {
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