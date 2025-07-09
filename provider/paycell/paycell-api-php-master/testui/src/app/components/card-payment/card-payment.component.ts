import { Component, OnInit, Input } from '@angular/core';
import { Card } from 'src/app/models/response/card.model';
import { CardPaymentState } from './card-payment-state';

@Component({
  selector: 'app-card-payment',
  templateUrl: './card-payment.component.html',
  styleUrls: ['./card-payment.component.css']
})
export class CardPaymentComponent implements OnInit {

  @Input() msisdn: string;
  @Input() webCallMethod: string;
  @Input() state: CardPaymentState;

  constructor() { }

  ngOnInit() {
  }

  selectCard(card: Card) {
    this.state.selectedCard = card;
  }

}
