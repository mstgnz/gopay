import { SubMerchant } from './sub-merchant';

export enum Currency {
  TRY, EUR, USD
}

export interface PaymentInfoState {
  amount?: String,
  currency?: Currency,
  installmentCount?: Number,
  pointAmount?: string,
  originalReferenceNumber?: String,
  postauthReferenceNumber?: String,
  customerEmail?: String,
  subMerchants?: SubMerchant[],
}