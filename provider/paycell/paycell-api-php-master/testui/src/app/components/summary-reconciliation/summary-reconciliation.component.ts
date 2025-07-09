import { Component, OnInit, Input } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ProvisionServicesService } from 'src/app/services/wsclient/provision-services.service';
import { SummaryReconciliationState } from './summary-reconciliation-state';
import * as moment from 'moment';
import { SummaryReconciliationResponse } from 'src/app/models/response/summary-reconciliation-response.model';

@Component({
  selector: 'app-summary-reconciliation',
  templateUrl: './summary-reconciliation.component.html',
  styleUrls: ['./summary-reconciliation.component.css']
})
export class SummaryReconciliationComponent implements OnInit {

  @Input() webCallMethod: String;
  @Input() state: SummaryReconciliationState;
  form: FormGroup;

  constructor(
    private formBuilder: FormBuilder,
    private provisionServicesService: ProvisionServicesService
  ) {
    this.form = this.formBuilder.group({
      reconciliationDate: [null, Validators.required],
      totalPostAuthAmount: [0, Validators.required],
      totalPostAuthCount: [0, Validators.required],
      totalPostAuthReverseAmount: [0, Validators.required],
      totalPostAuthReverseCount: [0, Validators.required],
      totalPreAuthAmount: [0, Validators.required],
      totalPreAuthCount: [0, Validators.required],
      totalPreAuthReverseAmount: [0, Validators.required],
      totalPreAuthReverseCount: [0, Validators.required],
      totalRefundAmount: [0, Validators.required],
      totalRefundCount: [0, Validators.required],
      totalReverseAmount: [0, Validators.required],
      totalReverseCount: [0, Validators.required],
      totalSaleAmount: [0, Validators.required],
      totalSaleCount: [0, Validators.required],
    })
  }

  ngOnInit() {
  }

  async sendReconciliation() {
    if (this.form.valid) {
      let reconciliationDate: String = moment(this.form.value.reconciliationDate).toLocaleString();
      let resp: SummaryReconciliationResponse = await this.provisionServicesService.summaryReconciliation({
        reconciliationDate,
        totalPostAuthAmount: this.form.value.totalPostAuthAmount,
        totalPostAuthCount: this.form.value.totalPostAuthCount,
        totalPostAuthReverseAmount: this.form.value.totalPostAuthReverseAmount,
        totalPostAuthReverseCount: this.form.value.totalPostAuthReverseCount,
        totalPreAuthAmount: this.form.value.totalPreAuthAmount,
        totalPreAuthCount: this.form.value.totalPreAuthCount,
        totalPreAuthReverseAmount: this.form.value.totalPreAuthReverseAmount,
        totalPreAuthReverseCount: this.form.value.totalPreAuthReverseCount,
        totalRefundAmount: this.form.value.totalRefundAmount,
        totalRefundCount: this.form.value.totalRefundCount,
        totalReverseAmount: this.form.value.totalReverseAmount,
        totalReverseCount: this.form.value.totalReverseCount,
        totalSaleAmount: this.form.value.totalSaleAmount,
        totalSaleCount: this.form.value.totalSaleCount,
        webCallMethod: this.webCallMethod,
      })
      this.state = {
        ...resp
      }
    }
  }

}
