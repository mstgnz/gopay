import { Component, OnInit, Input } from '@angular/core';
import { ProvisionHistoryState } from './provision-history-state';
import { FormGroup, FormBuilder, Validators } from '@angular/forms';
import { ProvisionServicesService } from 'src/app/services/wsclient/provision-services.service';
import { GetProvisionHistoryResponse } from 'src/app/models/response/get-provision-history-response.model';
import { TransactionParam } from 'src/app/models/response/transaction-param.model';
import * as moment from 'moment';

@Component({
  selector: 'app-provision-history',
  templateUrl: './provision-history.component.html',
  styleUrls: ['./provision-history.component.css']
})
export class ProvisionHistoryComponent implements OnInit {

  @Input() webCallMethod: String;
  @Input() state: ProvisionHistoryState;

  form: FormGroup;

  displayedColumns: String[] = [
    "transactionId",
    "referenceNumber",
    "transactionDateTime",
    "amount",
    "netAmount",
    "transactionType",
    "issuerBankCode",
    "acquirerBankCode",
    "orderId",
    "approvalCode",
    "transactionParams",
  ]

  constructor(
    private formBuilder: FormBuilder,
    private provisionServicesService: ProvisionServicesService,
  ) {
    this.form = this.formBuilder.group({
      reconciliationDate: [null, Validators.required]
    });
  }

  ngOnInit() {
  }

  async query() {
    if (this.form.valid) {
      let reconciliationDate: String = moment(this.form.value.reconciliationDate).toLocaleString();
      let history: GetProvisionHistoryResponse = await this.provisionServicesService.getProvisionHistory({
        partitionNo: 1,
        reconciliationDate,
        webCallMethod: this.webCallMethod,
      });
      this.state = {
        reconciliationDate,
        currentPartitionNo: 1,
        nextPartitionNo: history.nextPartitionNo,
        transactions: history.transactionList
      }
    }
  }

  async next() {
    let nextPartitionNo = this.state.nextPartitionNo;
    let history: GetProvisionHistoryResponse = await this.provisionServicesService.getProvisionHistory({
      partitionNo: nextPartitionNo,
      reconciliationDate: this.state.reconciliationDate,
      webCallMethod: this.webCallMethod,
    });
    this.state = {
      reconciliationDate: this.state.reconciliationDate,
      currentPartitionNo: nextPartitionNo,
      nextPartitionNo: history.nextPartitionNo,
      transactions: history.transactionList
    }
  }

  async back() {
    let previousPartitionNo: Number = this.state.currentPartitionNo.valueOf() - 1;
    let history: GetProvisionHistoryResponse = await this.provisionServicesService.getProvisionHistory({
      partitionNo: previousPartitionNo,
      reconciliationDate: this.state.reconciliationDate,
      webCallMethod: this.webCallMethod,
    });
    this.state = {
      reconciliationDate: this.state.reconciliationDate,
      currentPartitionNo: previousPartitionNo,
      nextPartitionNo: history.nextPartitionNo,
      transactions: history.transactionList
    }
  }

  formatTransactionParams(transactionParams: TransactionParam[]) {
    let result: String = "";
    transactionParams.forEach((p: TransactionParam) => {
      result = result.concat(p.key + " : " + p.value + "; ");
    })
    return result;
  }

  isBackDisabled() {
    return this.state.currentPartitionNo == undefined || this.state.currentPartitionNo == 1;
  }

  isNextDisabled() {
    return !this.state.nextPartitionNo;
  }

}
