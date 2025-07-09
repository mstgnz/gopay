import { BaseOutput } from './base-output.model';

export interface GetTermsOfServiceContentResponse extends BaseOutput {
  eulaId?: number,
  termsOfServiceHtmlContentEN?: string,
  termsOfServiceHtmlContentTR?: string
}