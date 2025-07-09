<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/refundRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');


if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $referenceNumber = $header->transactionId;
    $request = new RefundRequest(null, $header, TestConstants::$merchantCode, 
        substr($data["msisdn"], 2, 10), $data["originalReferenceNumber"], $referenceNumber, 
        isset($data["amount"]) ? $data["amount"] : null,
        isset($data["pointAmount"]) ? $data["pointAmount"] : null);
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        $resp = json_decode(post(RestEndpointConstants::$refund, $request));
    } else {
        $resp = soap(SoapEndpointConstants::$provisionServices, "refund", $request);
    }
    $resp->originalReferenceNumber = $request->referenceNumber;
    echo json_encode($resp);
}

?>