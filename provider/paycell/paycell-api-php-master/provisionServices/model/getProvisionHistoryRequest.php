<?php 
include 'baseInput.php';
class GetProvisionHistoryRequest extends BaseInput {
    
    public $merchantCode;
    public $partitionNo;
    public $reconciliationDate;
    
    function __construct($extraParameters, $requestHeader, $merchantCode, $partitionNo, $reconciliationDate) {
        parent::__construct($extraParameters, $requestHeader);
        $this->merchantCode = $merchantCode;
        $this->partitionNo = $partitionNo;
        $this->reconciliationDate = $reconciliationDate;
    }
    
}
?>