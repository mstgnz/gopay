import { Injectable } from '@angular/core';
import { GetCardsResponse } from '../../models/response/get-cards-response.model';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { GetCardsRequest } from '../../models/request/get-cards-request.model';
import Endpoints from "../../constant/endpoint-constants"
import { Subject, Observable } from 'rxjs';
import { GenerateHashDataRequest } from 'src/app/models/request/generate-hashdata-request.model';
import { GenerateHashDataResponse } from 'src/app/models/response/generate-hashdata-response.model';
import { RegisterCardResponse } from 'src/app/models/response/register-card-response.model';
import { RegisterCardRequest } from 'src/app/models/request/register-card-request.model';
import { DeleteCardRequest } from 'src/app/models/request/delete-card-request.model';
import { DeleteCardResponse } from 'src/app/models/response/delete-card-response.model';
import { UpdateCardRequest } from 'src/app/models/request/update-card-request.model';
import { UpdateCardResponse } from 'src/app/models/response/update-card-response.model';
import { GetThreeDSessionRequest } from 'src/app/models/request/get-three-d-session-request.model';
import { GetThreeDSessionResponse } from 'src/app/models/response/get-three-d-session-response.model';
import { GetThreeDSessionResultRequest } from 'src/app/models/request/get-three-d-session-result-request.model';
import { GetThreeDSessionResultResponse } from 'src/app/models/response/get-three-d-session-result-response.model';
import { GetTermsOfServiceContentRequest } from 'src/app/models/request/get-terms-of-service-content-request.model';
import { GetTermsOfServiceContentResponse } from 'src/app/models/response/get-terms-of-service-content-response.model';
import { GetPaymentMethodsRequest } from 'src/app/models/request/get-payment-methods-request.model';
import { GetPaymentMethodsResponse } from 'src/app/models/response/get-payment-methods-response.model';
import { ProvisionRequest } from 'src/app/models/request/provision-request.model';
import { ProvisionResponse } from 'src/app/models/response/provision-response.model';
import { GetProvisionHistoryRequest } from 'src/app/models/request/get-provision-history-request.model';
import { GetProvisionHistoryResponse } from 'src/app/models/response/get-provision-history-response.model';
import { InquireRequest } from 'src/app/models/request/inquire-request.model';
import { InquireResponse } from 'src/app/models/response/inquire-response.model';
import { ReverseRequest } from 'src/app/models/request/reverse-request.model';
import { ReverseResponse } from 'src/app/models/response/reverse-response.model';
import { RefundRequest } from 'src/app/models/request/refund-request.model';
import { RefundResponse } from 'src/app/models/response/refund-response.model';
import { ProvisionForMarketPlaceRequest } from 'src/app/models/request/provision-for-market-place-request.model';
import { ProvisionForMarketPlaceResponse } from 'src/app/models/response/provision-for-market-place-response.model';
import { SummaryReconciliationRequest } from 'src/app/models/request/summary-reconciliation-request.model';
import { SummaryReconciliationResponse } from 'src/app/models/response/summary-reconciliation-response.model';
import { SendOtpRequest } from 'src/app/models/request/send-otp-request.model';
import { SendOtpResponse } from 'src/app/models/response/send-otp-response.model';
import { ValidateOtpRequest } from 'src/app/models/request/validate-otp-request.model';
import { ValidateOtpResponse } from 'src/app/models/response/validate-otp-response.model';
import { OpenMobilePaymentResponse } from 'src/app/models/response/open-mobile-payment-response.model';
import { OpenMobilePaymentRequest } from 'src/app/models/request/open-mobile-payment-request.model';
import { ProvisionAllRequest } from 'src/app/models/request/provision-all-request.model';
import { ProvisionAllResponse } from 'src/app/models/response/provision-all-response.model';
import { InquireAllRequest } from 'src/app/models/request/inquire-all-request.model';
import { InquireAllResponse } from 'src/app/models/response/inquire-all-response.model';
import { RefundAllRequest } from 'src/app/models/request/refund-all-request.model';
import { RefundAllResponse } from 'src/app/models/response/refund-all-response.model';

@Injectable({
  providedIn: 'root'
})

export class ProvisionServicesService {

  constructor(
    private httpClient: HttpClient
  ) { }

