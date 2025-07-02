#!/bin/bash

# GoPay High Availability Deployment Script
# This script deploys GoPay with multiple API replicas for enterprise use

set -e

# Configuration
REPLICAS=${REPLICAS:-3}
APP_NAME=${APP_NAME:-gopay}
ENVIRONMENT=${ENVIRONMENT:-production}

echo "🚀 Starting GoPay High Availability Deployment"
echo "   Replicas: $REPLICAS"
echo "   App Name: $APP_NAME"
echo "   Environment: $ENVIRONMENT"

# Step 1: Build the application
echo "📦 Building GoPay application..."
docker-compose build

# Step 2: Start core services first (OpenSearch)
echo "🔍 Starting OpenSearch..."
docker-compose up -d opensearch

# Wait for OpenSearch to be ready
echo "⏳ Waiting for OpenSearch to be ready..."
timeout 60s bash -c 'until curl -s http://localhost:9200/_cluster/health | grep -q "yellow\|green"; do sleep 2; done'
echo "✅ OpenSearch is ready"

# Step 3: Start API services with scaling
echo "🌐 Starting API services with $REPLICAS replicas..."
docker-compose up -d --scale api=$REPLICAS api

# Step 4: Start Nginx load balancer
echo "⚖️  Starting Nginx load balancer..."
docker-compose up -d nginx

# Step 5: Health checks
echo "🏥 Performing health checks..."
sleep 5

# Check if all services are running
if ! docker-compose ps | grep -q "Up"; then
    echo "❌ Some services are not running!"
    docker-compose ps
    exit 1
fi

# Check API health through load balancer
APP_PORT=${APP_PORT:-9999}
if curl -f http://localhost:$APP_PORT/health > /dev/null 2>&1; then
    echo "✅ Load balancer health check passed"
else
    echo "❌ Load balancer health check failed"
    exit 1
fi

echo ""
echo "🎉 GoPay High Availability Deployment Completed Successfully!"
echo ""
echo "📊 Deployment Summary:"
echo "   • API Replicas: $REPLICAS"
echo "   • Load Balancer: Nginx (Port: $APP_PORT)"
echo "   • OpenSearch: Running (Port: 9200)"
echo "   • Health Check: ✅ Passed"
echo ""
echo "🔗 Endpoints:"
echo "   • API: http://localhost:$APP_PORT"
echo "   • Health: http://localhost:$APP_PORT/health"
echo "   • OpenSearch: http://localhost:9200"
echo ""
echo "📋 Useful Commands:"
echo "   • Scale API: docker-compose up -d --scale api=5"
echo "   • View logs: docker-compose logs -f api"
echo "   • Monitor: docker-compose ps"
echo "   • Stop: docker-compose down"
echo ""

# Show running containers
echo "🐳 Running Containers:"
docker-compose ps 