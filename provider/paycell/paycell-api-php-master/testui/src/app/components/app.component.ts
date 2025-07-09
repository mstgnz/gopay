import { Component } from '@angular/core';
import { State } from './State';
import { ValidationStatus } from './otp-validation/otp-validation-state';

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.css']
})
export class AppComponent {
  title = 'testui';
  state: State = {
    msisdn: null,
    latestEulaID: null,
    webCallMethod: "REST",
    cardManagementState: {
    },
    paymentState: {
      customCardPaymentState: {

      },
      cardPaymentState: {

      },
      paymentInfoState: {

      },
      otpValidationState: {
        status: ValidationStatus.NOT_INIT
      }
    },
    provisionHistoryState: {

    },
    paymentInquiryState: {
      paymentReverseState: {},
      paymentRefundState: {},
    },
    summaryReconciliationState: {

    }
  }
}
