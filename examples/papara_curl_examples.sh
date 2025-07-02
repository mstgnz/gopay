#!/bin/bash

# Papara Payment Provider - cURL Examples
# 
# This script demonstrates how to use the Papara payment provider
# through the GoPay unified payment API.
#
# Prerequisites:
# 1. GoPay service running on localhost:9999
# 2. Valid Papara API key configured
# 3. API_KEY environment variable set for GoPay authentication
#
# Usage: ./papara_curl_examples.sh

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
GOPAY_BASE_URL="${GOPAY_BASE_URL:-http://localhost:9999}"
API_KEY="${API_KEY:-your_gopay_api_key_here}"

# Check if API key is set
if [ "$API_KEY" == "your_gopay_api_key_here" ]; then
    echo -e "${RED}Error: Please set your API_KEY environment variable${NC}"
    echo "Example: export API_KEY=your_actual_api_key"
    exit 1
fi

# Common headers
HEADERS=(
    -H "Authorization: Bearer $API_KEY"
    -H "Content-Type: application/json"
    -H "Accept: application/json"
)

echo -e "${BLUE}==================================${NC}"
echo -e "${BLUE}  Papara Payment Provider Tests  ${NC}"
echo -e "${BLUE}==================================${NC}"
echo ""

# Global variables for test data
PAYMENT_ID=""
REFERENCE_ID="papara-test-$(date +%Y%m%d%H%M%S)"

# Function to print section headers
print_section() {
    echo -e "${YELLOW}$1${NC}"
    echo "----------------------------------------"
}

# Function to print test results
print_result() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓ Success${NC}"
    else
        echo -e "${RED}✗ Failed${NC}"
    fi
    echo ""
}

# Function to extract payment ID from response
extract_payment_id() {
    echo "$1" | grep -o '"paymentId":"[^"]*"' | sed 's/"paymentId":"\([^"]*\)"/\1/'
}

# 1. BASIC PAYMENT
print_section "1. Basic Payment (Non-3D)"
echo "Creating a basic payment without 3D Secure..."

BASIC_PAYMENT_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    "${HEADERS[@]}" \
    -X POST \
    "$GOPAY_BASE_URL/v1/payments/papara" \
    -d '{
        "amount": 100.50,
        "currency": "TRY",
        "referenceId": "'$REFERENCE_ID'-basic",
        "description": "Papara Basic Payment Test",
        "customer": {
            "name": "Ahmet",
            "surname": "Yılmaz",
            "email": "ahmet.yilmaz@example.com",
            "phoneNumber": "+905551234567",
            "address": {
                "city": "Istanbul",
                "country": "Turkey",
                "address": "Test Mahallesi, Test Sokak No:1",
                "zipCode": "34000"
            }
        },
        "items": [
            {
                "id": "item-1",
                "name": "Test Ürün",
                "description": "Test ürün açıklaması",
                "category": "Elektronik",
                "price": 100.50,
                "quantity": 1
            }
        ]
    }')

HTTP_STATUS=$(echo "$BASIC_PAYMENT_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
RESPONSE_BODY=$(echo "$BASIC_PAYMENT_RESPONSE" | sed '/HTTP_STATUS:/d')

echo "Response Status: $HTTP_STATUS"
echo "Response: $RESPONSE_BODY"

if [ "$HTTP_STATUS" == "200" ] || [ "$HTTP_STATUS" == "201" ]; then
    PAYMENT_ID=$(extract_payment_id "$RESPONSE_BODY")
    echo "Payment ID: $PAYMENT_ID"
    print_result 0
else
    print_result 1
fi

# 2. 3D SECURE PAYMENT
print_section "2. 3D Secure Payment"
echo "Creating a 3D Secure payment..."

SECURE_PAYMENT_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    "${HEADERS[@]}" \
    -X POST \
    "$GOPAY_BASE_URL/v1/payments/papara" \
    -d '{
        "amount": 250.75,
        "currency": "TRY",
        "use3D": true,
        "callbackUrl": "https://yourwebsite.com/payment-success",
        "referenceId": "'$REFERENCE_ID'-3d",
        "description": "Papara 3D Secure Payment Test",
        "customer": {
            "name": "Mehmet",
            "surname": "Demir",
            "email": "mehmet.demir@example.com",
            "phoneNumber": "+905559876543",
            "address": {
                "city": "Ankara",
                "country": "Turkey",
                "address": "Çankaya Mahallesi, Atatürk Bulvarı No:123",
                "zipCode": "06000"
            }
        }
    }')

