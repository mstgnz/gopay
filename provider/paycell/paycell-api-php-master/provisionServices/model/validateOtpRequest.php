<?php 
include 'baseInput.php';
class ValidateOtpRequest extends BaseInput {
    
    public $amount;
    public $msisdn;
    public $referenceNumber;
    public $otp;
    public $token;
    
    public function __construct($extraParameters, $requestHeader, $amount, $msisdn, $referenceNumber, $otp, $token){
        parent::__construct($extraParameters, $requestHeader);
        $this->amount = $amount;
        $this->msisdn = $msisdn;
        $this->referenceNumber = $referenceNumber;
        $this->otp = $otp;
        $this->token = $token;
    }
}
?>