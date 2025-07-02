#!/bin/bash

# Paycell Payment Provider - Curl Examples
# Make sure your GoPay server is running on http://localhost:9999

BASE_URL="http://localhost:9999"
PROVIDER="paycell"

echo "🔸 Paycell Payment Provider - Curl Examples"
echo "============================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Make sure your .env file contains Paycell configuration:${NC}"
echo "PAYCELL_USERNAME=your-paycell-username"
echo "PAYCELL_PASSWORD=your-paycell-password"
echo "PAYCELL_MERCHANT_ID=your-paycell-merchant-id"
echo "PAYCELL_TERMINAL_ID=your-paycell-terminal-id"
echo "PAYCELL_ENVIRONMENT=sandbox"
echo ""

# 1. Regular Payment (Without 3D Secure)
echo -e "${YELLOW}1. Regular Payment (Without 3D Secure)${NC}"
echo "---------------------------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 125.50,
    "currency": "TRY",
    "customer": {
      "id": "paycell_customer123",
      "name": "Emre",
      "surname": "Yılmaz",
      "email": "emre@example.com",
      "phoneNumber": "+905551234567",
      "ipAddress": "192.168.1.1",
      "address": {
        "city": "Istanbul",
        "country": "Turkey",
        "address": "Şişli Paycell Test Address 123",
        "zipCode": "34360"
      }
    },
    "cardInfo": {
      "cardHolderName": "Emre Yılmaz",
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "paycell_item1",
        "name": "Paycell Test Product",
        "category": "Technology",
        "price": 125.50,
        "quantity": 1
      }
    ],
    "description": "Test payment via Paycell",
    "use3D": false,
    "conversationId": "paycell_conv123"
  }'

echo -e "\n${GREEN}✅ Regular payment completed${NC}\n"

# 2. 3D Secure Payment
echo -e "${YELLOW}2. 3D Secure Payment${NC}"
echo "----------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 275.25,
    "currency": "TRY",
    "customer": {
      "id": "paycell_customer456",
      "name": "Selin",
      "surname": "Kaya",
      "email": "selin@example.com",
      "phoneNumber": "+905559876543",
      "ipAddress": "192.168.1.2",
      "address": {
        "city": "Ankara",
        "country": "Turkey",
        "address": "Kızılay Paycell 3D Address 456",
        "zipCode": "06420"
      }
    },
    "cardInfo": {
      "cardHolderName": "Selin Kaya",
      "cardNumber": "5528790000000057",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "paycell_3d_item",
        "name": "Secure Paycell Product",
        "category": "Premium",
        "price": 275.25,
        "quantity": 1
      }
    ],
    "description": "3D Secure test payment via Paycell",
    "use3D": true,
    "callbackUrl": "https://your-website.com/paycell-callback",
    "conversationId": "paycell_3d_conv456"
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
    "reason": "Customer requested cancellation"
  }'

echo -e "\n${GREEN}✅ Payment cancellation requested${NC}\n"

# 5. Partial Refund
echo -e "${YELLOW}5. Partial Refund${NC}"
echo "-------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}/refund" \
  -H "Content-Type: application/json" \
  -d '{
    "paymentId": "PAYMENT_ID",
    "refundAmount": 62.75,
    "reason": "Defective item return",
    "description": "Partial refund for damaged product",
    "currency": "TRY",
    "conversationId": "paycell_partial_refund123"
  }'

echo -e "\n${GREEN}✅ Partial refund processed${NC}\n"

# 6. Full Refund
echo -e "${YELLOW}6. Full Refund${NC}"
echo "----------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}/refund" \
  -H "Content-Type: application/json" \
  -d '{
    "paymentId": "PAYMENT_ID",
    "reason": "Complete order cancellation",
    "description": "Full refund for cancelled order",
    "currency": "TRY",
    "conversationId": "paycell_full_refund123"
  }'

echo -e "\n${GREEN}✅ Full refund processed${NC}\n"

# 7. Mobile Payment (Turkish Market Focus)
echo -e "${YELLOW}7. Mobile Payment${NC}"
echo "-------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 199.99,
    "currency": "TRY",
    "customer": {
      "id": "paycell_mobile_customer",
      "name": "Burak",
      "surname": "Özcan",
      "email": "burak@example.com",
      "phoneNumber": "+905551112233",
      "ipAddress": "192.168.1.3",
      "address": {
        "city": "İzmir",
        "country": "Turkey",
        "address": "Konak Mobile Payment Address",
        "zipCode": "35250"
      }
    },
    "cardInfo": {
      "cardHolderName": "Burak Özcan",
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "mobile_app_purchase",
        "name": "Mobile App Premium",
        "category": "Digital",
        "price": 199.99,
        "quantity": 1
      }
    ],
    "description": "Mobile app premium subscription",
    "use3D": false,
    "paymentChannel": "MOBILE",
    "conversationId": "paycell_mobile123"
  }'

echo -e "\n${GREEN}✅ Mobile payment completed${NC}\n"

