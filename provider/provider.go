package provider

import (
	"context"
	"fmt"
	"reflect"
)

// GenericProvider is a generic wrapper for any type that has methods matching
// the PaymentProvider interface signature.
type GenericProvider[T any] struct {
	provider T
}

// NewGenericProvider creates a new GenericProvider wrapping the given provider
func NewGenericProvider[T any](provider T) *GenericProvider[T] {
	return &GenericProvider[T]{
		provider: provider,
	}
}

// Initialize sets up the payment provider with authentication and configuration
func (g *GenericProvider[T]) Initialize(config map[string]string) error {
	method := reflect.ValueOf(g.provider).MethodByName("Initialize")
	if !method.IsValid() {
		return fmt.Errorf("provider does not implement Initialize method")
	}

	results := method.Call([]reflect.Value{reflect.ValueOf(config)})
	if len(results) > 0 && !results[0].IsNil() {
		return results[0].Interface().(error)
	}

	return nil
}

// CreatePayment makes a non-3D payment request
func (g *GenericProvider[T]) CreatePayment(ctx context.Context, request PaymentRequest) (*PaymentResponse, error) {
	method := reflect.ValueOf(g.provider).MethodByName("CreatePayment")
	if !method.IsValid() {
		return nil, fmt.Errorf("provider does not implement CreatePayment method")
	}

	results := method.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(request)})

	var response *PaymentResponse
	if !results[0].IsNil() {
		response = results[0].Interface().(*PaymentResponse)
	}

	var err error
	if !results[1].IsNil() {
		err = results[1].Interface().(error)
	}

	return response, err
}

// Create3DPayment starts a 3D secure payment process
func (g *GenericProvider[T]) Create3DPayment(ctx context.Context, request PaymentRequest) (*PaymentResponse, error) {
	method := reflect.ValueOf(g.provider).MethodByName("Create3DPayment")
	if !method.IsValid() {
		return nil, fmt.Errorf("provider does not implement Create3DPayment method")
	}

	results := method.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(request)})

	var response *PaymentResponse
	if !results[0].IsNil() {
		response = results[0].Interface().(*PaymentResponse)
	}

	var err error
	if !results[1].IsNil() {
		err = results[1].Interface().(error)
	}

	return response, err
}

// Complete3DPayment completes a 3D secure payment after user authentication
func (g *GenericProvider[T]) Complete3DPayment(ctx context.Context, paymentID string, conversationID string, data map[string]string) (*PaymentResponse, error) {
	method := reflect.ValueOf(g.provider).MethodByName("Complete3DPayment")
	if !method.IsValid() {
		return nil, fmt.Errorf("provider does not implement Complete3DPayment method")
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(paymentID),
		reflect.ValueOf(conversationID),
		reflect.ValueOf(data),
	})

	var response *PaymentResponse
	if !results[0].IsNil() {
		response = results[0].Interface().(*PaymentResponse)
	}

	var err error
	if !results[1].IsNil() {
		err = results[1].Interface().(error)
	}

	return response, err
}

// GetPaymentStatus retrieves the current status of a payment
func (g *GenericProvider[T]) GetPaymentStatus(ctx context.Context, paymentID string) (*PaymentResponse, error) {
	method := reflect.ValueOf(g.provider).MethodByName("GetPaymentStatus")
	if !method.IsValid() {
		return nil, fmt.Errorf("provider does not implement GetPaymentStatus method")
	}

	results := method.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(paymentID)})

	var response *PaymentResponse
	if !results[0].IsNil() {
		response = results[0].Interface().(*PaymentResponse)
	}

	var err error
	if !results[1].IsNil() {
		err = results[1].Interface().(error)
	}

	return response, err
}

// CancelPayment cancels a payment
func (g *GenericProvider[T]) CancelPayment(ctx context.Context, paymentID string, reason string) (*PaymentResponse, error) {
	method := reflect.ValueOf(g.provider).MethodByName("CancelPayment")
	if !method.IsValid() {
		return nil, fmt.Errorf("provider does not implement CancelPayment method")
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(paymentID),
		reflect.ValueOf(reason),
	})

	var response *PaymentResponse
	if !results[0].IsNil() {
		response = results[0].Interface().(*PaymentResponse)
	}

	var err error
	if !results[1].IsNil() {
		err = results[1].Interface().(error)
	}

	return response, err
}

// RefundPayment issues a refund for a payment
func (g *GenericProvider[T]) RefundPayment(ctx context.Context, request RefundRequest) (*RefundResponse, error) {
	method := reflect.ValueOf(g.provider).MethodByName("RefundPayment")
	if !method.IsValid() {
		return nil, fmt.Errorf("provider does not implement RefundPayment method")
	}

	results := method.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(request)})

	var response *RefundResponse
	if !results[0].IsNil() {
		response = results[0].Interface().(*RefundResponse)
	}

	var err error
	if !results[1].IsNil() {
		err = results[1].Interface().(error)
	}

	return response, err
}

// ValidateWebhook validates an incoming webhook notification
func (g *GenericProvider[T]) ValidateWebhook(ctx context.Context, data map[string]string, headers map[string]string) (bool, map[string]string, error) {
	method := reflect.ValueOf(g.provider).MethodByName("ValidateWebhook")
	if !method.IsValid() {
		return false, nil, fmt.Errorf("provider does not implement ValidateWebhook method")
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(data),
		reflect.ValueOf(headers),
	})

	valid := results[0].Bool()

	var responseData map[string]string
	if !results[1].IsNil() {
		responseData = results[1].Interface().(map[string]string)
	}

	var err error
	if !results[2].IsNil() {
		err = results[2].Interface().(error)
	}

	return valid, responseData, err
}
