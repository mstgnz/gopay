#!/bin/bash

# İyzico Payment Provider - Curl Examples
# Make sure your GoPay server is running on http://localhost:9999

BASE_URL="http://localhost:9999"
PROVIDER="iyzico"

echo "🔸 İyzico Payment Provider - Curl Examples"
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Make sure your .env file contains İyzico configuration:${NC}"
echo "IYZICO_API_KEY=your-iyzico-api-key"
echo "IYZICO_SECRET_KEY=your-iyzico-secret-key"
echo "IYZICO_ENVIRONMENT=sandbox"
echo ""

# 1. Regular Payment (Without 3D Secure)
echo -e "${YELLOW}1. Regular Payment (Without 3D Secure)${NC}"
echo "---------------------------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 100.50,
    "currency": "TRY",
    "customer": {
      "id": "customer123",
      "name": "John",
      "surname": "Doe",
      "email": "john@example.com",
      "phoneNumber": "+905551234567",
      "ipAddress": "192.168.1.1",
      "address": {
        "city": "Istanbul",
        "country": "Turkey",
        "address": "Test Address 123",
        "zipCode": "34000"
      }
    },
    "cardInfo": {
      "cardHolderName": "John Doe",
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "item1",
        "name": "Test Product",
        "category": "Electronics",
        "price": 100.50,
        "quantity": 1
      }
    ],
    "description": "Test payment via İyzico",
    "use3D": false,
    "conversationId": "conv123"
  }'

echo -e "\n${GREEN}✅ Regular payment completed${NC}\n"

# 2. 3D Secure Payment
echo -e "${YELLOW}2. 3D Secure Payment${NC}"
echo "----------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 250.00,
    "currency": "TRY",
    "customer": {
      "id": "customer456",
      "name": "Jane",
      "surname": "Smith",
      "email": "jane@example.com",
      "phoneNumber": "+905559876543",
      "ipAddress": "192.168.1.2",
      "address": {
        "city": "Ankara",
        "country": "Turkey",
        "address": "Test Address 456",
        "zipCode": "06000"
      }
    },
    "cardInfo": {
      "cardHolderName": "Jane Smith",
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "item2",
        "name": "Premium Product",
        "category": "Premium",
        "price": 250.00,
        "quantity": 1
      }
    ],
    "description": "3D Secure test payment via İyzico",
    "use3D": true,
    "callbackUrl": "https://your-website.com/payment-callback",
    "conversationId": "conv456"
  }'

echo -e "\n${GREEN}✅ 3D Secure payment initiated (check response for redirectUrl/html)${NC}\n"

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
    "reason": "Customer request"
  }'

echo -e "\n${GREEN}✅ Payment cancellation requested${NC}\n"

# 5. Refund Payment
echo -e "${YELLOW}5. Refund Payment${NC}"
echo "------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}/refund" \
  -H "Content-Type: application/json" \
  -d '{
    "paymentId": "PAYMENT_ID",
    "refundAmount": 50.25,
    "reason": "Product return",
    "description": "Partial refund for returned item",
    "currency": "TRY",
    "conversationId": "refund123"
  }'

echo -e "\n${GREEN}✅ Refund processed${NC}\n"

# 6. Full Refund
echo -e "${YELLOW}6. Full Refund (without refundAmount)${NC}"
echo "-------------------------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}/refund" \
  -H "Content-Type: application/json" \
  -d '{
    "paymentId": "PAYMENT_ID",
    "reason": "Order cancellation",
    "description": "Full refund for cancelled order",
    "currency": "TRY",
    "conversationId": "fullrefund123"
  }'

echo -e "\n${GREEN}✅ Full refund processed${NC}\n"

# 7. Multiple Items Payment
echo -e "${YELLOW}7. Multiple Items Payment${NC}"
echo "---------------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 350.75,
    "currency": "TRY",
    "customer": {
      "id": "customer789",
      "name": "Ali",
      "surname": "Veli",
      "email": "ali@example.com",
      "phoneNumber": "+905551112233",
      "ipAddress": "192.168.1.3",
      "address": {
        "city": "İzmir",
        "country": "Turkey",
        "address": "Test Address 789",
        "zipCode": "35000"
      }
    },
    "cardInfo": {
      "cardHolderName": "Ali Veli",
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "item3",
        "name": "Laptop",
        "category": "Electronics",
        "price": 250.50,
        "quantity": 1
      },
      {
        "id": "item4",
        "name": "Mouse",
        "category": "Electronics",
        "price": 50.25,
        "quantity": 1
      },
      {
        "id": "item5",
        "name": "Shipping",
        "category": "Service",
        "price": 50.00,
        "quantity": 1
      }
    ],
    "description": "Multiple items order",
    "use3D": false,
    "conversationId": "conv789"
  }'

echo -e "\n${GREEN}✅ Multiple items payment completed${NC}\n"

# 8. Payment with Installments
echo -e "${YELLOW}8. Payment with Installments${NC}"
echo "-----------------------------"
curl -X POST "${BASE_URL}/payments/${PROVIDER}" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 1200.00,
    "currency": "TRY",
    "customer": {
      "id": "customer999",
      "name": "Ayşe",
      "surname": "Yılmaz",
      "email": "ayse@example.com",
      "phoneNumber": "+905554445566",
      "ipAddress": "192.168.1.4",
      "address": {
        "city": "Bursa",
        "country": "Turkey",
        "address": "Test Address 999",
        "zipCode": "16000"
      }
    },
    "cardInfo": {
      "cardHolderName": "Ayşe Yılmaz",
      "cardNumber": "5528790000000008",
      "expireMonth": "12",
      "expireYear": "2030",
      "cvv": "123"
    },
    "items": [
      {
        "id": "item6",
        "name": "Expensive Product",
        "category": "Premium",
        "price": 1200.00,
        "quantity": 1
      }
    ],
    "description": "Installment payment test",
    "use3D": true,
    "callbackUrl": "https://your-website.com/payment-callback",
    "installmentCount": 6,
    "conversationId": "conv999"
  }'

echo -e "\n${GREEN}✅ Installment payment completed${NC}\n"

# Test with different İyzico test cards
echo -e "${BLUE}İyzico Test Cards:${NC}"
echo "Success: 5528790000000008"
echo "Insufficient funds: 5528790000000016"  
echo "Invalid card: 5528790000000024"
echo "Expired card: 5528790000000032"
echo "3D Secure: 5528790000000065"
echo ""

echo -e "${RED}Note: Replace 'PAYMENT_ID' with actual payment IDs from responses${NC}"
echo -e "${RED}Make sure your GoPay server is running and İyzico is configured${NC}" 