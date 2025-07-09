<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/validateOtpRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');


if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $request = new ValidateOtpRequest(null, $header, $data["amount"], substr($data["msisdn"],2,10), $data["referenceNumber"], $data["otp"], $data["token"]);
    
    debug_to_console($request);
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        $resp = post(RestEndpointConstants::$validateOtp, $request);
    } else {
        $resp = json_encode(soap(SoapEndpointConstants::$provisionServices, "validateOtp", $request));
    }
    echo $resp;
}

?>