import { Component, OnInit, Inject, ViewChild, ElementRef } from '@angular/core';
import { DomSanitizer } from '@angular/platform-browser';
import { ProvisionServicesService } from 'src/app/services/wsclient/provision-services.service';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material';
import { GetThreeDSessionResponse } from 'src/app/models/response/get-three-d-session-response.model';
import { GetThreeDSessionResultResponse } from 'src/app/models/response/get-three-d-session-result-response.model';

export interface ThreeDSecureData {
  msisdn: String,
  cardId?: String,
  cardToken?: String,
  referenceNumber?: String,
  amount?: String;
  installmentCount?: String,
  webCallMethod: String,
}

@Component({
  selector: 'app-three-d-secure',
  templateUrl: './three-d-secure.component.html',
  styleUrls: ['./three-d-secure.component.css']
})
export class ThreeDSecureComponent implements OnInit {

  @ViewChild('threeDSecureForm', { static: true }) threeDSecureFormElement: ElementRef;
  @ViewChild('threeDSessionId', { static: true }) threeDSessionIdElement: ElementRef;

  constructor(
    public dialogRef: MatDialogRef<ThreeDSecureComponent>,
    @Inject(MAT_DIALOG_DATA) public data: ThreeDSecureData,
    public sanitizer: DomSanitizer,
    public provisionServicesService: ProvisionServicesService,
  ) {
  }

  async ngOnInit() {
    let threeDSessionId: String = await this.getThreeDSession();
    this.redirectThreeDSecurePage(threeDSessionId);
    this.waitThreeDSecureResult(threeDSessionId);

  }

  redirectThreeDSecurePage(threeDSessionId: String) {
    this.threeDSessionIdElement.nativeElement.value = threeDSessionId;
    this.threeDSecureFormElement.nativeElement.submit();
  }

  async waitThreeDSecureResult(threeDSessionId: String) {
    let startTime: number = new Date().getTime();
    let interval = setInterval(async () => {
      let threeDSuccess: Boolean = await this.checkTreeDSessionResult(threeDSessionId);
      if (threeDSuccess) {
        clearInterval(interval);
        this.dialogRef.close(threeDSessionId);
      } else if (new Date().getTime() - startTime > 2 * 60 * 1000) {
        clearInterval(interval);
        this.dialogRef.close("-1");
      }
    }, 3000);
  }

  async checkTreeDSessionResult(threeDSessionId: String) {
    let response: GetThreeDSessionResultResponse = await this.provisionServicesService.getThreeDSessionResult({
      msisdn: this.data.msisdn,
      referenceNumber: this.data.referenceNumber,
      threeDSessionId,
      webCallMethod: this.data.webCallMethod,
    });
    if (response.threeDOperationResult && response.threeDOperationResult.threeDResult == "0") {
      return true;
    }
    return false;
  }

  async getThreeDSession(): Promise<String> {
    let threeDSessionResponse: GetThreeDSessionResponse = await this.provisionServicesService.getThreeDSession(this.data);
    return threeDSessionResponse.threeDSessionId;
  }

}
