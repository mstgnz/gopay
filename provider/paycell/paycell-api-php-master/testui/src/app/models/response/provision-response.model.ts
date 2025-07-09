import { BaseOutput } from './base-output.model';

export interface ProvisionResponse extends BaseOutput {
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