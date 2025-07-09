import { Card } from './card.model';
import { BaseOutput } from './base-output.model';

export interface GetCardsResponse extends BaseOutput {
  eulaId: string;
  cardList: Card[];
}