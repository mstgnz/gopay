<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/inquireAllRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');


if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $referenceNumber = $header->transactionId;
    $request = new InquireAllRequest(null, $header, TestConstants::$merchantCode, 
        substr($data["msisdn"],2,10), $data["originalReferenceNumber"], $referenceNumber,
        isset($data["paymentMethodType"]) ? $data["paymentMethodType"] : null);
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        $resp = json_decode(post(RestEndpointConstants::$inquireAll, $request));
    } else {
        $resp = soap(SoapEndpointConstants::$provisionServices, "inquireAll", $request);
    }
    $resp->originalReferenceNumber = $referenceNumber;
    echo json_encode($resp);
}


?>