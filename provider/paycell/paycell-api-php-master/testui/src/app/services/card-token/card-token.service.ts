import { Injectable } from '@angular/core';
import { Subject, Observable } from 'rxjs';
import { HttpClient } from '@angular/common/http';
import endpointConstants from 'src/app/constant/endpoint-constants';
import { GetCardTokenSecureRequest } from 'src/app/models/request/get-card-token-secure-request.model';
import { GetCardTokenSecureResponse } from 'src/app/models/response/get-card-token-secure-response.model';
import { createTestRequestHeader } from 'src/app/models/request/get-card-token-secure-request.model';
import { GenerateHashDataResponse } from 'src/app/models/response/generate-hashdata-response.model';
import { ProvisionServicesService } from '../wsclient/provision-services.service';

@Injectable({
  providedIn: 'root'
})
export class CardTokenService {

  constructor(
    private httpClient: HttpClient,
    private provisionServicesService: ProvisionServicesService
  ) { }

  async getCardTokenSecure(request: GetCardTokenSecureRequest): Promise<GetCardTokenSecureResponse> {
    request.header = createTestRequestHeader();
    request = await this.generateRequestHashData(request);
    let cardToken = await this.getCardTokenSecureFromService(request);
    if (await this.checkResponseHashData(cardToken)) {
      return cardToken;
    }
    return null;
  }
  async generateRequestHashData(request: GetCardTokenSecureRequest): Promise<GetCardTokenSecureRequest> {
    let requestHashData = await this.provisionServicesService.generateHashData({
      transactionId: request.header.transactionId,
      transactionDateTime: request.header.transactionDateTime,
      cardToken: null,
      responseCode: null,
      responseDateTime: null,
    });
    request.hashData = requestHashData.hashData;
    return request;
  }

  async getCardTokenSecureFromService(request: GetCardTokenSecureRequest): Promise<GetCardTokenSecureResponse> {
    let cardTokenResponse = await this.httpClient
      .post(endpointConstants.getCardTokenSecureEndpoint, request).toPromise<any>();
    return cardTokenResponse;
  }

  async checkResponseHashData(resp: GetCardTokenSecureResponse): Promise<Boolean> {
    let responseHash = await this.provisionServicesService.generateHashData({
      cardToken: resp.cardToken,
      responseDateTime: resp.header.responseDateTime,
      responseCode: resp.header.responseCode,
      transactionDateTime: null,
      transactionId: resp.header.transactionId,
    });
    return responseHash.hashData == resp.hashData;
  }

  handleError(onError?: (error: any) => void) {
    return (error: any) => {
      if (onError) {
        onError(error);
      }
    }
  }
}


