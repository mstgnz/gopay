import { BaseOutput } from './base-output.model';
import { Provision } from './provision.model';

export interface InquireResponse extends BaseOutput {
  acquirerBankCode?: String,
  orderId?: String,
  provisionList?: Provision[],
  status?: String,
}