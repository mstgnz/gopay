import { Component, OnInit, Input } from '@angular/core';
import { ProvisionServicesService } from 'src/app/services/wsclient/provision-services.service';
import { PaymentState, PaymentMethod } from './payment-state';
import { ProvisionRequest, PaymentType } from 'src/app/models/request/provision-request.model';
import { ThreeDSecureData, ThreeDSecureComponent } from '../three-d-secure/three-d-secure.component';
import { MatDialogConfig, MatDialogRef, MatDialog } from '@angular/material';
import { ProvisionResponse } from 'src/app/models/response/provision-response.model';
import { ProvisionForMarketPlaceResponse } from 'src/app/models/response/provision-for-market-place-response.model';
import { ProvisionForMarketPlaceRequest } from 'src/app/models/request/provision-for-market-place-request.model';
import { PaymentMethodType } from 'src/app/models/request/payment-method-type';
import { CardTokenService } from 'src/app/services/card-token/card-token.service';
import { GetCardTokenSecureResponse } from 'src/app/models/response/get-card-token-secure-response.model';
import { padZeros } from 'src/app/utils/string-utils';

@Component({
  selector: 'app-payment',
  templateUrl: './payment.component.html',
  styleUrls: ['./payment.component.css']
})
export class PaymentComponent implements OnInit {

  @Input() state: PaymentState;
  @Input() msisdn: string;
  @Input() webCallMethod: string;

  constructor(
    private provisionServicesService: ProvisionServicesService,
    private cardTokenService: CardTokenService,
    private dialog: MatDialog,
  ) { }

  ngOnInit() {
    this.refreshPaymentMethods();
  }

  async refreshPaymentMethods() {
    let paymentMethodsResponse = await this.provisionServicesService.getPaymentMethods({
      msisdn: this.msisdn,
      webCallMethod: this.webCallMethod,
    })
    this.state.cardPaymentState.cards = paymentMethodsResponse.cardList;
    this.state.mobilePayment = paymentMethodsResponse.mobilePayment;
    this.state.eulaId = paymentMethodsResponse.eulaId;
  }

  isCardsAvailable() {
    return this.state.cardPaymentState.cards;
  }

  isMobilePaymentAvailable() {
    return this.state.mobilePayment;
  }

  isCustomCardPaymentSelected() {
    if (this.state.selectedPaymentMethod) {
      let paymentMethod: string = PaymentMethod[PaymentMethod.CUSTOM_CARD];
      return this.state.selectedPaymentMethod.toString() == paymentMethod;
    }
    return false;
  }

  isCardPaymentSelected() {
    if (this.state.selectedPaymentMethod) {
      let paymentMethod: string = PaymentMethod[PaymentMethod.CARD];
      return this.state.selectedPaymentMethod.toString() == paymentMethod;
    }
    return false;
  }

  isMobilePaymentSelected() {
    if (this.state.selectedPaymentMethod) {
      let paymentMethod: string = PaymentMethod[PaymentMethod.MOBILE];
      return this.state.selectedPaymentMethod.toString() == paymentMethod;
    }
    return false;
  }

  provisionSale() {
    this.provision("SALE");
  }

  provisionPreauth() {
    this.provision("PREAUTH");
  }

  provisionPostauth() {
    this.provision("POSTAUTH");
  }

  provision(paymentType: String) {
    if (this.state.paymentInfoState.subMerchants || this.state.paymentInfoState.customerEmail) {
      this.sendProvisionForMarketPlace(paymentType);
    } else {
      this.sendProvision(paymentType);
    }

  }

