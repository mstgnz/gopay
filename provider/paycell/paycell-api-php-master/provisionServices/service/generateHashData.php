<?php 
include '../testConstants.php';
include '../customHash.php';

header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Method: POST');
header('Access-Control-Allow-Headers: access-control-request-origin,content-type');

if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    if(isset($data["transactionDateTime"])){
        $hashData = CustomHash::hashRequest($data["transactionId"], $data["transactionDateTime"]);
    } else if(isset($data["responseDateTime"])){
        $hashData = CustomHash::hashResponse($data["transactionId"], $data["responseDateTime"], 
            $data["responseCode"], $data["cardToken"]);
    }
    echo json_encode(array("hashData" => $hashData));
}

?>