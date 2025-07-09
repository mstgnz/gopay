<?php 
include 'baseInput.php';
class ReverseRequest extends BaseInput {
    public $merchantCode;
    public $msisdn;
    public $originalReferenceNumber;
    public $referenceNumber;
    
    public function __construct($extraParameters, $requestHeader, $merchantCode, $msisdn, $originalReferenceNumber, $referenceNumber){
        parent::__construct($extraParameters, $requestHeader);
        $this->merchantCode = $merchantCode;
        $this->msisdn = $msisdn;
        $this->originalReferenceNumber = $originalReferenceNumber;
        $this->referenceNumber = $referenceNumber;
    }
}
?>