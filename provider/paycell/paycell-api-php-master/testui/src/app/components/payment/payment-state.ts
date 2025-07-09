import { Card } from 'src/app/models/response/card.model';
import { MobilePayment } from 'src/app/models/response/mobile-payment.model';
import { CardPaymentState } from '../card-payment/card-payment-state';
import { PaymentInfoState } from '../payment-info/payment-info-state';
import { OtpValidationState } from '../otp-validation/otp-validation-state';
import { CustomCardPaymentState } from '../custom-card-payment/custom-card-payment-state';

export enum PaymentMethod {
  MOBILE, CARD, CUSTOM_CARD
}


export interface PaymentState {
  selectedPaymentMethod?: PaymentMethod,
  mobilePayment?: MobilePayment,
  eulaId?: string,
  cardPaymentState: CardPaymentState,
  paymentInfoState: PaymentInfoState,
  otpValidationState: OtpValidationState,
  customCardPaymentState: CustomCardPaymentState,
}