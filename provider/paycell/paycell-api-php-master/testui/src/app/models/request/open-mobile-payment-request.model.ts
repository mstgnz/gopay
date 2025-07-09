import { BaseInput } from './base-input.model';

export interface OpenMobilePaymentRequest extends BaseInput {
  msisdn: String,
  eulaId: String,
}