<?php 
include 'baseInput.php';
class SendOtpRequest extends BaseInput {
    
    public $amount;
    public $msisdn;
    public $referenceNumber;
    
    public function __construct($extraParameters, $requestHeader, $amount, $msisdn, $referenceNumber){
        parent::__construct($extraParameters, $requestHeader);
        $this->amount = $amount;
        $this->msisdn = $msisdn;
        $this->referenceNumber = $referenceNumber;
    }
}
?>