  async sendProvision(paymentType: String) {
    let provisionRequest: ProvisionRequest = {
      amount: "" + this.state.paymentInfoState.amount,
      currency: "" + this.state.paymentInfoState.currency,
      msisdn: this.msisdn,
      installmentCount: "" + this.state.paymentInfoState.installmentCount,
      paymentType,
      pointAmount: this.state.paymentInfoState.pointAmount,
      webCallMethod: this.webCallMethod,
    };
    if (paymentType == "POSTAUTH") {
      provisionRequest.originalReferenceNumber = this.state.paymentInfoState.originalReferenceNumber;
    }
    let provisionReponse: ProvisionResponse = undefined;
    if (this.state.selectedPaymentMethod.toString() == PaymentMethod[PaymentMethod.CARD]) {
      provisionReponse = await this.provisionRegisteredCard(provisionRequest);
    } else if (this.state.selectedPaymentMethod.toString() == PaymentMethod[PaymentMethod.MOBILE]) {
      provisionReponse = await this.provisionMobile(provisionRequest);
    } else if (this.state.selectedPaymentMethod.toString() == PaymentMethod[PaymentMethod.CUSTOM_CARD]) {
      provisionReponse = await this.provisionCustomCard(provisionRequest);
    }
    if (provisionReponse && paymentType != "POSTAUTH") {
      this.state.paymentInfoState.originalReferenceNumber = provisionReponse.originalReferenceNumber;
    } else if (provisionReponse && paymentType == "POSTAUTH") {
      this.state.paymentInfoState.postauthReferenceNumber = provisionReponse.originalReferenceNumber;
    }
  }

  async sendProvisionForMarketPlace(paymentType: String) {
    let provisionRequest: ProvisionForMarketPlaceRequest = {
      amount: "" + this.state.paymentInfoState.amount,
      currency: "" + this.state.paymentInfoState.currency,
      msisdn: this.msisdn,
      installmentCount: "" + this.state.paymentInfoState.installmentCount,
      paymentType,
      pointAmount: this.state.paymentInfoState.pointAmount,
      webCallMethod: this.webCallMethod,
      customerEmail: this.state.paymentInfoState.customerEmail,
      subMerchants: this.state.paymentInfoState.subMerchants,
    };
    if (paymentType == "POSTAUTH") {
      provisionRequest.originalReferenceNumber = this.state.paymentInfoState.originalReferenceNumber;
    }
    let provisionReponse: ProvisionForMarketPlaceResponse = undefined;
    if (this.state.selectedPaymentMethod.toString() == PaymentMethod[PaymentMethod.CARD]) {
      provisionReponse = await this.provisionForMarketPlaceRegisteredCard(provisionRequest);
    } else if (this.state.selectedPaymentMethod.toString() == PaymentMethod[PaymentMethod.CUSTOM_CARD]) {
      provisionReponse = await this.provisionForMarketPlaceCustomCard(provisionRequest);
    }
    if (provisionReponse && paymentType != "POSTAUTH") {
      this.state.paymentInfoState.originalReferenceNumber = provisionReponse.originalReferenceNumber;
    } else if (provisionReponse && paymentType == "POSTAUTH") {
      this.state.paymentInfoState.postauthReferenceNumber = provisionReponse.originalReferenceNumber;
    }
  }

  async provisionCustomCard(provisionRequest: ProvisionRequest) {
    if (!this.state.customCardPaymentState.isEulaSigned) return;
    let getCardTokenSecureResponse: GetCardTokenSecureResponse = await this.cardTokenService.getCardTokenSecure({
      creditCardNo: "" + this.state.customCardPaymentState.creditCardNo,
      cvcNo: padZeros(this.state.customCardPaymentState.cvcNo, 3),
      expireDateMonth: padZeros(this.state.customCardPaymentState.expireDateMonth, 2),
      expireDateYear: padZeros(this.state.customCardPaymentState.expireDateYear, 2),
      hashData: null,//filled within service
      header: null,//filled within service
    })
    provisionRequest.cardToken = getCardTokenSecureResponse.cardToken;
    if (this.state.customCardPaymentState.isThreeDValidated) {
      provisionRequest.threeDSessionId = await this.startThreeDSession(undefined, provisionRequest.cardToken)
    }
    return this.provisionServicesService.provision(provisionRequest);
  }

