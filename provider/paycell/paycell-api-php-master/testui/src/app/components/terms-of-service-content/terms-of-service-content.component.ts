import { Component, OnInit, Inject } from '@angular/core';
import { MatDialogRef, MAT_DIALOG_DATA } from '@angular/material';
import { ProvisionServicesService } from 'src/app/services/wsclient/provision-services.service';
import { GetTermsOfServiceContentResponse } from 'src/app/models/response/get-terms-of-service-content-response.model';

export interface TermsOfServiceContentData {
  webCallMethod: String,
}

@Component({
  selector: 'app-terms-of-service-content',
  templateUrl: './terms-of-service-content.component.html',
  styleUrls: ['./terms-of-service-content.component.css']
})
export class TermsOfServiceContentComponent implements OnInit {

  termsOfServiceHtmlContentEN: string;
  termsOfServiceHtmlContentTR: string;

  hide() {
    this.dialogRef.close();
  }

  constructor(
    @Inject(MAT_DIALOG_DATA) public data: TermsOfServiceContentData,
    private dialogRef: MatDialogRef<TermsOfServiceContentComponent>,
    private provisionServicesService: ProvisionServicesService,
  ) {
    this.loadContent();
  }

  async loadContent() {
    let response: GetTermsOfServiceContentResponse = await this.provisionServicesService.getTermsOfServiceContent({
      webCallMethod: this.data.webCallMethod
    });
    this.termsOfServiceHtmlContentEN = response.termsOfServiceHtmlContentEN;
    this.termsOfServiceHtmlContentTR = response.termsOfServiceHtmlContentTR;
  }
  ngOnInit() {
  }

}
