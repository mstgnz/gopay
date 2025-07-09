import { BaseOutput } from './base-output.model';

export interface GetThreeDSessionResultResponse extends BaseOutput {
  currentStep?: String;
  mdErrorMessage?: String;
  mdStatus?: String;
  threeDOperationResult?: ThreeDOperationResult;
}

export interface ThreeDOperationResult {
  threeDResult?: String;
  threeDResultDescription?: String;
}