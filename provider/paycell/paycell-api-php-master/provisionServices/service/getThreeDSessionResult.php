<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/getThreeDSessionResultRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');

if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $target = "MERCHANT";
    $transactionType = "PREAUTH";
    $request = new GetThreeDSessionResultRequest(null, $header, TestConstants::$merchantCode, substr($data["msisdn"],2,10), 
        isset($data["referenceNumber"]) ? $data["referenceNumber"] : null, $data["threeDSessionId"]);
    
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        echo post(RestEndpointConstants::$getThreeDSessionResult, $request);
    } else {
        echo json_encode(soap(SoapEndpointConstants::$provisionServices, "getThreeDSessionResult", $request));
    }
}

?>