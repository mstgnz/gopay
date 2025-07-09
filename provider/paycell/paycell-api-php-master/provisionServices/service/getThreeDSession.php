<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/getThreeDSessionRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');

if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $target = "MERCHANT";
    $transactionType = "AUTH";
    $request = new GetThreeDSessionRequest(
        null, $header,
        isset($data["amount"]) ? $data["amount"] : null,
        isset($data["cardId"]) ? $data["cardId"] : null,
        isset($data["cardToken"]) ? $data["cardToken"] : null,
        isset($data["installmentCount"]) ? $data["installmentCount"] : null,
        TestConstants::$merchantCode,
        substr($data["msisdn"],2,10),
        isset($data["referenceNumber"]) ? $data["referenceNumber"] : null,
        $target,
        $transactionType);
    
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        echo post(RestEndpointConstants::$getThreeDSession, $request);
    } else {
        echo json_encode(soap(SoapEndpointConstants::$provisionServices, "getThreeDSession", $request));
    }
}



?>