<?php 
include 'baseInput.php';
class RefundRequest extends BaseInput {
    public $merchantCode;
    public $msisdn;
    public $originalReferenceNumber;
    public $referenceNumber;
    public $amount;
    public $pointAmount;
    
    public function __construct($extraParameters, $requestHeader, $merchantCode, $msisdn, $originalReferenceNumber, $referenceNumber, $amount, $pointAmount){
        parent::__construct($extraParameters, $requestHeader);
        $this->merchantCode = $merchantCode;
        $this->msisdn = $msisdn;
        $this->originalReferenceNumber = $originalReferenceNumber;
        $this->referenceNumber = $referenceNumber;
        $this->amount = $amount;
        $this->pointAmount = $pointAmount;
    }
}
?>