HTTP_STATUS=$(echo "$SECURE_PAYMENT_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
RESPONSE_BODY=$(echo "$SECURE_PAYMENT_RESPONSE" | sed '/HTTP_STATUS:/d')

echo "Response Status: $HTTP_STATUS"
echo "Response: $RESPONSE_BODY"

if [ "$HTTP_STATUS" == "200" ] || [ "$HTTP_STATUS" == "201" ]; then
    SECURE_PAYMENT_ID=$(extract_payment_id "$RESPONSE_BODY")
    echo "3D Secure Payment ID: $SECURE_PAYMENT_ID"
    print_result 0
else
    print_result 1
fi

# 3. GET PAYMENT STATUS
if [ ! -z "$PAYMENT_ID" ]; then
    print_section "3. Get Payment Status"
    echo "Retrieving payment status for Payment ID: $PAYMENT_ID"

    STATUS_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
        "${HEADERS[@]}" \
        -X GET \
        "$GOPAY_BASE_URL/v1/payments/papara/$PAYMENT_ID")

    HTTP_STATUS=$(echo "$STATUS_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
    RESPONSE_BODY=$(echo "$STATUS_RESPONSE" | sed '/HTTP_STATUS:/d')

    echo "Response Status: $HTTP_STATUS"
    echo "Response: $RESPONSE_BODY"
    print_result $([ "$HTTP_STATUS" == "200" ] && echo 0 || echo 1)
else
    echo -e "${YELLOW}Skipping payment status check - no payment ID available${NC}"
    echo ""
fi

# 4. PARTIAL REFUND
if [ ! -z "$PAYMENT_ID" ]; then
    print_section "4. Partial Refund"
    echo "Processing partial refund for Payment ID: $PAYMENT_ID"

    REFUND_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
        "${HEADERS[@]}" \
        -X POST \
        "$GOPAY_BASE_URL/v1/payments/papara/refund" \
        -d '{
            "paymentId": "'$PAYMENT_ID'",
            "refundAmount": 50.25,
            "reason": "Kısmi iade talebi",
            "description": "Müşteri talep ettiği için kısmi iade",
            "currency": "TRY"
        }')

    HTTP_STATUS=$(echo "$REFUND_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
    RESPONSE_BODY=$(echo "$REFUND_RESPONSE" | sed '/HTTP_STATUS:/d')

    echo "Response Status: $HTTP_STATUS"
    echo "Response: $RESPONSE_BODY"
    print_result $([ "$HTTP_STATUS" == "200" ] && echo 0 || echo 1)
else
    echo -e "${YELLOW}Skipping refund test - no payment ID available${NC}"
    echo ""
fi

# 5. FULL REFUND
if [ ! -z "$SECURE_PAYMENT_ID" ]; then
    print_section "5. Full Refund"
    echo "Processing full refund for Payment ID: $SECURE_PAYMENT_ID"

    FULL_REFUND_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
        "${HEADERS[@]}" \
        -X POST \
        "$GOPAY_BASE_URL/v1/payments/papara/refund" \
        -d '{
            "paymentId": "'$SECURE_PAYMENT_ID'",
            "reason": "Tam iade talebi",
            "description": "Sipariş iptali - tam iade",
            "currency": "TRY"
        }')

    HTTP_STATUS=$(echo "$FULL_REFUND_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
    RESPONSE_BODY=$(echo "$FULL_REFUND_RESPONSE" | sed '/HTTP_STATUS:/d')

    echo "Response Status: $HTTP_STATUS"
    echo "Response: $RESPONSE_BODY"
    print_result $([ "$HTTP_STATUS" == "200" ] && echo 0 || echo 1)
else
    echo -e "${YELLOW}Skipping full refund test - no 3D secure payment ID available${NC}"
    echo ""
fi

# 6. CANCEL PAYMENT
print_section "6. Cancel Payment"
echo "Creating a payment to cancel..."

CANCEL_PAYMENT_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    "${HEADERS[@]}" \
    -X POST \
    "$GOPAY_BASE_URL/v1/payments/papara" \
    -d '{
        "amount": 75.00,
        "currency": "TRY",
        "referenceId": "'$REFERENCE_ID'-cancel",
        "description": "Papara Payment to Cancel",
        "customer": {
            "name": "Fatma",
            "surname": "Kaya",
            "email": "fatma.kaya@example.com"
        }
    }')

HTTP_STATUS=$(echo "$CANCEL_PAYMENT_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
RESPONSE_BODY=$(echo "$CANCEL_PAYMENT_RESPONSE" | sed '/HTTP_STATUS:/d')

if [ "$HTTP_STATUS" == "200" ] || [ "$HTTP_STATUS" == "201" ]; then
    CANCEL_PAYMENT_ID=$(extract_payment_id "$RESPONSE_BODY")
    echo "Payment created for cancellation. ID: $CANCEL_PAYMENT_ID"
    
    echo "Now cancelling the payment..."
    
    CANCEL_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
        "${HEADERS[@]}" \
        -X DELETE \
        "$GOPAY_BASE_URL/v1/payments/papara/$CANCEL_PAYMENT_ID")

    HTTP_STATUS=$(echo "$CANCEL_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
    RESPONSE_BODY=$(echo "$CANCEL_RESPONSE" | sed '/HTTP_STATUS:/d')

    echo "Cancel Response Status: $HTTP_STATUS"
    echo "Cancel Response: $RESPONSE_BODY"
    print_result $([ "$HTTP_STATUS" == "200" ] && echo 0 || echo 1)
else
    echo "Failed to create payment for cancellation test"
    print_result 1
fi

# 7. PAYMENT WITH INSTALLMENTS
print_section "7. Payment with Installments"
echo "Creating payment with installment options..."

INSTALLMENT_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    "${HEADERS[@]}" \
    -X POST \
    "$GOPAY_BASE_URL/v1/payments/papara" \
    -d '{
        "amount": 500.00,
        "currency": "TRY",
        "installmentCount": 3,
        "referenceId": "'$REFERENCE_ID'-installment",
        "description": "Papara Installment Payment Test",
        "customer": {
            "name": "Caner",
            "surname": "Özkan",
            "email": "caner.ozkan@example.com",
            "phoneNumber": "+905551112233"
        }
    }')

