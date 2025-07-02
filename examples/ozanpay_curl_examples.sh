#!/bin/bash

# OzanPay Payment Provider - Curl Examples
# Make sure your GoPay server is running on http://localhost:9999

BASE_URL="http://localhost:9999"
PROVIDER="ozanpay"

echo "🔸 OzanPay Payment Provider - Curl Examples"
echo "============================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Make sure your .env file contains OzanPay configuration:${NC}"
echo "OZANPAY_API_KEY=your-ozanpay-api-key"
echo "OZANPAY_SECRET_KEY=your-ozanpay-secret-key"
echo "OZANPAY_MERCHANT_ID=your-ozanpay-merchant-id"
echo "OZANPAY_ENVIRONMENT=sandbox"
echo ""

# 1. Regular Payment (Without 3D Secure)
echo -e "${YELLOW}1. Regular Payment (Without 3D Secure)${NC}"
echo "---------------------------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 150.75,
    "currency": "TRY",
    "customer": {
      "id": "ozan_customer123",
      "name": "Mehmet",
      "surname": "Özkan",
      "email": "mehmet@example.com",
      "phoneNumber": "+905551234567",
      "ipAddress": "192.168.1.1",
      "address": {
        "city": "Istanbul",
        "country": "Turkey",
        "address": "Kadıköy Test Address 123",
        "zipCode": "34710"
      }
    },
    "cardInfo": {
      "cardHolderName": "Mehmet Özkan",
      "cardNumber": "4111111111111111",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "ozan_item1",
        "name": "OzanPay Test Product",
        "category": "Electronics",
        "price": 150.75,
        "quantity": 1
      }
    ],
    "description": "Test payment via OzanPay",
    "use3D": false,
    "conversationId": "ozan_conv123"
  }'

echo -e "\n${GREEN}✅ Regular payment completed${NC}\n"

# 2. 3D Secure Payment
echo -e "${YELLOW}2. 3D Secure Payment${NC}"
echo "----------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 300.00,
    "currency": "TRY",
    "customer": {
      "id": "ozan_customer456",
      "name": "Fatma",
      "surname": "Demir",
      "email": "fatma@example.com",
      "phoneNumber": "+905559876543",
      "ipAddress": "192.168.1.2",
      "address": {
        "city": "Ankara",
        "country": "Turkey",
        "address": "Çankaya Test Address 456",
        "zipCode": "06690"
      }
    },
    "cardInfo": {
      "cardHolderName": "Fatma Demir",
      "cardNumber": "4000000000003220",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "ozan_item2",
        "name": "Premium OzanPay Product",
        "category": "Premium",
        "price": 300.00,
        "quantity": 1
      }
    ],
    "description": "3D Secure test payment via OzanPay",
    "use3D": true,
    "callbackUrl": "https://your-website.com/ozanpay-callback",
    "conversationId": "ozan_conv456"
  }'

echo -e "\n${GREEN}✅ 3D Secure payment initiated${NC}\n"

# 3. Payment Status Check
echo -e "${YELLOW}3. Payment Status Check${NC}"
echo "------------------------"
echo "Replace 'PAYMENT_ID' with actual payment ID from previous responses:"
curl -X GET "${BASE_URL}/payments/${PROVIDER}/PAYMENT_ID" \
  -H "Content-Type: application/json"

echo -e "\n${GREEN}✅ Payment status checked${NC}\n"

# 4. Cancel Payment
echo -e "${YELLOW}4. Cancel Payment${NC}"
echo "------------------"
echo "Replace 'PAYMENT_ID' with actual payment ID:"
curl -X DELETE "${BASE_URL}/payments/${PROVIDER}/PAYMENT_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "reason": "Customer cancellation request"
  }'

echo -e "\n${GREEN}✅ Payment cancellation requested${NC}\n"

# 5. Partial Refund
echo -e "${YELLOW}5. Partial Refund${NC}"
echo "-------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}/refund" \
  -H "Content-Type: application/json" \
  -d '{
    "paymentId": "PAYMENT_ID",
    "refundAmount": 75.50,
    "reason": "Partial product return",
    "description": "Customer returned one item",
    "currency": "TRY",
    "conversationId": "ozan_refund123"
  }'

echo -e "\n${GREEN}✅ Partial refund processed${NC}\n"

# 6. Full Refund
echo -e "${YELLOW}6. Full Refund${NC}"
echo "----------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}/refund" \
  -H "Content-Type: application/json" \
  -d '{
    "paymentId": "PAYMENT_ID",
    "reason": "Full order cancellation",
    "description": "Complete order refund",
    "currency": "TRY",
    "conversationId": "ozan_fullrefund123"
  }'

echo -e "\n${GREEN}✅ Full refund processed${NC}\n"

