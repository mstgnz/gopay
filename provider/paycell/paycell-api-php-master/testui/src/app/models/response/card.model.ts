export interface Card {
  cardId: String;
  maskedCardNo: String;
  alias: String;
  cardBrand: String;
  isDefault: Boolean;
  isExpired: Boolean;
  showEulaId: Boolean;
  activationDate: Date;
  isThreeDValidated: Boolean;
  isOTPValidated: Boolean;
}