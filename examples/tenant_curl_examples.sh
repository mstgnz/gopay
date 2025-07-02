#!/bin/bash

# Multi-Tenant Payment Examples for GoPay
# This script demonstrates how different tenants make payments using their configured providers

BASE_URL="http://localhost:9999/v1"

echo "🏢 Multi-Tenant Payment Examples for GoPay"
echo "==========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Before running this script, make sure you have run multi_tenant_setup.sh${NC}"
echo -e "${BLUE}This demonstrates how different tenants use their own provider configurations${NC}"
echo ""

# 1. ABC Tenant Payment with İyzico
echo -e "${YELLOW}1. ABC Tenant Payment (İyzico Provider)${NC}"
echo "--------------------------------------"
curl -X POST "${BASE_URL}/payments/iyzico" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: ABC" \
  -d '{
    "amount": 150.00,
    "currency": "TRY",
    "customer": {
      "id": "abc_customer_001",
      "name": "Ahmet",
      "surname": "Yılmaz",
      "email": "ahmet@abc-company.com",
      "phoneNumber": "+905551234567",
      "ipAddress": "192.168.1.100",
      "address": {
        "city": "Istanbul",
        "country": "Turkey",
        "address": "ABC Company Address",
        "zipCode": "34000"
      }
    },
    "cardInfo": {
      "cardHolderName": "Ahmet Yılmaz",
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "abc_product_1",
        "name": "ABC Product",
        "category": "Electronics",
        "price": 150.00,
        "quantity": 1
      }
    ],
    "description": "ABC Tenant payment via İyzico",
    "use3D": false,
    "conversationId": "abc_conv_001"
  }'

echo -e "\n${GREEN}✅ ABC Tenant payment completed${NC}\n"

# 2. DEF Tenant Payment with different İyzico credentials
echo -e "${YELLOW}2. DEF Tenant Payment (Different İyzico Credentials)${NC}"
echo "--------------------------------------------------"
curl -X POST "${BASE_URL}/payments/iyzico" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: DEF" \
  -d '{
    "amount": 275.50,
    "currency": "TRY",
    "customer": {
      "id": "def_customer_001",
      "name": "Fatma",
      "surname": "Demir",
      "email": "fatma@def-enterprise.com",
      "phoneNumber": "+905559876543",
      "ipAddress": "192.168.1.200",
      "address": {
        "city": "Ankara",
        "country": "Turkey",
        "address": "DEF Enterprise Headquarters",
        "zipCode": "06000"
      }
    },
    "cardInfo": {
      "cardHolderName": "Fatma Demir",
      "cardNumber": "5528790000000008",
      "expireMonth": "11",
      "expireYear": "2029",
      "cvv": "456"
    },
    "items": [
      {
        "id": "def_product_premium",
        "name": "DEF Premium Service",
        "category": "Services",
        "price": 275.50,
        "quantity": 1
      }
    ],
    "description": "DEF Tenant premium service payment",
    "use3D": false,
    "conversationId": "def_conv_001"
  }'

echo -e "\n${GREEN}✅ DEF Tenant payment completed${NC}\n"

# 3. XYZ Tenant Payment with OzanPay
echo -e "${YELLOW}3. XYZ Tenant Payment (OzanPay Provider)${NC}"
echo "---------------------------------------"
curl -X POST "${BASE_URL}/payments/ozanpay" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: XYZ" \
  -d '{
    "amount": 320.75,
    "currency": "TRY",
    "customer": {
      "id": "xyz_customer_vip",
      "name": "Mehmet",
      "surname": "Özkan",
      "email": "mehmet@xyz-solutions.com",
      "phoneNumber": "+905551112233",
      "ipAddress": "192.168.1.300",
      "address": {
        "city": "İzmir",
        "country": "Turkey",
        "address": "XYZ Solutions Office",
        "zipCode": "35000"
      }
    },
    "cardInfo": {
      "cardHolderName": "Mehmet Özkan",
      "cardNumber": "4111111111111111",
      "expireMonth": "10",
      "expireYear": "2028",
      "cvv": "789"
    },
    "items": [
      {
        "id": "xyz_solution_package",
        "name": "XYZ Solution Package",
        "category": "Software",
        "price": 320.75,
        "quantity": 1
      }
    ],
    "description": "XYZ Tenant solution package purchase",
    "use3D": false,
    "conversationId": "xyz_conv_vip_001"
  }'

echo -e "\n${GREEN}✅ XYZ Tenant payment completed${NC}\n"

