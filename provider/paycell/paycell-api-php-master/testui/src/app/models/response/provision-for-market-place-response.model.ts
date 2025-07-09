import { BaseOutput } from './base-output.model';

export interface ProvisionForMarketPlaceResponse extends BaseOutput {
  originalReferenceNumber: string,
  acquirerBankCode?: string,
  approvalCode?: string,
  issuerBankCode?: string,
  iyzPaymentId?: string,
  iyzPaymentTransactionId?: string,
  orderId?: string,
  /** YYYYMMDD */
  reconciliationDate?: string,
}