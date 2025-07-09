import { Component, OnInit, Input } from '@angular/core';
import { CustomCardPaymentState } from './custom-card-payment-state';
import { TermsOfServiceContentData, TermsOfServiceContentComponent } from '../terms-of-service-content/terms-of-service-content.component';
import { MatDialog } from '@angular/material';

@Component({
  selector: 'app-custom-card-payment',
  templateUrl: './custom-card-payment.component.html',
  styleUrls: ['./custom-card-payment.component.css']
})
export class CustomCardPaymentComponent implements OnInit {

  @Input() state: CustomCardPaymentState;
  @Input() webCallMethod: String;

  constructor(
    private dialog: MatDialog,
  ) { }

  ngOnInit() {
  }

  displayTermsOfService() {
    let data: TermsOfServiceContentData = {
      webCallMethod: this.webCallMethod
    }
    const dialogRef = this.dialog.open(TermsOfServiceContentComponent, {
      width: '90%',
      data
    });
  }

}
