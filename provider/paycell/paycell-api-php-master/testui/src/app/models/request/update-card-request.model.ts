import { BaseInput } from './base-input.model';

export interface UpdateCardRequest extends BaseInput {
  alias: String;
  cardId: String;
  eulaId: number;
  isDefault: Boolean;
  msisdn: String;
  otp: String;
  otpValidationId: String;
  threeDSessionId: String;
}