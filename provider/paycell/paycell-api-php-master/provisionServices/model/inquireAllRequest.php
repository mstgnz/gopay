<?php 
include 'baseInput.php';
class InquireAllRequest extends BaseInput {

    public $merchantCode;
    public $msisdn;
    public $originalReferenceNumber;
    public $referenceNumber;
    public $paymentMethodType;
    
    public function __construct($extraParameters, $requestHeader, $merchantCode, $msisdn, $originalReferenceNumber, $referenceNumber, $paymentMethodType){
        parent::__construct($extraParameters, $requestHeader);
        $this->merchantCode = $merchantCode;
        $this->msisdn = $msisdn;
        $this->originalReferenceNumber = $originalReferenceNumber;
        $this->referenceNumber = $referenceNumber;
        $this->paymentMethodType = $paymentMethodType;
    }
    
}?>