HTTP_STATUS=$(echo "$INSTALLMENT_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
RESPONSE_BODY=$(echo "$INSTALLMENT_RESPONSE" | sed '/HTTP_STATUS:/d')

echo "Response Status: $HTTP_STATUS"
echo "Response: $RESPONSE_BODY"
print_result $([ "$HTTP_STATUS" == "200" ] || [ "$HTTP_STATUS" == "201" ] && echo 0 || echo 1)

# 8. INVALID PAYMENT TEST
print_section "8. Invalid Payment Test"
echo "Testing error handling with invalid payment data..."

INVALID_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    "${HEADERS[@]}" \
    -X POST \
    "$GOPAY_BASE_URL/v1/payments/papara" \
    -d '{
        "amount": -10,
        "currency": "",
        "referenceId": "",
        "description": "",
        "customer": {}
    }')

HTTP_STATUS=$(echo "$INVALID_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
RESPONSE_BODY=$(echo "$INVALID_RESPONSE" | sed '/HTTP_STATUS:/d')

echo "Response Status: $HTTP_STATUS"
echo "Response: $RESPONSE_BODY"

# For invalid data, we expect a 400 error
if [ "$HTTP_STATUS" == "400" ]; then
    echo -e "${GREEN}✓ Correctly handled invalid data${NC}"
else
    echo -e "${RED}✗ Expected 400 status for invalid data${NC}"
fi
echo ""

# 9. PAYMENT STATUS FOR NON-EXISTENT PAYMENT
print_section "9. Non-existent Payment Status Test"
echo "Testing payment status for non-existent payment..."

NONEXISTENT_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    "${HEADERS[@]}" \
    -X GET \
    "$GOPAY_BASE_URL/v1/payments/papara/nonexistent-payment-id-12345")

HTTP_STATUS=$(echo "$NONEXISTENT_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
RESPONSE_BODY=$(echo "$NONEXISTENT_RESPONSE" | sed '/HTTP_STATUS:/d')

echo "Response Status: $HTTP_STATUS"
echo "Response: $RESPONSE_BODY"

# For non-existent payment, we expect a 404 error
if [ "$HTTP_STATUS" == "404" ]; then
    echo -e "${GREEN}✓ Correctly handled non-existent payment${NC}"
else
    echo -e "${RED}✗ Expected 404 status for non-existent payment${NC}"
fi
echo ""

# 10. WEBHOOK SIMULATION
print_section "10. Webhook Simulation"
echo "Simulating webhook callback processing..."

WEBHOOK_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
    "${HEADERS[@]}" \
    -X POST \
    "$GOPAY_BASE_URL/v1/callback/papara" \
    -H "X-Papara-Signature: test-signature" \
    -d '{
        "paymentId": "test-webhook-payment-123",
        "status": "COMPLETED",
        "amount": 150.00,
        "currency": "TRY",
        "referenceId": "webhook-test-ref",
        "timestamp": "'$(date +%s)'"
    }')

HTTP_STATUS=$(echo "$WEBHOOK_RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
RESPONSE_BODY=$(echo "$WEBHOOK_RESPONSE" | sed '/HTTP_STATUS:/d')

echo "Response Status: $HTTP_STATUS"
echo "Response: $RESPONSE_BODY"
print_result $([ "$HTTP_STATUS" == "200" ] && echo 0 || echo 1)

# Summary
echo -e "${BLUE}==================================${NC}"
echo -e "${BLUE}           Test Summary           ${NC}"
echo -e "${BLUE}==================================${NC}"
echo "Tests completed for Papara payment provider"
echo ""
echo -e "${YELLOW}Note:${NC} Some tests may fail if:"
echo "- Papara API credentials are not properly configured"
echo "- GoPay service is not running"
echo "- Network connectivity issues"
echo "- Invalid API keys"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Check GoPay logs for detailed error information"
echo "2. Verify Papara API credentials in environment variables"
echo "3. Test with different payment amounts and scenarios"
echo "4. Configure webhooks in Papara merchant panel"
echo ""
echo -e "${GREEN}For more information, see:${NC}"
echo "- Papara Provider Documentation: provider/papara/README.md"
echo "- GoPay Main Documentation: README.md" 