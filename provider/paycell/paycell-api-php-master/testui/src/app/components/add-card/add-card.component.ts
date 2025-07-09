import { Component, OnInit, Inject } from '@angular/core';
import { FormGroup, FormBuilder, Validators } from '@angular/forms';
import { CardTokenService } from 'src/app/services/card-token/card-token.service';
import { GetCardTokenSecureResponse } from 'src/app/models/response/get-card-token-secure-response.model';
import { ProvisionServicesService } from 'src/app/services/wsclient/provision-services.service';
import { MAT_DIALOG_DATA, MatDialogRef, MatDialogConfig, MatDialog } from '@angular/material';
import { padZeros } from 'src/app/utils/string-utils';
import { ThreeDSecureData, ThreeDSecureComponent } from '../three-d-secure/three-d-secure.component';
import { RegisterCardRequest } from 'src/app/models/request/register-card-request.model';
import { TermsOfServiceContentComponent, TermsOfServiceContentData } from '../terms-of-service-content/terms-of-service-content.component';

export interface AddCardData {
  latestEulaId: Number;
  msisdn: String;
  webCallMethod: String;
}
@Component({
  selector: 'app-add-card',
  templateUrl: './add-card.component.html',
  styleUrls: ['./add-card.component.css']
})
export class AddCardComponent implements OnInit {

  addCardForm: FormGroup;

  constructor(
    public dialog: MatDialog,
    public dialogRef: MatDialogRef<AddCardComponent>,
    @Inject(MAT_DIALOG_DATA) public data: AddCardData,
    private formBuilder: FormBuilder,
    private cardTokenService: CardTokenService,
    private provisionServicesService: ProvisionServicesService,
  ) {
    this.addCardForm = this.formBuilder.group({
      alias: [null, Validators.required],
      creditCardNo: [null, Validators.required],
      expireDateMonth: [null, Validators.required],
      expireDateYear: [null, Validators.required],
      cvcNo: [null, Validators.required],
      isDefault: [false, Validators.required],
      isThreeDValidated: [false, Validators.required],
      isTermsOfServiceAgreed: [false, Validators.requiredTrue],
    });
  }

  ngOnInit() {
  }

  async addCard() {
    if (this.addCardForm.valid) {
      let getCardTokenSecureResponse: GetCardTokenSecureResponse = await this.cardTokenService.getCardTokenSecure({
        creditCardNo: "" + this.addCardForm.value.creditCardNo,
        cvcNo: padZeros("" + this.addCardForm.value.cvcNo, 3),
        expireDateMonth: "" + this.addCardForm.value.expireDateMonth,
        expireDateYear: "" + this.addCardForm.value.expireDateYear,
        hashData: null,//filled within service
        header: null,//filled within service
      });
      let registerCardRequest: RegisterCardRequest = {
        alias: this.addCardForm.value.alias,
        cardToken: getCardTokenSecureResponse.cardToken,
        eulaId: this.data.latestEulaId,
        isDefault: this.addCardForm.value.isDefault,
        msisdn: this.data.msisdn,
        webCallMethod: this.data.webCallMethod,
      };
      if (this.addCardForm.value.isThreeDValidated) {
        registerCardRequest.threeDSessionId = await this.startThreeDSession(registerCardRequest.cardToken)
      }
      let regiterCardResponse = await this.provisionServicesService.registerCard(registerCardRequest);
      if (regiterCardResponse.cardId) {
        this.dialogRef.close();
      }
    }
  }

  startThreeDSession(cardToken: String): Promise<String> {
    let data: ThreeDSecureData = {
      msisdn: "" + this.data.msisdn,
      amount: "1",
      installmentCount: "0",
      cardToken,
      webCallMethod: this.data.webCallMethod,
    };
    let config: MatDialogConfig = {
      disableClose: true,
      data
    }
    const dialogRef: MatDialogRef<ThreeDSecureComponent> = this.dialog.open(ThreeDSecureComponent, config);

    return dialogRef.afterClosed().toPromise<String>();
  }

  displayTermsOfService() {
    let data: TermsOfServiceContentData = {
      webCallMethod: this.data.webCallMethod
    }
    const dialogRef = this.dialog.open(TermsOfServiceContentComponent, {
      width: '90%',
      data
    });
  }

  hide() {
    this.dialogRef.close();
  }

}
