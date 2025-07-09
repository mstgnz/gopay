<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/sendOtpRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');


if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $referenceNumber = $header->transactionId;
    $request = new SendOtpRequest(null, $header, $data["amount"], substr($data["msisdn"],2,10), $referenceNumber);
    
    debug_to_console($request);
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        $resp = json_decode(post(RestEndpointConstants::$sendOtp, $request));
    } else {
        $resp = soap(SoapEndpointConstants::$provisionServices, "sendOtp", $request);
    }
    $resp->originalReferenceNumber = $request->referenceNumber;
    echo json_encode($resp);
}

?>