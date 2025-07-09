import { BaseInput } from './base-input.model';
import { ProvisionRequest } from './provision-request.model';
export enum PaymentType {
  SALE, PREAUTH, POSTAUTH
}

export interface ProvisionAllRequest extends ProvisionRequest {
  paymentMethodType: string,
}