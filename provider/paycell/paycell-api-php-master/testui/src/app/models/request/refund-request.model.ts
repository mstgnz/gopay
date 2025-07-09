import { BaseInput } from './base-input.model';

export interface RefundRequest extends BaseInput {
  msisdn: String,
  originalReferenceNumber: String,
  amount: String,
  pointAmount: String,
}