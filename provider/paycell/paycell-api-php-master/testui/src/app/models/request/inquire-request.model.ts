import { BaseInput } from './base-input.model';

export interface InquireRequest extends BaseInput {
  msisdn: String,
  originalReferenceNumber: String,
}