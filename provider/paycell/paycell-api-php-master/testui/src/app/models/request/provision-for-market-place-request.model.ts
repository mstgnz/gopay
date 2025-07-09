import { BaseInput } from './base-input.model';
import { SubMerchant } from './sub-merchant.model';
export enum PaymentType {
  SALE, PREAUTH, POSTAUTH
}
export interface ProvisionForMarketPlaceRequest extends BaseInput {
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
  customerEmail: String,
  subMerchants: SubMerchant[],
}