# 7. Marketplace Payment (Multiple Sub-merchants)
echo -e "${YELLOW}7. Marketplace Payment${NC}"
echo "-----------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 500.00,
    "currency": "TRY",
    "customer": {
      "id": "ozan_marketplace_customer",
      "name": "Ahmet",
      "surname": "Kaya",
      "email": "ahmet@example.com",
      "phoneNumber": "+905551112233",
      "ipAddress": "192.168.1.3",
      "address": {
        "city": "İzmir",
        "country": "Turkey",
        "address": "Bornova Marketplace Address",
        "zipCode": "35030"
      }
    },
    "cardInfo": {
      "cardHolderName": "Ahmet Kaya",
      "cardNumber": "4111111111111111",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "marketplace_item1",
        "name": "Seller A Product",
        "category": "Electronics",
        "price": 200.00,
        "quantity": 1
      },
      {
        "id": "marketplace_item2",
        "name": "Seller B Product",
        "category": "Fashion",
        "price": 150.00,
        "quantity": 1
      },
      {
        "id": "marketplace_item3",
        "name": "Platform Fee",
        "category": "Service",
        "price": 150.00,
        "quantity": 1
      }
    ],
    "description": "Marketplace payment with multiple sellers",
    "use3D": false,
    "paymentChannel": "MARKETPLACE",
    "conversationId": "ozan_marketplace123"
  }'

echo -e "\n${GREEN}✅ Marketplace payment completed${NC}\n"

# 8. Recurring Payment Setup
echo -e "${YELLOW}8. Recurring Payment${NC}"
echo "---------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 99.99,
    "currency": "TRY",
    "customer": {
      "id": "ozan_recurring_customer",
      "name": "Zeynep",
      "surname": "Çelik",
      "email": "zeynep@example.com",
      "phoneNumber": "+905554445566",
      "ipAddress": "192.168.1.4",
      "address": {
        "city": "Antalya",
        "country": "Turkey",
        "address": "Muratpaşa Recurring Address",
        "zipCode": "07160"
      }
    },
    "cardInfo": {
      "cardHolderName": "Zeynep Çelik",
      "cardNumber": "4111111111111111",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "subscription_item",
        "name": "Monthly Subscription",
        "category": "Subscription",
        "price": 99.99,
        "quantity": 1
      }
    ],
    "description": "Monthly subscription payment",
    "use3D": false,
    "paymentChannel": "RECURRING",
    "conversationId": "ozan_recurring123"
  }'

echo -e "\n${GREEN}✅ Recurring payment processed${NC}\n"

# 9. High Amount 3D Secure Payment
echo -e "${YELLOW}9. High Amount 3D Secure Payment${NC}"
echo "-----------------------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 2500.00,
    "currency": "TRY",
    "customer": {
      "id": "ozan_high_amount_customer",
      "name": "Mustafa",
      "surname": "Eren",
      "email": "mustafa@example.com",
      "phoneNumber": "+905557778899",
      "ipAddress": "192.168.1.5",
      "address": {
        "city": "Trabzon",
        "country": "Turkey",
        "address": "Ortahisar High Amount Address",
        "zipCode": "61040"
      }
    },
    "cardInfo": {
      "cardHolderName": "Mustafa Eren",
      "cardNumber": "4000000000003220",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "high_value_item",
        "name": "High Value Product",
        "category": "Luxury",
        "price": 2500.00,
        "quantity": 1
      }
    ],
    "description": "High amount 3D secure payment",
    "use3D": true,
    "callbackUrl": "https://your-website.com/ozanpay-high-amount-callback",
    "conversationId": "ozan_high_amount123"
  }'

echo -e "\n${GREEN}✅ High amount 3D secure payment initiated${NC}\n"

# 10. Currency Conversion Payment (USD to TRY)
echo -e "${YELLOW}10. Currency Conversion Payment${NC}"
echo "--------------------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 50.00,
    "currency": "USD",
    "customer": {
      "id": "ozan_forex_customer",
      "name": "İrem",
      "surname": "Güler",
      "email": "irem@example.com",
      "phoneNumber": "+905556667788",
      "ipAddress": "192.168.1.6",
      "address": {
        "city": "Eskişehir",
        "country": "Turkey",
        "address": "Tepebaşı Forex Address",
        "zipCode": "26110"
      }
    },
    "cardInfo": {
      "cardHolderName": "İrem Güler",
      "cardNumber": "4111111111111111",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "forex_item",
        "name": "International Product",
        "category": "Import",
        "price": 50.00,
        "quantity": 1
      }
    ],
    "description": "USD payment with currency conversion",
    "use3D": false,
    "conversationId": "ozan_forex123"
  }'

echo -e "\n${GREEN}✅ Currency conversion payment completed${NC}\n"

# Test with different OzanPay test cards
echo -e "${BLUE}OzanPay Test Cards:${NC}"
echo "Success Visa: 4111111111111111"
echo "Success Mastercard: 5555555555554444"
echo "3D Secure: 4000000000003220"
echo "Declined: 4000000000000002"
echo "Insufficient Funds: 4000000000009995"
echo "Invalid Expiry: 4000000000000069"
echo ""

echo -e "${RED}Note: Replace 'PAYMENT_ID' with actual payment IDs from responses${NC}"
echo -e "${RED}Make sure your GoPay server is running and OzanPay is configured${NC}" 