  async provisionRegisteredCard(provisionRequest: ProvisionRequest) {
    provisionRequest.cardId = this.state.cardPaymentState.selectedCard.cardId
    let cvcNo = padZeros(this.state.cardPaymentState.cvcNo, 3);
    if (cvcNo) {
      let getCardTokenSecureResponse: GetCardTokenSecureResponse = await this.cardTokenService.getCardTokenSecure({
        cvcNo,
      })
      provisionRequest.cardToken = getCardTokenSecureResponse.cardToken;
    }
    if (this.state.cardPaymentState.selectedCard.isThreeDValidated) {
      provisionRequest.threeDSessionId = await this.startThreeDSession(provisionRequest.cardId, undefined)
    }
    return this.provisionServicesService.provision(provisionRequest);
  }

  async provisionMobile(provisionRequest: ProvisionRequest) {
    return this.provisionServicesService.provisionAll({
      ...provisionRequest,
      paymentMethodType: PaymentMethodType[PaymentMethodType.MOBILE_PAYMENT],
      referenceNumber: this.state.paymentInfoState.originalReferenceNumber,
    });
  }

  async provisionForMarketPlaceRegisteredCard(provisionRequest: ProvisionForMarketPlaceRequest) {
    provisionRequest.cardId = this.state.cardPaymentState.selectedCard.cardId
    let cvcNo = padZeros(this.state.cardPaymentState.cvcNo, 3);
    if (cvcNo) {
      let getCardTokenSecureResponse: GetCardTokenSecureResponse = await this.cardTokenService.getCardTokenSecure({
        cvcNo,
      })
      provisionRequest.cardToken = getCardTokenSecureResponse.cardToken;
    }
    if (this.state.cardPaymentState.selectedCard.isThreeDValidated) {
      provisionRequest.threeDSessionId = await this.startThreeDSession(provisionRequest.cardId, undefined)
    }
    return this.provisionServicesService.provisionForMarketPlace(provisionRequest);
  }

  async provisionForMarketPlaceCustomCard(provisionRequest: ProvisionForMarketPlaceRequest) {
    if (!this.state.customCardPaymentState.isEulaSigned) return;
    let getCardTokenSecureResponse: GetCardTokenSecureResponse = await this.cardTokenService.getCardTokenSecure({
      creditCardNo: "" + this.state.customCardPaymentState.creditCardNo,
      cvcNo: padZeros(this.state.customCardPaymentState.cvcNo, 3),
      expireDateMonth: padZeros(this.state.customCardPaymentState.expireDateMonth, 2),
      expireDateYear: padZeros(this.state.customCardPaymentState.expireDateYear, 2),
      hashData: null,//filled within service
      header: null,//filled within service
    })
    provisionRequest.cardToken = getCardTokenSecureResponse.cardToken;
    if (this.state.customCardPaymentState.isThreeDValidated) {
      provisionRequest.threeDSessionId = await this.startThreeDSession(undefined, provisionRequest.cardToken)
    }
    return this.provisionServicesService.provisionForMarketPlace(provisionRequest);
  }

  startThreeDSession(cardId: String, cardToken: String): Promise<String> {
    let data: ThreeDSecureData = {
      msisdn: this.msisdn,
      amount: this.state.paymentInfoState.amount,
      installmentCount: "" + this.state.paymentInfoState.installmentCount,
      cardId,
      cardToken,
      webCallMethod: this.webCallMethod,
    };
    let config: MatDialogConfig = {
      disableClose: true,
      data
    }
    const dialogRef: MatDialogRef<ThreeDSecureComponent> = this.dialog.open(ThreeDSecureComponent, config);

    return dialogRef.afterClosed().toPromise<String>();
  }

  onOtpValidated(originalReferenceNumber: String) {
    this.state.paymentInfoState.originalReferenceNumber = originalReferenceNumber;
  }
}
