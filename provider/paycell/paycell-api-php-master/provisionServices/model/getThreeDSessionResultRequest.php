<?php 
include 'baseInput.php';

class GetThreeDSessionResultRequest extends BaseInput {
    public $merchantCode;
    public $msisdn;
    public $referenceNumber;
    public $threeDSessionId;
    
    public function __construct($extraParameters, $requestHeader, $merchantCode, $msisdn, $referenceNumber, $threeDSessionId){
        parent::__construct($extraParameters, $requestHeader);
        
        $this->merchantCode = $merchantCode;
        $this->msisdn = $msisdn;
        $this->referenceNumber = $referenceNumber;
        $this->threeDSessionId = $threeDSessionId;
    }
}
?>