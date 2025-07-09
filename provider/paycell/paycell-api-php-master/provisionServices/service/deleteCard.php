<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/deleteCardRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');

if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $request = new DeleteCardRequest(null, $header, $data["cardId"], $data["msisdn"]);
    
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        echo post(RestEndpointConstants::$deleteCard, $request);
    } else {
        echo json_encode(soap(SoapEndpointConstants::$provisionServices, "deleteCard", $request));
    }
}


?>