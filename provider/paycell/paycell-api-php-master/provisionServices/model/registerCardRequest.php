<?php
include 'baseInput.php';

class RegisterCardRequest extends BaseInput
{

    public $alias;
    public $cardToken;
    public $eulaId;
    public $isDefault;
    public $msisdn;
    public $otp;
    public $otpValidationId;
    public $threeDSessionId;

    public function __construct($extraParameters, $requestHeader, $alias, $cardToken, $eulaId, $isDefault, $msisdn, $otp, $otpValidationId, $threeDSessionId)
    {
        parent::__construct($extraParameters, $requestHeader);
        $this->alias = $alias;
        $this->cardToken = $cardToken;
        $this->eulaId = $eulaId;
        $this->isDefault = $isDefault;
        $this->msisdn = $msisdn;
        $this->otp = $otp;
        $this->otpValidationId = $otpValidationId;       
        $this->threeDSessionId = $threeDSessionId;
    }
}
?>