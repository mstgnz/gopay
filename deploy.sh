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

# Step 2: Start core services first (PostgreSQL)
echo "🐘 Starting PostgreSQL database..."
docker-compose up -d postgres

# Wait for PostgreSQL to be ready
echo "⏳ Waiting for PostgreSQL to be ready..."
timeout 60s bash -c 'until docker-compose exec -T postgres pg_isready -U ${DB_USER:-gopay} > /dev/null 2>&1; do sleep 2; done'
echo "✅ PostgreSQL is ready"

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
echo "   • Database: PostgreSQL (Port: ${DB_PORT:-5432})"
echo "   • Health Check: ✅ Passed"
echo ""
echo "🔗 Endpoints:"
echo "   • API: http://localhost:$APP_PORT"
echo "   • Health: http://localhost:$APP_PORT/health"
echo "   • Database: postgresql://localhost:${DB_PORT:-5432}"
echo ""
echo "📋 Useful Commands:"
echo "   • Scale API: docker-compose up -d --scale api=5"
echo "   • View logs: docker-compose logs -f api"
echo "   • Monitor: docker-compose ps"
echo "   • Stop: docker-compose down"
echo "   • Database logs: docker-compose logs -f postgres"
echo ""

# Show running containers
echo "🐳 Running Containers:"
docker-compose ps 