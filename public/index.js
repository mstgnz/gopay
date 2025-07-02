// Analytics Dashboard JavaScript
class GoPayAnalytics {
    constructor() {
        this.providers = ['iyzico', 'stripe', 'ozanpay', 'paycell', 'papara', 'nkolay', 'paytr', 'payu'];
        this.trendsChart = null;
        this.distributionChart = null;
        this.init();
    }

    async init() {
        await this.loadDashboardData();
        this.initCharts();
        this.startRealTimeUpdates();
    }

    async loadDashboardData() {
        try {
            // Load dashboard stats from analytics API
            const dashboardResponse = await fetch('/v1/analytics/dashboard?hours=24');
            if (dashboardResponse.ok) {
                const dashboardData = await dashboardResponse.json();
                if (dashboardData.success) {
                    this.updateStats(dashboardData.data);
                }
            }

            // Load provider status and recent activity
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
                avgResponseChange: "-15ms from yesterday"
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
            const response = await fetch('/v1/analytics/trends?hours=24');
            if (response.ok) {
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
            const response = await fetch('/v1/analytics/providers');
            if (response.ok) {
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
                            maintainAspectRatio: false
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
        this.distributionChart = new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: ['İyzico', 'Stripe', 'OzanPay', 'Paycell', 'Others'],
                datasets: [{
                    data: [35, 25, 20, 15, 5],
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
                maintainAspectRatio: false
            }
        });
    }

    generateTrendData() {
        const labels = [];
        const success = [];
        const failed = [];
        
        for (let i = 23; i >= 0; i--) {
            labels.push(`${i}h ago`);
            success.push(Math.floor(Math.random() * 100) + 50);
            failed.push(Math.floor(Math.random() * 10) + 2);
        }
        
        return { labels, success, failed };
    }

    async loadProviderStatus() {
        const statusContainer = document.getElementById('providerStatus');
        
        try {
            const response = await fetch('/v1/analytics/providers');
            if (response.ok) {
                const data = await response.json();
                if (data.success) {
                    const providers = data.data;
                    statusContainer.innerHTML = providers.map(provider => `
                        <div class="provider-item">
                            <div class="provider-info">
                                <div class="provider-status ${provider.status}"></div>
                                <span class="provider-name">${provider.name}</span>
                            </div>
                            <span class="provider-response">${provider.responseTime}</span>
                        </div>
                    `).join('');
                    return;
                }
            }
        } catch (error) {
            console.error('Error loading provider status:', error);
        }

        // Fallback to static data
        const providers = [
            { name: 'İyzico', status: 'online', responseTime: '145ms' },
            { name: 'Stripe', status: 'online', responseTime: '89ms' },
            { name: 'OzanPay', status: 'online', responseTime: '203ms' },
            { name: 'Paycell', status: 'degraded', responseTime: '456ms' },
            { name: 'Papara', status: 'online', responseTime: '167ms' }
        ];

        statusContainer.innerHTML = providers.map(provider => `
            <div class="provider-item">
                <div class="provider-info">
                    <div class="provider-status ${provider.status}"></div>
                    <span class="provider-name">${provider.name}</span>
                </div>
                <span class="provider-response">${provider.responseTime}</span>
            </div>
        `).join('');
    }

    async loadRecentActivity() {
        const activityContainer = document.getElementById('recentActivity');
        
        try {
            const response = await fetch('/v1/analytics/activity?limit=5');
            if (response.ok) {
                const data = await response.json();
                if (data.success) {
                    const activities = data.data;
                    activityContainer.innerHTML = activities.map(activity => `
                        <div class="activity-item">
                            <div class="activity-info">
                                <div class="activity-icon" style="background: ${
                                    activity.status === 'success' ? '#dcfce7; color: #16a34a' :
                                    activity.status === 'failed' ? '#fecaca; color: #dc2626' :
                                    '#dbeafe; color: #2563eb'
                                };">
                                    ${activity.type === 'payment' ? '💳' : '↩️'}
                                </div>
                                <div class="activity-details">
                                    <h4>${activity.provider} ${activity.type}</h4>
                                    <p>${activity.amount}</p>
                                </div>
                            </div>
                            <span class="activity-time">${activity.time}</span>
                        </div>
                    `).join('');
                    return;
                }
            }
        } catch (error) {
            console.error('Error loading recent activity:', error);
        }

        // Fallback to static data
        const activities = [
            { type: 'payment', provider: 'İyzico', amount: '₺150.00', status: 'success', time: '2 min ago' },
            { type: 'refund', provider: 'Stripe', amount: '₺75.50', status: 'processed', time: '5 min ago' },
            { type: 'payment', provider: 'OzanPay', amount: '₺300.00', status: 'success', time: '8 min ago' },
            { type: 'payment', provider: 'Paycell', amount: '₺89.99', status: 'failed', time: '12 min ago' },
            { type: 'payment', provider: 'Papara', amount: '₺250.00', status: 'success', time: '15 min ago' }
        ];

        activityContainer.innerHTML = activities.map(activity => `
            <div class="activity-item">
                <div class="activity-info">
                    <div class="activity-icon" style="background: ${
                        activity.status === 'success' ? '#dcfce7; color: #16a34a' :
                        activity.status === 'failed' ? '#fecaca; color: #dc2626' :
                        '#dbeafe; color: #2563eb'
                    };">
                        ${activity.type === 'payment' ? '💳' : '↩️'}
                    </div>
                    <div class="activity-details">
                        <h4>${activity.provider} ${activity.type}</h4>
                        <p>${activity.amount}</p>
                    </div>
                </div>
                <span class="activity-time">${activity.time}</span>
            </div>
        `).join('');
    }

    startRealTimeUpdates() {
        // Update dashboard every 30 seconds
        setInterval(() => {
            this.loadDashboardData();
        }, 30000);
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