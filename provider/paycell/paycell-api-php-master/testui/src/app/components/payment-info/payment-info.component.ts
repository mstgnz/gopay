import { Component, OnInit, Input } from '@angular/core';
import { PaymentInfoState } from './payment-info-state';
import { FormGroup, FormBuilder, Validators } from '@angular/forms';

@Component({
  selector: 'app-payment-info',
  templateUrl: './payment-info.component.html',
  styleUrls: ['./payment-info.component.css']
})
export class PaymentInfoComponent implements OnInit {

  @Input() state: PaymentInfoState;

  subMerchantForm: FormGroup;

  displayedColumns: String[] = [
    "subMerchantKey",
    "subMerchantPrice",
  ];

  constructor(
    private formBuilder: FormBuilder
  ) {
    this.initSubMerchantForm();
  }

  ngOnInit() {

  }

  initSubMerchantForm() {
    this.subMerchantForm = this.formBuilder.group({
      subMerchantKey: [null, Validators.required],
      subMerchantPrice: [null, Validators.required],
    });
  }

  addSubMerchant() {
    if (this.subMerchantForm.valid) {
      if (!this.state.subMerchants) {
        this.state.subMerchants = [];
      }
      this.state.subMerchants.push({
        subMerchantKey: this.subMerchantForm.value.subMerchantKey,
        subMerchantPrice: this.subMerchantForm.value.subMerchantPrice,
      })
      this.initSubMerchantForm();
    }
  }

}
