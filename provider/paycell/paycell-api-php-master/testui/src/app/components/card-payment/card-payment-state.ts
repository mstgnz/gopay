import { Card } from 'src/app/models/response/card.model';

export interface CardPaymentState {
  cards?: Card[];
  selectedCard?: Card;
  threeDSessionId?: string;
  cvcNo?: String;
}