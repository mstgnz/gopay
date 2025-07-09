import { ResponseHeader } from './response-header.model';
import { ExtraParameter } from '../extra-parameter.model';

export interface BaseOutput {
  extraParameters?: ExtraParameter[],
  responseHeader?: ResponseHeader,
}
