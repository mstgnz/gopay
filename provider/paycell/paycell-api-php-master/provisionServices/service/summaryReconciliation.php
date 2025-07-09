<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/summaryReconciliationRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');


if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $reconciliationDate = new DateTime($data["reconciliationDate"]);
    $reconciliationDate = date_format($reconciliationDate, "Ymd");
    $request = new SummaryReconciliationRequest(null, $header, TestConstants::$merchantCode, 
        $reconciliationDate, 
        $data["totalPostAuthAmount"], 
        $data["totalPostAuthCount"], 
        $data["totalPostAuthReverseAmount"], 
        $data["totalPostAuthReverseCount"], 
        $data["totalPreAuthAmount"], 
        $data["totalPreAuthCount"], 
        $data["totalPreAuthReverseAmount"], 
        $data["totalPreAuthReverseCount"], 
        $data["totalRefundAmount"], 
        $data["totalRefundCount"], 
        $data["totalReverseAmount"], 
        $data["totalReverseCount"], 
        $data["totalSaleAmount"], 
        $data["totalSaleCount"]);    
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        echo post(RestEndpointConstants::$summaryReconciliation, $request);
    } else {
        echo json_encode(soap(SoapEndpointConstants::$provisionServices, "summaryReconciliation", $request));
    }
}


?>