# 8. Installment Payment (Turkish Banks)
echo -e "${YELLOW}8. Installment Payment${NC}"
echo "------------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 1500.00,
    "currency": "TRY",
    "customer": {
      "id": "paycell_installment_customer",
      "name": "Deniz",
      "surname": "Aktaş",
      "email": "deniz@example.com",
      "phoneNumber": "+905554445566",
      "ipAddress": "192.168.1.4",
      "address": {
        "city": "Bursa",
        "country": "Turkey",
        "address": "Nilüfer Installment Address",
        "zipCode": "16110"
      }
    },
    "cardInfo": {
      "cardHolderName": "Deniz Aktaş",
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "installment_product",
        "name": "High Value Electronics",
        "category": "Electronics",
        "price": 1500.00,
        "quantity": 1
      }
    ],
    "description": "Installment payment for electronics",
    "use3D": true,
    "callbackUrl": "https://your-website.com/paycell-installment-callback",
    "installmentCount": 9,
    "conversationId": "paycell_installment123"
  }'

echo -e "\n${GREEN}✅ Installment payment initiated${NC}\n"

# 9. Error Testing - Insufficient Funds
echo -e "${YELLOW}9. Error Testing - Insufficient Funds${NC}"
echo "--------------------------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 100.00,
    "currency": "TRY",
    "customer": {
      "id": "paycell_error_customer",
      "name": "Test",
      "surname": "Error",
      "email": "test@example.com",
      "phoneNumber": "+905556667788",
      "ipAddress": "192.168.1.5",
      "address": {
        "city": "Adana",
        "country": "Turkey",
        "address": "Error Test Address",
        "zipCode": "01160"
      }
    },
    "cardInfo": {
      "cardHolderName": "Test Error",
      "cardNumber": "5528790000000016",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "error_test_item",
        "name": "Error Test Product",
        "category": "Test",
        "price": 100.00,
        "quantity": 1
      }
    ],
    "description": "Testing insufficient funds error",
    "use3D": false,
    "conversationId": "paycell_error123"
  }'

echo -e "\n${GREEN}✅ Error test completed (should show insufficient funds)${NC}\n"

# 10. Timeout Testing
echo -e "${YELLOW}10. Timeout Testing${NC}"
echo "---------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 999.99,
    "currency": "TRY",
    "customer": {
      "id": "paycell_timeout_customer",
      "name": "Timeout",
      "surname": "Test",
      "email": "timeout@example.com",
      "phoneNumber": "+905557778899",
      "ipAddress": "192.168.1.6",
      "address": {
        "city": "Gaziantep",
        "country": "Turkey",
        "address": "Timeout Test Address",
        "zipCode": "27090"
      }
    },
    "cardInfo": {
      "cardHolderName": "Timeout Test",
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "timeout_test_item",
        "name": "Timeout Test Product",
        "category": "Test",
        "price": 999.99,
        "quantity": 1
      }
    ],
    "description": "Testing timeout scenario",
    "use3D": false,
    "conversationId": "paycell_timeout123"
  }'

echo -e "\n${GREEN}✅ Timeout test initiated (should timeout after 35s)${NC}\n"

# 11. Multiple Currency Support
echo -e "${YELLOW}11. Multiple Currency Support (EUR)${NC}"
echo "-------------------------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 25.50,
    "currency": "EUR",
    "customer": {
      "id": "paycell_eur_customer",
      "name": "Can",
      "surname": "Doğan",
      "email": "can@example.com",
      "phoneNumber": "+905558889900",
      "ipAddress": "192.168.1.7",
      "address": {
        "city": "Antalya",
        "country": "Turkey",
        "address": "Muratpaşa EUR Address",
        "zipCode": "07230"
      }
    },
    "cardInfo": {
      "cardHolderName": "Can Doğan",
      "cardNumber": "4111111111111111",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "eur_item",
        "name": "European Product",
        "category": "International",
        "price": 25.50,
        "quantity": 1
      }
    ],
    "description": "EUR currency payment test",
    "use3D": false,
    "conversationId": "paycell_eur123"
  }'

echo -e "\n${GREEN}✅ EUR payment completed${NC}\n"

# Test with different Paycell test cards
echo -e "${BLUE}Paycell Test Cards:${NC}"
echo "Success: 5528790000000008"
echo "3D Secure: 5528790000000057"
echo "Insufficient funds: 5528790000000016"
echo "Invalid card: 5528790000000024"
echo "Expired card: 5528790000000032"
echo "Declined: 5528790000000040"
echo "Visa Success: 4111111111111111"
echo "Visa 3D: 4000000000003220"
echo ""

echo -e "${BLUE}Test Amounts:${NC}"
echo "999.99 - Triggers timeout after 35 seconds"
echo "0.01 - Minimum amount test"
echo "9999.99 - Maximum amount test"
echo ""

echo -e "${RED}Note: Replace 'PAYMENT_ID' with actual payment IDs from responses${NC}"
echo -e "${RED}Make sure your GoPay server is running and Paycell is configured${NC}" 