  getCards(request: GetCardsRequest, onResponse: (response: GetCardsResponse) => void, onError?: (error: any) => void): Observable<GetCardsResponse> {
    let subject = new Subject<GetCardsResponse>();
    this.httpClient
      .post(Endpoints.getCardsEndpoint, request)
      .subscribe(resp => {
        onResponse(<GetCardsResponse>resp);
      }, error => {
        onError(error);
      })
    return subject.asObservable();
  }

  async generateHashData(request: GenerateHashDataRequest): Promise<GenerateHashDataResponse> {
    let endpoint = Endpoints.generateHashDataEndpoint;
    let resp = await this.httpClient.post(endpoint, request).toPromise<any>();
    return resp;
  }

  async registerCard(request: RegisterCardRequest): Promise<RegisterCardResponse> {
    let endpoint: string = Endpoints.registerCardEndpoint;
    let registerCardResponse = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return registerCardResponse;
  }

  async deleteCard(request: DeleteCardRequest): Promise<DeleteCardResponse> {
    let endpoint: string = Endpoints.deleteCardEndpoint;
    let deleteCardResponse = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return deleteCardResponse;
  }

  async updateCard(request: UpdateCardRequest): Promise<UpdateCardResponse> {
    let endpoint: string = Endpoints.updateCardEndpoint;
    let updateCardResponse = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return updateCardResponse;
  }

  async getThreeDSession(request: GetThreeDSessionRequest): Promise<GetThreeDSessionResponse> {
    let endpoint: string = Endpoints.getThreeDSessionEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async getThreeDSessionResult(request: GetThreeDSessionResultRequest): Promise<GetThreeDSessionResultResponse> {
    let endpoint: string = Endpoints.getThreeDSessionResultEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async getTermsOfServiceContent(request: GetTermsOfServiceContentRequest): Promise<GetTermsOfServiceContentResponse> {
    let endpoint: string = Endpoints.getTermsOfServiceEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async getPaymentMethods(request: GetPaymentMethodsRequest): Promise<GetPaymentMethodsResponse> {
    let endpoint: string = Endpoints.getPaymentMethodsEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async provision(request: ProvisionRequest): Promise<ProvisionResponse> {
    let endpoint: string = Endpoints.provisionEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async provisionAll(request: ProvisionAllRequest): Promise<ProvisionAllResponse> {
    let endpoint: string = Endpoints.provisionAllEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async provisionForMarketPlace(request: ProvisionForMarketPlaceRequest): Promise<ProvisionForMarketPlaceResponse> {
    let endpoint: string = Endpoints.provisionForMarketPlaceEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async getProvisionHistory(request: GetProvisionHistoryRequest): Promise<GetProvisionHistoryResponse> {
    let endpoint: string = Endpoints.getProvisionHistoryEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async inquire(request: InquireRequest): Promise<InquireResponse> {
    let endpoint: string = Endpoints.inquireEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async inquireAll(request: InquireAllRequest): Promise<InquireAllResponse> {
    let endpoint: string = Endpoints.inquireAllEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async reverse(request: ReverseRequest): Promise<ReverseResponse> {
    let endpoint: string = Endpoints.reverseEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async refund(request: RefundRequest): Promise<RefundResponse> {
    let endpoint: string = Endpoints.refundEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async refundAll(request: RefundAllRequest): Promise<RefundAllResponse> {
    let endpoint: string = Endpoints.refundAllEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async summaryReconciliation(request: SummaryReconciliationRequest): Promise<SummaryReconciliationResponse> {
    let endpoint: string = Endpoints.summaryReconciliationEndpoint;
    let response = await this.httpClient
      .post(endpoint, request).toPromise<any>();
    return response;
  }

  async sendOtp(request: SendOtpRequest): Promise<SendOtpResponse> {
    let endpoint: string = Endpoints.sendOtpEndpoint;
    return await this.httpClient.post(endpoint, request).toPromise<any>();
  }

  async validateOtp(request: ValidateOtpRequest): Promise<ValidateOtpResponse> {
    let endpoint: string = Endpoints.validateOtpEndpoint;
    return await this.httpClient.post(endpoint, request).toPromise<any>();
  }

  async openMobilePayment(request: OpenMobilePaymentRequest): Promise<OpenMobilePaymentResponse> {
    let endpoint: string = Endpoints.openMobilePaymentEndpoint;
    return await this.httpClient.post(endpoint, request).toPromise<any>();
  }
}
