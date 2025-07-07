# GoPay

## 🚀 Unified Payment Integration Service

GoPay is a centralized payment gateway that abstracts multiple payment providers behind a single, standardized API. It acts as a bridge between your applications and payment providers, handling callbacks, webhooks, and logging seamlessly.

## 🎯 Why GoPay?

**Problem:** Every payment provider has different APIs, authentication methods, callback mechanisms, and response formats.

**Solution:** GoPay standardizes everything into one consistent interface.

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│                 │    │                 │    │                 │
│   Your Apps     │◄──►│     GoPay       │◄──►│   Payment       │
│  (APP1, APP2)   │    │   (Gateway)     │    │   Providers     │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🔄 Payment Flow & Architecture

### 🔄 **Basic Request-Response Flow**

GoPay acts as a standardized gateway between your applications and payment providers:

```mermaid
graph LR
    A["APP1, APP2, APP3<br/>🔹 Standard GoPay Request"] --> B["GoPay Gateway<br/>🔄 Request Translation"]
    B --> C["Payment Provider<br/>🏦 Provider-Specific Request"]
    C --> D["Provider Response<br/>💳 Provider-Specific Format"]
    D --> B2["GoPay Gateway<br/>🔄 Response Translation"]
    B2 --> E["APP1, APP2, APP3<br/>📋 Standard GoPay Response"]

    style A fill:#e1f5fe
    style B fill:#f3e5f5
    style B2 fill:#f3e5f5
    style C fill:#ffebee
    style D fill:#fff3e0
    style E fill:#e8f5e8
```

**Flow Explanation:**

1. **APP1** initiates payment → sends **standard GoPay request**
2. **GoPay** receives request → **translates to provider-specific format**
3. **GoPay** sends request → **provider processes payment**
4. **Provider** sends response → **GoPay receives provider response**
5. **GoPay** translates response → **returns standard GoPay response** to **APP1**

### 🔐 **Complete 3D Secure Flow**

For payments requiring 3D Secure authentication:

```mermaid
graph TD
    A["APP1<br/>X-Tenant-ID: APP1<br/>Provider: iyzico"] --> B["GoPay<br/>ProcessPayment"]
    B --> C["Provider: APP1_iyzico<br/>Config"]
    C --> D["İyzico<br/>Payment Request"]
    D --> E["3D Secure<br/>Bank Page"]
    E --> F["İyzico Callback<br/>to GoPay"]
    F --> G["GoPay Callback Handler<br/>/callback/iyzico?originalCallbackUrl=...&tenantId=APP1"]
    G --> H["Provider: APP1_iyzico<br/>Complete3D"]
    H --> I["APP1 Redirect<br/>originalCallbackUrl"]

    J["İyzico Webhook"] --> K["GoPay Webhook Handler<br/>/webhooks/iyzico?tenantId=APP1"]
    K --> L["Provider: APP1_iyzico<br/>ValidateWebhook"]
    L --> M["PostgreSQL<br/>gopay-payment-logs"]

    style A fill:#e1f5fe
    style B fill:#f3e5f5
    style C fill:#fff3e0
    style D fill:#ffebee
    style G fill:#e8f5e8
    style I fill:#e1f5fe
    style K fill:#fff9c4
    style M fill:#e8f5e8
```

### 📋 Complete Payment Flow Steps:

1. **Application** sends payment request to **GoPay** with `X-Tenant-ID` header
2. **GoPay** translates standard request to provider-specific format
3. **GoPay** forwards request to chosen **Provider** using tenant-specific configuration
4. **Provider** returns response (direct payment or 3D Secure URL)
5. **For 3D Secure**: User completes authentication on provider's page
6. **Provider** sends callback to **GoPay** with payment result
7. **GoPay** processes callback and redirects user back to **Application**
8. **Provider** sends webhook to **GoPay** for final confirmation
9. **GoPay** logs everything to **PostgreSQL** with structured logging

## 🌟 Core Capabilities

### 🏗️ **Multi-Tenant Architecture**

- **Tenant Isolation**: Each application uses separate provider configurations
- **Flexible Routing**: Support for multiple apps with different providers
- **Secure Separation**: Complete data isolation between tenants

### 🔄 **Environment Support**

- **Sandbox/Production**: Each provider supports both test and live environments
- **Dynamic Switching**: Different tenants can use different environments
- **Configuration Management**: Runtime configuration updates

### 🛡️ **Security & Reliability**

- **API Authentication**: Bearer token security
- **Rate Limiting**: Configurable limits per tenant/endpoint
- **IP Whitelisting**: Additional security layer
- **Request Validation** and size limits
- **Webhook Signature Validation**

### 📊 **Monitoring & Analytics**

- **Real-time Logging**: PostgreSQL integration for comprehensive tracking
- **Performance Metrics**: Provider-specific analytics
- **Dashboard**: Web-based monitoring interface
- **Audit Trails**: Complete request/response logging

## 🏪 Supported Payment Providers

