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
    L --> M["OpenSearch<br/>gopay-app1-iyzico-logs"]

    style A fill:#e1f5fe
    style B fill:#f3e5f5
    style C fill:#fff3e0
    style D fill:#ffebee
    style G fill:#e8f5e8
    style I fill:#e1f5fe
    style K fill:#fff9c4
    style M fill:#e8f5e8
```

### 📋 Payment Flow Steps:

1. **Application** sends payment request to **GoPay** with `X-Tenant-ID` header
2. **GoPay** forwards request to chosen **Provider** using tenant-specific configuration
3. **Provider** returns 3D Secure URL for user authentication (if required)
4. **User** completes 3D authentication on provider's page
5. **Provider** sends callback to **GoPay** with payment result
6. **GoPay** processes callback and redirects user back to **Application**
7. **Provider** sends webhook to **GoPay** for final confirmation
8. **GoPay** logs everything to **OpenSearch** in tenant-specific indexes

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
- **Webhook Validation**: Cryptographic verification of provider notifications

### 📊 **Monitoring & Analytics**

- **Real-time Logging**: OpenSearch integration for comprehensive tracking
- **Performance Metrics**: Provider-specific analytics
- **Dashboard**: Web-based monitoring interface
- **Audit Trails**: Complete request/response logging

## 🏪 Supported Payment Providers

| Provider    | Status         | Documentation                       | Features                    |
| ----------- | -------------- | ----------------------------------- | --------------------------- |
| **İyzico**  | ✅ Production  | [Guide](provider/iyzico/README.md)  | Payment, 3D, Refund, Cancel |
| **Stripe**  | ✅ Production  | [Guide](provider/stripe/README.md)  | Payment, 3D, Refund, Cancel |
| **OzanPay** | ✅ Production  | [Guide](provider/ozanpay/README.md) | Payment, 3D, Refund         |
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

### 2. **Configuration**

Set your environment variables:

```bash
# Core settings
API_KEY=your_super_secret_api_key
APP_URL=https://your-gopay-domain.com

# OpenSearch logging (optional)
OPENSEARCH_URL=http://localhost:9200
ENABLE_OPENSEARCH_LOGGING=true

# Provider credentials (example for İyzico)
IYZICO_API_KEY=your_iyzico_api_key
IYZICO_SECRET_KEY=your_iyzico_secret_key
IYZICO_ENVIRONMENT=sandbox
```

### 3. **Run Service**

```bash
# Using Docker (Recommended)
docker-compose up -d

# Or directly with Go
go run ./cmd/main.go

# Service will be available at http://localhost:9999
```

## 📡 API Endpoints

### 🔐 **Authenticated Endpoints**

```bash
# Payment Operations
POST   /v1/payments/{provider}              # Create payment
GET    /v1/payments/{provider}/{paymentID}  # Check status
DELETE /v1/payments/{provider}/{paymentID}  # Cancel payment
POST   /v1/payments/{provider}/refund       # Process refund

# Configuration Management
POST   /v1/set-env                          # Set tenant config
GET    /v1/config/tenant-config             # Get tenant config
DELETE /v1/config/tenant-config             # Delete tenant config

# Analytics & Monitoring
GET    /v1/analytics/dashboard              # Dashboard stats
GET    /v1/analytics/providers              # Provider stats
GET    /v1/logs/{provider}                  # Payment logs
GET    /v1/stats                           # General statistics
```

### 🌐 **Public Endpoints** (No Authentication)

```bash
# Callbacks & Webhooks
POST   /callback/{provider}                 # 3D Secure callbacks
GET    /callback/{provider}                 # 3D Secure callbacks
POST   /webhooks/{provider}                 # Payment webhooks

# System
GET    /health                              # Health check
GET    /                                    # Analytics dashboard
GET    /docs                                # API documentation
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

- **🔐 API Key Authentication** with Bearer tokens
- **🛡️ Rate Limiting** (configurable per endpoint)
- **🚨 IP Whitelisting** support
- **🔍 Request Validation** and size limits
- **📊 Audit Logging** for all operations
- **🔐 Webhook Signature Validation**

## 📊 Monitoring & Analytics

- **📈 Real-time Dashboard** at http://localhost:9999
- **🔍 OpenSearch Integration** for advanced analytics
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

This project uses a **dual license** approach:

### 🏗️ **Full Project License (MPL 2.0)**

- **File**: [LICENSE](LICENSE) - Mozilla Public License 2.0
- **Applies to**: Complete project, forks, distributions
- ✅ **Free to use** in personal and commercial projects
- ✅ **Contributions welcome** via pull requests
- ✅ **Modification allowed** with same license requirements
- ❌ **Redistribution** must maintain MPL 2.0 license
- ❌ **Proprietary forks** are not permitted

### 📦 **Go Package License (MIT)**

- **File**: [LICENSE.pkggo](LICENSE.pkggo) - MIT License
- **Applies to**: Go package usage via `go get` and pkg.go.dev
- ✅ **Liberal usage** as a Go library/dependency
- ✅ **Commercial integration** without restrictions
- ✅ **Compatible** with Go ecosystem standards

> **💡 Summary**: Use GoPay as a **Go library** → MIT applies. Fork/distribute the **full project** → MPL 2.0 applies.

## 🆘 Support

- **📖 Documentation**: Check the docs links above
- **🐛 Bug Reports**: [GitHub Issues](https://github.com/mstgnz/gopay/issues)
- **💬 Discussions**: [GitHub Discussions](https://github.com/mstgnz/gopay/discussions)
- **📧 Contact**: Create an issue for questions

---

**🚀 Ready to integrate payments?** Start with the [examples](examples/) or check the [API documentation](http://localhost:9999/docs)!
