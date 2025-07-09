import { BaseInput } from './base-input.model';

export interface RegisterCardRequest extends BaseInput {
  alias: String;
  cardToken: String;
  eulaId: Number;
  isDefault: Boolean;
  msisdn: String;
  otp?: String;
  otpValidationId?: String;
  threeDSessionId?: String;
}