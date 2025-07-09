<?php 
include '../testConstants.php';

class RequestHeader {
    public $applicationName;
    public $applicationPwd;
    public $clientIPAddress;
    public $transactionDateTime;
    public $transactionId;
    
    public function __construct($pApplicationName, $pApplicationPwd, $pClientIPAddress, $pTransactionDateTime, $pTransactionId)
    {
        $this->applicationName = $pApplicationName;
        $this->applicationPwd = $pApplicationPwd;
        $this->clientIPAddress = $pClientIPAddress;
        $this->transactionDateTime = $pTransactionDateTime;
        $this->transactionId = $pTransactionId;
    }
    
    public static function createTestHeader(){
        $transactionDateTime = substr(date_format(new DateTime(), "YmdHisu"), 0, 17);
        $transactionId = TestConstants::$prefix.$transactionDateTime;
        $clientIPAddress = $_SERVER['REMOTE_ADDR'];
        return new RequestHeader(TestConstants::$applicationName, TestConstants::$applicationPwd, 
            $clientIPAddress, $transactionDateTime, $transactionId);
    }
}
?>