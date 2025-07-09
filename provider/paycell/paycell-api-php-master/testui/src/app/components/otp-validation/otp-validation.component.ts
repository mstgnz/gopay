import { Component, OnInit, Input, Output, EventEmitter } from '@angular/core';
import { OtpValidationState, ValidationStatus } from './otp-validation-state';
import { ProvisionServicesService } from 'src/app/services/wsclient/provision-services.service';
import { SendOtpResponse } from 'src/app/models/response/send-otp-response.model';
import { ValidateOtpResponse } from 'src/app/models/response/validate-otp-response.model';
import { MobilePayment } from 'src/app/models/response/mobile-payment.model';
import { MatDialog } from '@angular/material';

@Component({
  selector: 'app-otp-validation',
  templateUrl: './otp-validation.component.html',
  styleUrls: ['./otp-validation.component.css']
})
export class OtpValidationComponent implements OnInit {

  @Input() webCallMethod: String;
  @Input() msisdn: String;
  @Input() amount: String;
  @Input() mobilePayment: MobilePayment;
  @Input() state: OtpValidationState;

  @Output() onOtpValidated: EventEmitter<String> = new EventEmitter();

  constructor(
    private dialog: MatDialog,
    private provisionServicesService: ProvisionServicesService
  ) { }

  ngOnInit() {
  }

  isEulaAgreementRequired() {
    return !this.mobilePayment.isDcbOpen ||
      !this.mobilePayment.signedEulaId ||
      this.mobilePayment.signedEulaId != this.mobilePayment.eulaId;
  }

  isSendOtpDisabled() {
    if (this.isEulaAgreementRequired() && !this.state.eulaIsSigned) return true;
    return false;
  }

  showEula() {
    window.open(this.mobilePayment.eulaUrl, "_blank");
  }

  async sendOtp() {
    if (this.state.eulaIsSigned) {
      await this.provisionServicesService.openMobilePayment({
        eulaId: this.mobilePayment.eulaId,
        msisdn: this.msisdn,
        webCallMethod: this.webCallMethod,
      })
    }
    let otpResp: SendOtpResponse = await this.provisionServicesService.sendOtp({
      amount: "" + this.amount,
      msisdn: this.msisdn,
      webCallMethod: this.webCallMethod
    });
    this.state = {
      status: ValidationStatus.OTP_SENT,
      token: otpResp.token,
      originalReferenceNumber: otpResp.originalReferenceNumber,
    }
  }

  getValidationStatusLabel(): String {
    switch (this.state.status) {
      case ValidationStatus.OTP_SENT:
        return "Otp gönderildi"
      case ValidationStatus.VALIDATED:
        return "Doğrulandı"
      case ValidationStatus.NOT_INIT:
        return "Otp doğrulama başlatılmadı."
      default:
        return "";
    }
  }

  isValidationEnabled() {
    return this.state.status == ValidationStatus.OTP_SENT;
  }

  async validateOtp() {
    let validateResp: ValidateOtpResponse = await this.provisionServicesService.validateOtp({
      amount: "" + this.amount,
      msisdn: this.msisdn,
      otp: this.state.otp,
      token: this.state.token,
      webCallMethod: this.webCallMethod,
      referenceNumber: this.state.originalReferenceNumber,
    });
    if (validateResp.responseHeader.responseCode == "0") {
      this.state.status = ValidationStatus.VALIDATED;
      this.onOtpValidated.emit(this.state.originalReferenceNumber);
    }
  }

}
