<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/reverseRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');


if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $referenceNumber = $header->transactionId;
    $request = new ReverseRequest(null, $header, TestConstants::$merchantCode, 
        substr($data["msisdn"], 2, 10), $data["originalReferenceNumber"], $referenceNumber);
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        $resp = json_decode(post(RestEndpointConstants::$reverse, $request));
    } else {
        $resp = soap(SoapEndpointConstants::$provisionServices, "reverse", $request);
    }
    $resp->originalReferenceNumber = $request->referenceNumber;
    echo json_encode($resp);
}

?>