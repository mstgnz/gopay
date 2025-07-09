import { BaseInput } from './base-input.model';

export interface ReverseRequest extends BaseInput {
  msisdn: String,
  originalReferenceNumber: String,
}