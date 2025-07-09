<?php
include '../util.php';
include '../endpointConstants.php';
include '../model/provisionForMarketPlaceRequest.php';
include '../model/requestHeader.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');


if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    $header = RequestHeader::createTestHeader();
    $acquirerBankCode = null;
    $merchantCode = TestConstants::$merchantCode;
    $request = new ProvisionForMarketPlaceRequest(null, $header, 
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
        $header->transactionId, 
        isset($data["threeDSessionId"]) ? $data["threeDSessionId"] : null,
        isset($data["customerEmail"]) ? $data["customerEmail"] : null,
        isset($data["subMerchants"]) ? $data["subMerchants"] : null,
        ); 
    
    if(isset($data["webCallMethod"]) && $data["webCallMethod"] == "REST"){
        $resp = json_decode(post(RestEndpointConstants::$provisionForMarketPlace, $request));
    } else {
        $resp = soap(SoapEndpointConstants::$provisionServices, "provisionForMarketPlace", $request);
    }
    $resp->originalReferenceNumber = $request->referenceNumber;
    echo json_encode($resp);
}

?>