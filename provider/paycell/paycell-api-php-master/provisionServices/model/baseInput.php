<?php 
class BaseInput {
    public $extraParameters;
    public $requestHeader;
    
    public function __construct($extraParameters, $requestHeader)
    {
        $this->extraParameters = $extraParameters;
        $this->requestHeader = $requestHeader;
    }
}
?>