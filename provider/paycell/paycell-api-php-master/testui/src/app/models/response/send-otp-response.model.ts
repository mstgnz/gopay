import { BaseOutput } from './base-output.model';

export interface SendOtpResponse extends BaseOutput {
  expireDate: String,
  token: String,
  originalReferenceNumber: String,
}