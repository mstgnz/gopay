export enum ValidationStatus {
  NOT_INIT, OTP_SENT, VALIDATED
}

export interface OtpValidationState {
  eulaIsSigned?: Boolean,
  status?: ValidationStatus,
  token?: String,
  otp?: String,
  originalReferenceNumber?: String,
}