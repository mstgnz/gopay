import { CardManagementState } from './card-management/card-management-state';
import { PaymentState } from './payment/payment-state';
import { ProvisionHistoryState } from './provision-history/provision-history-state';
import { PaymentInquiryState } from './payment-inquiry/payment-inquiry-state';
import { SummaryReconciliationState } from './summary-reconciliation/summary-reconciliation-state';

export interface State {
  msisdn: String,
  latestEulaID: number,
  webCallMethod: String,
  cardManagementState: CardManagementState,
  paymentState: PaymentState,
  provisionHistoryState: ProvisionHistoryState,
  paymentInquiryState: PaymentInquiryState,
  summaryReconciliationState: SummaryReconciliationState,
}