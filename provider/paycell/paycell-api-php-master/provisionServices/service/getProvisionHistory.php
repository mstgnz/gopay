<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/getProvisionHistoryRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');


if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $reconciliationDate = new DateTime($data["reconciliationDate"]);
    $reconciliationDate = date_format($reconciliationDate, "Ymd");
    $request = new GetProvisionHistoryRequest(null, $header, TestConstants::$merchantCode, $data["partitionNo"], $reconciliationDate);
    
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        echo post(RestEndpointConstants::$getProvisionHistory, $request);
    } else {
        echo json_encode(soap(SoapEndpointConstants::$provisionServices, "getProvisionHistory", $request));
    }
}


?>