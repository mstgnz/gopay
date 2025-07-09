import { InquireRequest } from './inquire-request.model';

export interface InquireAllRequest extends InquireRequest {
  paymentMethodType?: string,
}