#!/bin/bash

# Multi-Tenant Setup Examples for GoPay
# This script demonstrates how to configure multiple tenants with different provider credentials

BASE_URL="http://localhost:9999/v1"

echo "🏢 Multi-Tenant Setup Examples for GoPay"
echo "========================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}This script shows how to configure different tenants with their own payment provider credentials.${NC}"
echo -e "${BLUE}Each tenant can use different providers (İyzico, OzanPay, Paycell) or the same provider with different credentials.${NC}"
echo ""

# 1. Setup Tenant APP1 with İyzico
echo -e "${YELLOW}1. Setup Tenant APP1 with İyzico Credentials${NC}"
echo "--------------------------------------------"
curl -X POST "${BASE_URL}/set-env" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: APP1" \
  -d '{
    "IYZICO_API_KEY": "sandbox-app1-iyzico-api-key",
    "IYZICO_SECRET_KEY": "sandbox-app1-iyzico-secret-key",
    "IYZICO_ENVIRONMENT": "sandbox"
  }'

echo -e "\n${GREEN}✅ Tenant APP1 configured with İyzico${NC}\n"

# 2. Setup Tenant APP2 with different İyzico credentials
echo -e "${YELLOW}2. Setup Tenant APP2 with Different İyzico Credentials${NC}"
echo "---------------------------------------------------"
curl -X POST "${BASE_URL}/set-env" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: APP2" \
  -d '{
    "IYZICO_API_KEY": "sandbox-app2-iyzico-api-key",
    "IYZICO_SECRET_KEY": "sandbox-app2-iyzico-secret-key",
    "IYZICO_ENVIRONMENT": "sandbox"
  }'

echo -e "\n${GREEN}✅ Tenant APP2 configured with different İyzico credentials${NC}\n"

# 3. Setup Tenant XYZ with OzanPay
echo -e "${YELLOW}3. Setup Tenant XYZ with OzanPay Credentials${NC}"
echo "-------------------------------------------"
curl -X POST "${BASE_URL}/set-env" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: XYZ" \
  -d '{
    "OZANPAY_API_KEY": "sandbox-xyz-ozanpay-api-key",
    "OZANPAY_SECRET_KEY": "sandbox-xyz-ozanpay-secret-key",
    "OZANPAY_MERCHANT_ID": "sandbox-xyz-merchant-12345",
    "OZANPAY_ENVIRONMENT": "sandbox"
  }'

echo -e "\n${GREEN}✅ Tenant XYZ configured with OzanPay${NC}\n"

# 4. Setup Tenant ENTERPRISE with Multiple Providers
echo -e "${YELLOW}4. Setup Tenant ENTERPRISE with Multiple Providers${NC}"
echo "-------------------------------------------------"
curl -X POST "${BASE_URL}/set-env" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: ENTERPRISE" \
  -d '{
    "IYZICO_API_KEY": "sandbox-enterprise-iyzico-api-key",
    "IYZICO_SECRET_KEY": "sandbox-enterprise-iyzico-secret-key",
    "IYZICO_ENVIRONMENT": "sandbox",
    "PAYCELL_USERNAME": "sandbox-enterprise-paycell-user",
    "PAYCELL_PASSWORD": "sandbox-enterprise-paycell-pass",
    "PAYCELL_MERCHANT_ID": "sandbox-enterprise-merchant-789",
    "PAYCELL_TERMINAL_ID": "sandbox-enterprise-terminal-456",
    "PAYCELL_ENVIRONMENT": "sandbox"
  }'

echo -e "\n${GREEN}✅ Tenant ENTERPRISE configured with İyzico and Paycell${NC}\n"

# 5. Check Tenant Configuration
echo -e "${YELLOW}5. Check Tenant Configurations${NC}"
echo "-------------------------------"

echo "Checking APP1 tenant İyzico config:"
curl -X GET "${BASE_URL}/config/tenant-config?provider=iyzico" \
  -H "X-Tenant-ID: APP1"

echo -e "\n"
echo "Checking XYZ tenant OzanPay config:"
curl -X GET "${BASE_URL}/config/tenant-config?provider=ozanpay" \
  -H "X-Tenant-ID: XYZ"

echo -e "\n${GREEN}✅ Configuration checks completed${NC}\n"

# 6. Get System Statistics
echo -e "${YELLOW}6. Get System Statistics${NC}"
echo "------------------------"
curl -X GET "${BASE_URL}/stats"

echo -e "\n${GREEN}✅ Multi-tenant setup completed!${NC}\n"

echo -e "${BLUE}Next Steps:${NC}"
echo "1. Run tenant_curl_examples.sh to test payments with different tenants"
echo "2. Check the multi_tenant_example.go for Go integration"
echo "3. Use the configured tenant IDs in your application requests"

echo -e "\n${BLUE}Important Notes:${NC}"
echo "• Each tenant must include X-Tenant-ID header in payment requests"
echo "• Tenant configurations persist across application restarts"
echo "• You can update/delete tenant configurations using the API"
echo "• Use GET /v1/stats to monitor tenant usage" 