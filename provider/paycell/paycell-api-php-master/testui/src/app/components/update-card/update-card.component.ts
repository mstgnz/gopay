import { Component, OnInit, Inject } from '@angular/core';
import { FormGroup, FormBuilder, Validators } from '@angular/forms';
import { MAT_DIALOG_DATA, MatDialogRef, MatDialog, MatDialogConfig } from '@angular/material';
import { Card } from 'src/app/models/response/card.model';
import { ProvisionServicesService } from 'src/app/services/wsclient/provision-services.service';
import { UpdateCardResponse } from 'src/app/models/response/update-card-response.model';
import { ThreeDSecureComponent, ThreeDSecureData } from '../three-d-secure/three-d-secure.component';
import { TermsOfServiceContentComponent, TermsOfServiceContentData } from '../terms-of-service-content/terms-of-service-content.component';

export interface UpdateCardData {
  card: Card,
  msisdn: String,
  latestEulaId: number,
  webCallMethod: String
}

@Component({
  selector: 'app-update-card',
  templateUrl: './update-card.component.html',
  styleUrls: ['./update-card.component.css']
})
export class UpdateCardComponent implements OnInit {

  updateCardForm: FormGroup;

  constructor(
    public dialogRef: MatDialogRef<UpdateCardComponent>,
    @Inject(MAT_DIALOG_DATA) public data: UpdateCardData,
    private dialog: MatDialog,
    private formBuilder: FormBuilder,
    private provisionServicesService: ProvisionServicesService,
  ) {
    this.updateCardForm = this.formBuilder.group({
      alias: [this.data.card.alias, Validators.required],
      isDefault: [this.data.card.isDefault, Validators.required],
      isThreeDValidated: [this.data.card.isThreeDValidated, Validators.required],
      isTermsOfServiceAgreed: [!this.isTermsOfServiceAgreementRequired(), Validators.requiredTrue]

    });
  }

  ngOnInit() {
  }

  isChangeThreeDEnabled() {
    return !this.data.card.isThreeDValidated;
  }

  async updateCard() {
    if (this.updateCardForm.valid) {
      let threeDSessionId: String = null;
      if (this.updateCardForm.value.isThreeDValidated) {
        threeDSessionId = await this.startThreeDSession();
      }
      let updateCardResponse: UpdateCardResponse = await
        this.provisionServicesService.updateCard({
          alias: this.updateCardForm.value.alias,
          cardId: this.data.card.cardId,
          eulaId: this.data.latestEulaId,
          isDefault: this.updateCardForm.value.isDefault,
          msisdn: this.data.msisdn,
          otp: null,
          otpValidationId: null,
          threeDSessionId,
          webCallMethod: this.data.webCallMethod,
        })
      if (updateCardResponse) {
        this.dialogRef.close();
      }
    }
  }

  startThreeDSession(): Promise<String> {
    let data: ThreeDSecureData = {
      msisdn: this.data.msisdn,
      amount: "1",
      installmentCount: "0",
      cardId: this.data.card.cardId,
      webCallMethod: this.data.webCallMethod,
    };
    let config: MatDialogConfig = {
      disableClose: true,
      data
    }
    const dialogRef: MatDialogRef<ThreeDSecureComponent> = this.dialog.open(ThreeDSecureComponent, config);

    return dialogRef.afterClosed().toPromise<String>();
  }

  isTermsOfServiceAgreementRequired() {
    return this.data.card.showEulaId;
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
