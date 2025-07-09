import { BaseInput } from './base-input.model';

export interface SummaryReconciliationRequest extends BaseInput {
  reconciliationDate: String,
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