export interface MobilePayment {
  eulaId: string,
  eulaUrl: string,
  isDcbOpen: boolean,
  isEulaExpired: boolean,
  limit: string,
  maxLimit: string,
  remainingLimit: string,
  signedEulaId: string,
  statementDate: string,
}