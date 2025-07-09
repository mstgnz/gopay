import { BaseInput } from './base-input.model';
export enum PaymentType {
  SALE, PREAUTH, POSTAUTH
}
export interface ProvisionRequest extends BaseInput {
  msisdn: string,
  amount: string,
  currency: string,
  installmentCount: string,
  paymentType: String,
  pointAmount?: string,
  cardId?: String,
  cardToken?: String,
  threeDSessionId?: String,
  originalReferenceNumber?: String,
  referenceNumber?: String,
}