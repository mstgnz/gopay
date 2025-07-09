import { Component, Input, Output, EventEmitter, ViewChild, OnChanges } from '@angular/core';
import { Card } from 'src/app/models/response/card.model';
import { MatPaginator, MatTableDataSource } from '@angular/material';

@Component({
  selector: 'app-card-list',
  templateUrl: './card-list.component.html',
  styleUrls: ['./card-list.component.css']
})
export class CardListComponent implements OnChanges {

  cardDataSource: MatTableDataSource<Card>;
  @ViewChild(MatPaginator, { static: false }) paginator: MatPaginator;
  displayedCardColumns: String[] = [
    "cardId",
    "maskedCardNo",
    "alias",
    "cardBrand",
    "isDefault",
    "isExpired",
    "showEulaId",
    "activationDate",
    "isThreeDValidated",
    "isOTPValidated",
  ]

  @Input('cards') cards: Card[];
  @Input() selectedCard: Card;

  @Output() onCardSelected: EventEmitter<Card> = new EventEmitter();

  constructor() {
  }

  ngOnChanges() {
    this.cardDataSource = new MatTableDataSource(this.cards);
    this.cardDataSource.paginator = this.paginator;
  }

  isCardSelected(card: Card) {
    return this.selectedCard && this.selectedCard.cardId == card.cardId;
  }

  selectCard(card: Card) {
    this.onCardSelected.emit(card);
  }

}
