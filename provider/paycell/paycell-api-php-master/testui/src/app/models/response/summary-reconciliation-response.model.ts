import { BaseOutput } from './base-output.model';

export interface SummaryReconciliationResponse extends BaseOutput {
  reconciliationDate: String,
  reconciliationResult: String,
  totalPostAuthAmount: String,
  totalPostAuthCount: Number,
  totalPostAuthReverseAmount: String,
  totalPostAuthReverseCount: Number,
  totalPreAuthAmount: String,
  totalPreAuthCount: Number,
  totalPreAuthReverseAmount: String,
  totalPreAuthReverseCount: Number,
  totalRefundAmount: String,
  totalRefundCount: Number,
  totalReverseAmount: String,
  totalReverseCount: Number,
  totalSaleAmount: String,
  totalSaleCount: Number,
}