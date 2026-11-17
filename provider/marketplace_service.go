package provider

import (
	"context"
	"strings"
)

// MarketplaceService runs sub-merchant and item-approval operations through a provider that has
// the MarketplaceProvider capability, logging each call like PaymentService.
type MarketplaceService struct {
	log operationLog
}

func NewMarketplaceService(logger PaymentLogger) *MarketplaceService {
	return &MarketplaceService{log: operationLog{logger: logger}}
}

func marketplaceProvider(ctx context.Context, providerName, environment string) (MarketplaceProvider, int, error) {
	tenantID, err := getTenantIDFromContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	p, err := GetProvider(tenantID, providerName, environment)
	if err != nil {
		return nil, 0, err
	}
	mp, ok := p.(MarketplaceProvider)
	if !ok {
		return nil, 0, ErrMarketplaceUnsupported
	}
	return mp, tenantID, nil
}

// CreateSubMerchant registers a seller and returns its key.
func (s *MarketplaceService) CreateSubMerchant(ctx context.Context, environment, providerName string, request SubMerchantRequest) (*SubMerchantResult, error) {
	mp, tenantID, err := marketplaceProvider(ctx, providerName, environment)
	if err != nil {
		return nil, err
	}
	if request.Key != "" {
		return nil, invalid("a new sub-merchant has no key yet")
	}
	if strings.TrimSpace(request.SubMerchant.ExternalID) == "" {
		return nil, invalid("externalId is required")
	}

	logID, started := s.log.start(ctx, tenantID, providerName, "POST", "/submerchants", request, "")
	request.LogID = logID
	resp, err := mp.CreateSubMerchant(ctx, request)
	s.log.finish(ctx, providerName, logID, started, resp, err)
	return resp, err
}

// UpdateSubMerchant changes a registered seller, found by its key.
func (s *MarketplaceService) UpdateSubMerchant(ctx context.Context, environment, providerName string, request SubMerchantRequest) (*SubMerchantResult, error) {
	mp, tenantID, err := marketplaceProvider(ctx, providerName, environment)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.Key) == "" {
		return nil, invalid("subMerchantKey is required")
	}

	logID, started := s.log.start(ctx, tenantID, providerName, "PUT", "/submerchants", request, "")
	request.LogID = logID
	resp, err := mp.UpdateSubMerchant(ctx, request)
	s.log.finish(ctx, providerName, logID, started, resp, err)
	return resp, err
}

// SubMerchantDetail finds a seller by the tenant's own id for it.
func (s *MarketplaceService) SubMerchantDetail(ctx context.Context, environment, providerName string, request SubMerchantDetailRequest) (*SubMerchantDetail, error) {
	mp, tenantID, err := marketplaceProvider(ctx, providerName, environment)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.ExternalID) == "" {
		return nil, invalid("externalId is required")
	}

	logID, started := s.log.start(ctx, tenantID, providerName, "GET", "/submerchants/detail", request, "")
	request.LogID = logID
	resp, err := mp.SubMerchantDetail(ctx, request)
	s.log.finish(ctx, providerName, logID, started, resp, err)
	return resp, err
}

// ApproveItem releases a paid item's sub-merchant share.
func (s *MarketplaceService) ApproveItem(ctx context.Context, environment, providerName string, request ItemApprovalRequest) (*ItemApprovalResult, error) {
	return s.itemApproval(ctx, environment, providerName, "/items/approve", request, MarketplaceProvider.ApproveItem)
}

// DisapproveItem withdraws an earlier approval.
func (s *MarketplaceService) DisapproveItem(ctx context.Context, environment, providerName string, request ItemApprovalRequest) (*ItemApprovalResult, error) {
	return s.itemApproval(ctx, environment, providerName, "/items/disapprove", request, MarketplaceProvider.DisapproveItem)
}

func (s *MarketplaceService) itemApproval(ctx context.Context, environment, providerName, endpoint string, request ItemApprovalRequest,
	call func(MarketplaceProvider, context.Context, ItemApprovalRequest) (*ItemApprovalResult, error)) (*ItemApprovalResult, error) {
	mp, tenantID, err := marketplaceProvider(ctx, providerName, environment)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.PaymentTransactionID) == "" {
		return nil, invalid("paymentTransactionId is required")
	}

	logID, started := s.log.start(ctx, tenantID, providerName, "POST", endpoint, request, "")
	request.LogID = logID
	resp, err := call(mp, ctx, request)
	s.log.finish(ctx, providerName, logID, started, resp, err)
	return resp, err
}
