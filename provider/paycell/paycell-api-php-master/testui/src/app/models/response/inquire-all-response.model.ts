import { PaymentMethodType } from '../request/payment-method-type';
import { InquireResponse } from './inquire-response.model';

export interface InquireAllResponse extends InquireResponse {
  paymentMethodType: PaymentMethodType;
}