<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/provisionAllRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');


if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $acquirerBankCode = null;
    $merchantCode = TestConstants::$merchantCode;
    $request = new ProvisionAllRequest(null, $header, 
        $acquirerBankCode, $data["amount"], 
        isset($data["cardId"]) ? $data["cardId"] : null, 
        isset($data["cardToken"]) ? $data["cardToken"] : null, 
        $data["currency"],
        isset($data["installmentCount"]) ? $data["installmentCount"] : null, 
        $merchantCode, substr($data["msisdn"],2,10), 
        isset($data["originalReferenceNumber"]) ? $data["originalReferenceNumber"] : null, 
        $data["paymentType"], 
        isset($data["pin"]) ? $data["pin"] : null, 
        isset($data["pointAmount"]) ? $data["pointAmount"] : null, 
        isset($data["referenceNumber"]) ? $data["referenceNumber"] : $header->transactionId, 
        isset($data["threeDSessionId"]) ? $data["threeDSessionId"] : null,
        $data["paymentMethodType"]); 
    
    debug_to_console($request);
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        $resp = json_decode(post(RestEndpointConstants::$provisionAll, $request));
    } else {
        $resp = soap(SoapEndpointConstants::$provisionServices, "provisionAll", $request);
    }
    $resp->originalReferenceNumber = $request->referenceNumber;
    echo json_encode($resp);
}

?>