# 4. ENTERPRISE Tenant 3D Payment with İyzico
echo -e "${YELLOW}4. ENTERPRISE Tenant 3D Secure Payment (İyzico)${NC}"
echo "-----------------------------------------------"
curl -X POST "${BASE_URL}/payments/iyzico" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: ENTERPRISE" \
  -d '{
    "amount": 1500.00,
    "currency": "TRY",
    "customer": {
      "id": "enterprise_ceo",
      "name": "Ali",
      "surname": "Kaya",
      "email": "ali.kaya@enterprise-corp.com",
      "phoneNumber": "+905550001122",
      "ipAddress": "192.168.1.400",
      "address": {
        "city": "Istanbul",
        "country": "Turkey",
        "address": "Enterprise Corp Tower",
        "zipCode": "34200"
      }
    },
    "cardInfo": {
      "cardHolderName": "Ali Kaya",
      "cardNumber": "5528790000000008",
      "expireMonth": "09",
      "expireYear": "2027",
      "cvv": "321"
    },
    "items": [
      {
        "id": "enterprise_license",
        "name": "Enterprise License",
        "category": "License",
        "price": 1500.00,
        "quantity": 1
      }
    ],
    "description": "Enterprise annual license payment",
    "use3D": true,
    "callbackUrl": "https://enterprise-corp.com/payment-callback",
    "conversationId": "enterprise_license_2024"
  }'

echo -e "\n${GREEN}✅ ENTERPRISE Tenant 3D payment initiated${NC}\n"

# 5. Check Payment Status for Different Tenants
echo -e "${YELLOW}5. Payment Status Checks${NC}"
echo "-------------------------"

echo "Checking ABC tenant payment status:"
curl -X GET "${BASE_URL}/payments/iyzico/PAYMENT_ID_FROM_ABOVE" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: ABC"

echo -e "\n"
echo "Checking XYZ tenant payment status:"
curl -X GET "${BASE_URL}/payments/ozanpay/PAYMENT_ID_FROM_ABOVE" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: XYZ"

echo -e "\n${GREEN}✅ Status checks completed${NC}\n"

# 6. Refund Examples for Different Tenants
echo -e "${YELLOW}6. Tenant-Specific Refund Examples${NC}"
echo "----------------------------------"

echo "ABC Tenant refund (İyzico):"
curl -X POST "${BASE_URL}/payments/iyzico/refund" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: ABC" \
  -d '{
    "paymentId": "PAYMENT_ID_FROM_ABC_PAYMENT",
    "refundAmount": 50.00,
    "reason": "Product return",
    "description": "Customer returned item",
    "currency": "TRY",
    "conversationId": "abc_refund_001"
  }'

echo -e "\n"
echo "XYZ Tenant refund (OzanPay):"
curl -X POST "${BASE_URL}/payments/ozanpay/refund" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: XYZ" \
  -d '{
    "paymentId": "PAYMENT_ID_FROM_XYZ_PAYMENT",
    "refundAmount": 100.25,
    "reason": "Service cancellation",
    "description": "Early cancellation refund",
    "currency": "TRY",
    "conversationId": "xyz_refund_001"
  }'

echo -e "\n${GREEN}✅ Refund examples completed${NC}\n"

# 7. Configuration Management Examples
echo -e "${YELLOW}7. Configuration Management${NC}"
echo "----------------------------"

echo "Update ABC tenant configuration:"
curl -X POST "${BASE_URL}/set-env" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: ABC" \
  -d '{
    "IYZICO_ENVIRONMENT": "production"
  }'

echo -e "\n"
echo "Delete DEF tenant OzanPay config (if exists):"
curl -X DELETE "${BASE_URL}/config/tenant-config?provider=ozanpay" \
  -H "X-Tenant-ID: DEF"

echo -e "\n${GREEN}✅ Configuration management examples completed${NC}\n"

echo -e "${BLUE}Multi-Tenant Payment Examples Summary:${NC}"
echo "• ABC Tenant: Uses İyzico with sandbox credentials"
echo "• DEF Tenant: Uses İyzico with different sandbox credentials"
echo "• XYZ Tenant: Uses OzanPay with their own credentials"
echo "• ENTERPRISE Tenant: Uses İyzico with production credentials"
echo ""
echo -e "${BLUE}Key Points:${NC}"
echo "• Each tenant's payments are processed with their own provider configuration"
echo "• The X-Tenant-ID header routes requests to the correct configuration"
echo "• Tenants can use the same provider with different credentials"
echo "• All standard payment operations work per tenant" 