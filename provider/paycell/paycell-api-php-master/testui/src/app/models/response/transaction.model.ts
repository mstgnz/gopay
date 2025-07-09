import { TransactionParam } from './transaction-param.model';

export interface Transaction {
  acquirerBankCode: String,
  amount: Number,
  approvalCode: String,
  issuerBankCode: String,
  netAmount: Number,
  orderId: String,
  referenceNumber: String,
  transactionDateTime: String,
  transactionId: String,
  transactionParams: TransactionParam[]
  transactionType: String,

}