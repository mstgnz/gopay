import { BaseInput } from './base-input.model';

export interface GetProvisionHistoryRequest extends BaseInput {

  partitionNo: Number,
  reconciliationDate: String

}