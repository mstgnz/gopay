<?php
include 'baseInput.php';

class UpdateCardRequest extends BaseInput
{
    public $alias;
    public $cardId;
    public $eulaId;
    public $isDefault;
    public $msisdn;
    public $otp;
    public $otpValidationId;
    public $threeDSessionId;

    public function __construct($extraParameters, $requestHeader, $alias, $cardId, $eulaId, $isDefault, $msisdn, $otp, $otpValidationId, $threeDSessionId)
    {
        parent::__construct($extraParameters, $requestHeader);
        $this->alias = $alias;
        $this->cardId = $cardId;
        $this->eulaId = $eulaId;
        $this->isDefault = $isDefault;
        $this->msisdn = $msisdn;
        $this->otp = $otp;
        $this->otpValidationId = $otpValidationId;       
        $this->threeDSessionId = $threeDSessionId;
    }
}
?>