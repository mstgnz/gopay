import { Transaction } from 'src/app/models/response/transaction.model';

export interface ProvisionHistoryState {
  reconciliationDate?: String,
  currentPartitionNo?: Number,
  nextPartitionNo?: Number,
  transactions?: Transaction[],
}