| Provider    | Status         | Documentation                       | Features                    |
| ----------- | -------------- | ----------------------------------- | --------------------------- |
| **İyzico**  | ✅ Production  | [Guide](provider/iyzico/README.md)  | Payment, 3D, Refund, Cancel |
| **Stripe**  | ✅ Production  | [Guide](provider/stripe/README.md)  | Payment, 3D, Refund, Cancel |
| **OzanPay** | ✅ Production  | [Guide](provider/ozanpay/README.md) | Payment, 3D, Refund, Cancel |
| **Paycell** | ✅ Production  | [Guide](provider/paycell/README.md) | Payment, 3D, Refund, Cancel |
| **Papara**  | ✅ Production  | [Guide](provider/papara/README.md)  | Payment, 3D, Refund, Cancel |
| **Nkolay**  | ✅ Production  | [Guide](provider/nkolay/README.md)  | Payment, 3D, Refund, Cancel |
| **PayTR**   | ✅ Production  | [Guide](provider/paytr/README.md)   | Payment, 3D, Refund, Cancel |
| **PayU**    | ✅ Production  | [Guide](provider/payu/README.md)    | Payment, 3D, Refund, Cancel |
| **Shopier** | 🚧 Development | [Guide](provider/shopier/README.md) | Coming Soon                 |

## 🚦 Quick Start

### 1. **Installation**

```bash
git clone https://github.com/mstgnz/gopay.git
cd gopay

# Configure environment
cp .env.example .env
# Edit .env with your settings
```

### 2. **Run Service**

```bash
# Using Docker (Recommended)
docker-compose up -d

# Or directly with Go
go run ./cmd/main.go

# Service will be available at http://localhost:9999
```

## 💻 Usage Examples & Integration

Comprehensive examples and integration guides are available:

### 📁 **Examples Directory**

- **[Main Examples](examples/README.md)** - Complete integration examples
- **[İyzico Example](examples/iyzico_example.go)** - Go integration example
- **[Multi-Tenant Setup](examples/multi_tenant/)** - Multi-tenant examples
- **[cURL Examples](examples/)** - HTTP API examples for each provider

### 🔧 **Provider-Specific Examples**

- **[İyzico cURL Examples](examples/iyzico_curl_examples.sh)**
- **[OzanPay cURL Examples](examples/ozanpay_curl_examples.sh)**
- **[Paycell cURL Examples](examples/paycell_curl_examples.sh)**
- **[Papara cURL Examples](examples/papara_curl_examples.sh)**
- **[Multi-Tenant Setup Script](examples/multi_tenant_setup.sh)**

## 🏗️ Development & Deployment

### **Environment Setup**

```bash
# Install dependencies
go mod tidy

# Run tests
go test ./...

# Build binary
go build -o gopay ./cmd/main.go
```

### **Docker Deployment**

```bash
# Build and run with Docker Compose
docker-compose up -d

# Or build custom image
docker build -t gopay .
docker run -p 9999:9999 gopay
```

### **Kubernetes Deployment**

```bash
# Apply Kubernetes manifests
kubectl apply -f k8s/
```

## 📚 Documentation

- **🌐 API Documentation**: [Scalar UI](http://localhost:9999/docs) - Interactive API documentation
- **📖 Go Documentation**: Run `pkgsite -http=localhost:8081 .` for comprehensive Go package docs
- **📝 Provider Guides**: Individual provider documentation in `provider/*/README.md`
- **🔧 Examples**: Complete integration examples in [examples/](examples/)

## 🔒 Security Features

- **🔐 JWT Authentication** with auto-rotating secret keys
- **🛡️ Tenant-Based Rate Limiting** (configurable per endpoint)
- **🚨 IP Whitelisting** support
- **🔍 Request Validation** and size limits
- **📊 Audit Logging** for all operations
- **🔐 Webhook Signature Validation**

### 🔐 JWT Security Model

**Auto-Rotating Secret Keys:**

- JWT secret key regenerates on every service restart
- Enhanced security through key rotation
- Tokens become invalid after restart, requiring re-authentication
- No persistent secret key storage required

**Token Validation Flow:**

1. User authenticates → Receives JWT token (24h expiry)
2. Each API request → Token validated against current secret
3. Service restart → All tokens invalidated, users re-authenticate
4. No token persistence → Maximum security

## 📊 Monitoring & Analytics

- **📈 Real-time Dashboard** at http://localhost:9999
- **🔍 PostgreSQL Integration** for advanced analytics
- **📋 Structured Logging** with tenant isolation
- **⚡ Performance Metrics** per provider
- **🎯 Success/Error Rate Tracking**

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines:

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/new-provider`)
3. **Add tests** for your changes
4. **Submit** a pull request

### **Adding New Providers**

1. Implement the `provider.PaymentProvider` interface
2. Add provider package under `provider/{provider}/`
3. Create comprehensive README and tests
4. Register the provider in `provider/{provider}/register.go`

## 📄 License

This project is licensed under the [Business Source License 1.1 (BUSL-1.1)](./LICENSE).

### 🎯 **License Terms Summary:**

**✅ Allowed Uses:**

- ✅ Download and use the code
- ✅ Fork and develop improvements
- ✅ Submit pull requests
- ✅ Clone and install on VMs/servers
- ✅ Use for internal/non-production systems
- ✅ Modify and create derivative works

**❌ Restricted Uses:**

- ❌ Redistribute under a different name
- ❌ Fork into a public project (without written permission)
- ❌ Commercial redistribution/selling
- ❌ Production use without commercial license

**📅 Change Date:** January 1, 2030 - After this date, the license changes to Mozilla Public License 2.0

For commercial licensing or special arrangements, please contact: https://github.com/mstgnz/gopay

## 🆘 Support

- **📖 Documentation**: Check the docs links above
- **🐛 Bug Reports**: [GitHub Issues](https://github.com/mstgnz/gopay/issues)
- **❓ Questions**: Create an issue for questions and help
- **💡 Feature Requests**: Submit via GitHub Issues

---

**🚀 Ready to integrate payments?** Start with the [examples](examples/) or check the [API documentation](http://localhost:9999/docs)!
