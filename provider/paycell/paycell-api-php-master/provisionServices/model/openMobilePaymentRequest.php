<?php 
include 'baseInput.php';

class OpenMobilePaymentRequest extends BaseInput {
    public $msisdn;
    public $eulaId;
    
    public function __construct($extraParameters, $requestHeader, $msisdn, $eulaId) {
        parent::__construct($extraParameters, $requestHeader);
        $this->msisdn = $msisdn;
        $this->eulaId = $eulaId;
    }
}
?>