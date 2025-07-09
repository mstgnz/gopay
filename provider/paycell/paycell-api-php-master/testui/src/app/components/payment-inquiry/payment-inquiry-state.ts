import { Provision } from 'src/app/models/response/provision.model';
import { PaymentReverseState } from './payment-reverse-state';
import { PaymentRefundState } from './payment-refund-state';

export interface PaymentInquiryState {
  originalReferenceNumber?: String,
  acquirerBankCode?: String,
  orderId?: String,
  provisionList?: Provision[],
  status?: String,
  paymentReverseState: PaymentReverseState,
  paymentRefundState: PaymentRefundState,
}