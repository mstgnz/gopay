import { BaseOutput } from './base-output.model';

export interface ValidateOtpResponse extends BaseOutput {
  remainingRetryCount?: Number;
}