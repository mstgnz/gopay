import { Component, OnInit, Input } from '@angular/core';
import { PaymentInquiryState } from './payment-inquiry-state';
import { ProvisionServicesService } from 'src/app/services/wsclient/provision-services.service';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { InquireResponse } from 'src/app/models/response/inquire-response.model';
import { ReverseResponse } from 'src/app/models/response/reverse-response.model';
import { RefundResponse } from 'src/app/models/response/refund-response.model';
import { PaymentMethodType } from 'src/app/models/request/payment-method-type';
import { InquireAllResponse } from 'src/app/models/response/inquire-all-response.model';
import { RefundAllResponse } from 'src/app/models/response/refund-all-response.model';

@Component({
  selector: 'app-payment-inquiry',
  templateUrl: './payment-inquiry.component.html',
  styleUrls: ['./payment-inquiry.component.css']
})
export class PaymentInquiryComponent implements OnInit {

  @Input() state: PaymentInquiryState;
  @Input() msisdn: String;
  @Input() webCallMethod: String;

  displayedColumns: String[] = [
    "amount",
    "approvalCode",
    "dateTime",
    "provisionType",
    "reconciliationDate",
    "responseCode",
    "responseDescription",
    "transactionId",
  ]

  inquiryForm: FormGroup;

  constructor(
    private provisionServicesService: ProvisionServicesService,
    private formBuilder: FormBuilder
  ) {
    this.inquiryForm = this.formBuilder.group({
      originalReferenceNumber: [null, Validators.required]
    })
  }

  ngOnInit() {
  }

  async inquireCard() {
    if (this.inquiryForm.valid) {
      let resp: InquireResponse = await this.provisionServicesService.inquire({
        msisdn: this.msisdn,
        originalReferenceNumber: this.inquiryForm.value.originalReferenceNumber,
        webCallMethod: this.webCallMethod,
      })
      this.state = {
        originalReferenceNumber: this.inquiryForm.value.originalReferenceNumber,
        acquirerBankCode: resp.acquirerBankCode,
        orderId: resp.orderId,
        status: resp.status,
        provisionList: resp.provisionList,
        paymentReverseState: {},
        paymentRefundState: {},
      }
    }
  }

  async inquireMobile() {
    if (this.inquiryForm.valid) {
      let resp: InquireAllResponse = await this.provisionServicesService.inquireAll({
        msisdn: this.msisdn,
        originalReferenceNumber: this.inquiryForm.value.originalReferenceNumber,
        webCallMethod: this.webCallMethod,
        paymentMethodType: PaymentMethodType[PaymentMethodType.MOBILE_PAYMENT],
      })
      this.state = {
        originalReferenceNumber: this.inquiryForm.value.originalReferenceNumber,
        acquirerBankCode: resp.acquirerBankCode,
        orderId: resp.orderId,
        status: resp.status,
        provisionList: resp.provisionList,
        paymentReverseState: {},
        paymentRefundState: {},
      }
    }
  }

  async reverse() {
    if (this.state.originalReferenceNumber) {
      let resp: ReverseResponse = await this.provisionServicesService.reverse({
        msisdn: this.msisdn,
        webCallMethod: this.webCallMethod,
        originalReferenceNumber: this.state.originalReferenceNumber,
      })
      this.state.paymentReverseState = {
        approvalCode: resp.approvalCode,
        reconciliationDate: resp.reconciliationDate,
        retryStatusCode: resp.retryStatusCode,
        retryStatusDescription: resp.retryStatusDescription,
        reverseReferenceNumber: resp.originalReferenceNumber,
      }
    }
  }

  async refundCard() {
    if (this.state.originalReferenceNumber) {
      let resp: RefundResponse = await this.provisionServicesService.refund({
        msisdn: this.msisdn,
        webCallMethod: this.webCallMethod,
        originalReferenceNumber: this.state.originalReferenceNumber,
        amount: this.state.paymentRefundState.amount,
        pointAmount: this.state.paymentRefundState.pointAmount,
      })
      this.state.paymentRefundState = {
        approvalCode: resp.approvalCode,
        reconciliationDate: resp.reconciliationDate,
        retryStatusCode: resp.retryStatusCode,
        retryStatusDescription: resp.retryStatusDescription,
        reverseReferenceNumber: resp.originalReferenceNumber,
      }
    }
  }

  async refundMobile() {
    if (this.state.originalReferenceNumber) {
      let resp: RefundAllResponse = await this.provisionServicesService.refundAll({
        msisdn: this.msisdn,
        webCallMethod: this.webCallMethod,
        originalReferenceNumber: this.state.originalReferenceNumber,
        amount: this.state.paymentRefundState.amount,
        pointAmount: this.state.paymentRefundState.pointAmount,
      })
      this.state.paymentRefundState = {
        approvalCode: resp.approvalCode,
        reconciliationDate: resp.reconciliationDate,
        retryStatusCode: resp.retryStatusCode,
        retryStatusDescription: resp.retryStatusDescription,
        reverseReferenceNumber: resp.originalReferenceNumber,
      }
    }
  }

}
