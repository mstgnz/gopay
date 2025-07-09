import { BaseOutput } from './base-output.model';

export interface RefundResponse extends BaseOutput {
  approvalCode: String,
  reconciliationDate: String,
  retryStatusCode: String,
  retryStatusDescription: String,
  originalReferenceNumber: String,
}