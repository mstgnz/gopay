import { BaseInput } from './base-input.model';

export interface DeleteCardRequest extends BaseInput{
  msisdn: String;
  cardId: String;
}