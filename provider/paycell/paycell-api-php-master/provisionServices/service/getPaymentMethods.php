<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/getPaymentMethodsRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');


if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $request = new GetPaymentMethodsRequest(null, $header, substr($data["msisdn"],2,10));
    
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        echo post(RestEndpointConstants::$getPaymentMethods, $request);
    } else {
        echo json_encode(soap(SoapEndpointConstants::$provisionServices, "getPaymentMethods", $request));
    }
}


?>