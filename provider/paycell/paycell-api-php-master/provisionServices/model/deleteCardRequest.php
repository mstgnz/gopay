<?php 
include 'baseInput.php';

class DeleteCardRequest extends BaseInput {
    
    public $cardId;
    public $msisdn;
    
    public function __construct($extraParameters, $requestHeader, $cardId, $msisdn){
        parent::__construct($extraParameters, $requestHeader);
        $this->cardId = $cardId;
        $this->msisdn = $msisdn;
    }
}
?>