<?php
include 'baseInput.php';

class GetThreeDSessionRequest extends BaseInput {
    public $amount;
    public $cardId;
    public $cardToken;
    public $installmentCount;
    public $merchantCode;
    public $msisdn;
    public $referenceNumber;
    public $target;
    public $transactionType;
    
    public function __construct($extraParameters, $requestHeader, $amount, $cardId, $cardToken, $installmentCount,
        $merchantCode, $msisdn, $referenceNumber, $target, $transactionType) {
        parent::__construct($extraParameters, $requestHeader);
        $this->amount = $amount;
        $this->cardId = $cardId;
        $this->cardToken = $cardToken;
        $this->installmentCount = $installmentCount;
        $this->merchantCode = $merchantCode;
        $this->msisdn = $msisdn;
        $this->referenceNumber = $referenceNumber;
        $this->target = $target;
        $this->transactionType = $transactionType;
    }
}

?>