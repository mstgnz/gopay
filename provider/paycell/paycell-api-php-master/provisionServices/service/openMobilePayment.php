<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/openMobilePaymentRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');


if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $request = new OpenMobilePaymentRequest(null, $header, substr($data["msisdn"],2,10), $data["eulaId"]);
    
    debug_to_console($request);
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        echo post(RestEndpointConstants::$openMobilePayment, $request);
    } else {
        echo json_encode(soap(SoapEndpointConstants::$provisionServices, "openMobilePayment", $request));
    }
}


?>