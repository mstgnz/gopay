import { BrowserModule } from '@angular/platform-browser';
import { NgModule } from '@angular/core';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';

import { AppComponent } from './components/app.component';
import { CardManagementComponent } from './components/card-management/card-management.component';
import { BrowserAnimationsModule } from '@angular/platform-browser/animations';
import { MatInputModule, MatTableModule, MatPaginatorModule, MatSortModule, MatButtonModule, MatFormFieldModule, MatGridListModule, MatTabsModule, MatDialogModule, MatCheckboxModule, MatRadioModule, MatSelectModule, MatDatepickerModule }
  from '@angular/material';
import { MatMomentDateModule } from '@angular/material-moment-adapter';
import { HttpClientModule } from '@angular/common/http';
import { AddCardComponent } from './components/add-card/add-card.component';
import { UpdateCardComponent } from './components/update-card/update-card.component';
import { ThreeDSecureComponent } from './components/three-d-secure/three-d-secure.component';
import { TermsOfServiceContentComponent } from './components/terms-of-service-content/terms-of-service-content.component';
import { WebCallMethodPickerComponent } from './components/web-call-method-picker/web-call-method-picker.component';
import { CardListComponent } from './components/card-list/card-list.component';
import { PaymentComponent } from './components/payment/payment.component';
import { QueryComponent } from './components/query/query.component';
import { CardPaymentComponent } from './components/card-payment/card-payment.component';
import { PaymentInfoComponent } from './components/payment-info/payment-info.component';
import { ProvisionHistoryComponent } from './components/provision-history/provision-history.component';
import { PaymentInquiryComponent } from './components/payment-inquiry/payment-inquiry.component';
import { SummaryReconciliationComponent } from './components/summary-reconciliation/summary-reconciliation.component';
import { OtpValidationComponent } from './components/otp-validation/otp-validation.component';
import { CustomCardPaymentComponent } from './components/custom-card-payment/custom-card-payment.component';

@NgModule({
  declarations: [
    AppComponent,
    CardManagementComponent,
    AddCardComponent,
    UpdateCardComponent,
    ThreeDSecureComponent,
    TermsOfServiceContentComponent,
    WebCallMethodPickerComponent,
    CardListComponent,
    PaymentComponent,
    QueryComponent,
    CardPaymentComponent,
    PaymentInfoComponent,
    ProvisionHistoryComponent,
    PaymentInquiryComponent,
    SummaryReconciliationComponent,
    OtpValidationComponent,
    CustomCardPaymentComponent
  ],
  imports: [
    BrowserModule,
    FormsModule,
    ReactiveFormsModule,
    BrowserAnimationsModule,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatTableModule,
    MatPaginatorModule,
    MatSortModule,
    MatGridListModule,
    HttpClientModule,
    MatTabsModule,
    MatDialogModule,
    MatCheckboxModule,
    MatRadioModule,
    MatSelectModule,
    MatDatepickerModule,
    MatMomentDateModule,
  ],
  entryComponents: [
    AddCardComponent,
    UpdateCardComponent,
    ThreeDSecureComponent,
    TermsOfServiceContentComponent
  ],
  providers: [],
  bootstrap: [AppComponent]
})
export class AppModule { }
