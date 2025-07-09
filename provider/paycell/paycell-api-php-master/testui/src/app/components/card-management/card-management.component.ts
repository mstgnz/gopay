import { Component, Input, OnInit } from '@angular/core';
import { Card } from '../../models/response/card.model';
import { ProvisionServicesService } from '../../services/wsclient/provision-services.service';
import { GetCardsResponse } from '../../models/response/get-cards-response.model';
import { MatDialog } from '@angular/material';
import { AddCardComponent, AddCardData } from '../add-card/add-card.component';
import { UpdateCardData, UpdateCardComponent } from '../update-card/update-card.component'
import { CardManagementState } from './card-management-state';

@Component({
  selector: 'app-card-management',
  templateUrl: './card-management.component.html',
  styleUrls: ['./card-management.component.css']
})
export class CardManagementComponent implements OnInit {

  @Input() msisdn: string;
  @Input() webCallMethod: string;
  @Input() state: CardManagementState;

  constructor(
    private provisionServicesService: ProvisionServicesService,
    private dialog: MatDialog,
  ) {
  }

  ngOnInit() {
    this.queryCardsFromService(this.msisdn);
  }

  queryCardsFromService(msisdn: String) {
    this.provisionServicesService.getCards({
      msisdn,
      webCallMethod: this.webCallMethod,
    },
      (response: GetCardsResponse) => {
        this.state.latestEulaId = parseInt(response.eulaId);
        this.state.cards = response.cardList;
        this.unselectCard()
      })
  }

  selectCard(card: Card) {
    this.state.selectedCard = card;
  }

  unselectCard() {
    this.state.selectedCard = null;
  }

  addCard() {
    let data: AddCardData = {
      latestEulaId: this.state.latestEulaId,
      msisdn: this.msisdn,
      webCallMethod: this.webCallMethod,
    }
    const dialogRef = this.dialog.open(AddCardComponent, {
      width: '90%',
      data,
    });

    dialogRef.afterClosed().subscribe(result => {
      this.queryCardsFromService(this.msisdn);
    });
  }

  editCard() {
    let card: Card = this.state.selectedCard;
    let data: UpdateCardData = {
      latestEulaId: this.state.latestEulaId,
      msisdn: this.msisdn,
      card,
      webCallMethod: this.webCallMethod,
    }
    const dialogRef = this.dialog.open(UpdateCardComponent, {
      data,
    });

    dialogRef.afterClosed().subscribe(result => {
      this.queryCardsFromService(this.msisdn);
    });
  }

  async removeCard() {
    let card: Card = this.state.selectedCard;
    let removeCardResponse = await this.provisionServicesService.deleteCard({
      msisdn: this.msisdn,
      cardId: card.cardId,
      webCallMethod: this.webCallMethod,
    })
    this.queryCardsFromService(this.msisdn);
  }

  isCardSelected(card: Card) {
    return this.state.selectedCard &&
      this.state.selectedCard.cardId == card.cardId;
  }

  isAddCardEnabled() {
    return this.msisdn;
  }

  isEditCardEnabled() {
    return this.state.selectedCard;
  }

}
