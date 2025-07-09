<?php 
function post($url, $data = false)
{    
    $options = array(
        'http' => array(
            'header'  => "Content-type: application/json",
            'method'  => 'POST',
            'content' => json_encode($data)
        )
    );
    $context  = stream_context_create($options);
    $result = file_get_contents($url, false, $context);
    if ($result === FALSE) { 
        var_dump($result);
    }
    
    return $result;
}

function soap($wsdl, $methodName, $params){
    
    $options = array(
        'uri'=>'http://schemas.xmlsoap.org/soap/envelope/',
        'style'=>'SOAP_RPC',
        'use'=>'SOAP_ENCODED',
        'soap_version'=>'SOAP_1_1',
        'cache_wsdl'=>'WSDL_CACHE_NONE',
        'connection_timeout'=>15,
        'trace'=>true,
        'encoding'=>'UTF-8',
        'exceptions'=>true,
    );
    $soap = new SoapClient($wsdl, $options);
    $data = $soap->__call($methodName, [$params]);
    return $data;
}

function debug_to_console($data) {
    file_put_contents('php://stderr', print_r($data, TRUE));
}
?>