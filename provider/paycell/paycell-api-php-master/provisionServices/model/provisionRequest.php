<?php
include 'baseInput.php';
class ProvisionRequest extends BaseInput {
    
    public $acquirerBankCode;
    public $amount;
    public $cardId;
    public $cardToken;
    public $currency;
    public $installmentCount;
    public $merchantCode;
    public $msisdn;
    public $originalReferenceNumber;
    public $paymentType;
    public $pin;
    public $pointAmount;
    public $referenceNumber;
    public $threeDSessionId;
    
    public function __construct($extraParameters, $requestHeader, $acquirerBankCode, $amount, $cardId, 
        $cardToken, $currency, $installmentCount, $merchantCode, $msisdn, $originalReferenceNumber, $paymentType,
        $pin, $pointAmount, $referenceNumber, $threeDSessionId){
        parent::__construct($extraParameters, $requestHeader);
        $this->acquirerBankCode = $acquirerBankCode;
        $this->amount = $amount;
        $this->cardId = $cardId;
        $this->cardToken = $cardToken;
        $this->currency = $currency;
        $this->installmentCount = $installmentCount;
        $this->merchantCode = $merchantCode;
        $this->msisdn = $msisdn;
        $this->originalReferenceNumber = $originalReferenceNumber;
        $this->paymentType = $paymentType;
        $this->pin = $pin;
        $this->pointAmount = $pointAmount;
        $this->referenceNumber = $referenceNumber;
        $this->threeDSessionId = $threeDSessionId;
    }
}
?>