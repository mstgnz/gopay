<?php 
include 'baseInput.php';

class GetCardsRequest extends BaseInput {
    public $msisdn;
    
    public function __construct($extraParameters, $requestHeader, $msisdn) {
        parent::__construct($extraParameters, $requestHeader);
        $this->msisdn = $msisdn;
    }
}
?>