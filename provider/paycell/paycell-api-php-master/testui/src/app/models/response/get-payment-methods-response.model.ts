import { Card } from './card.model';
import { BaseOutput } from './base-output.model';
import { MobilePayment } from './mobile-payment.model';

export interface GetPaymentMethodsResponse extends BaseOutput {
  eulaId: string,
  cardList: Card[],
  mobilePayment: MobilePayment,
}