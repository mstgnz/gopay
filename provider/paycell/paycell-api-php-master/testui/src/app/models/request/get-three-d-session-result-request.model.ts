import { BaseInput } from './base-input.model';

export interface GetThreeDSessionResultRequest extends BaseInput {
  msisdn: String;
  referenceNumber: String;
  threeDSessionId: String;
}