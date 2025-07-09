import { BaseOutput } from './base-output.model';
import { Transaction } from './transaction.model';

export interface GetProvisionHistoryResponse extends BaseOutput {

  nextPartitionNo: Number,
  transactionList: Transaction[],

}