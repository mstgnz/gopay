import { Card } from 'src/app/models/response/card.model';

export interface CardManagementState {
  latestEulaId?: number,
  selectedCard?: Card,
  cards?: Card[]
}