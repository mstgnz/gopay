<?php 
class CustomHash {
    static function hashRequest($transactionId, $transactionDateTime): string{
        $securityData = CustomHash::hash(TestConstants::$applicationPwd.TestConstants::$applicationName);
        $hashData = CustomHash::hash(TestConstants::$applicationName.$transactionId.$transactionDateTime
            .TestConstants::$secureCode.$securityData);
        return $hashData;
    }
    static function hashResponse($transactionId, $responseDateTime, $responseCode, $cardToken): string{
        $securityData = CustomHash::hash(TestConstants::$applicationPwd.TestConstants::$applicationName);
        $hashData = CustomHash::hash(TestConstants::$applicationName.$transactionId.$responseDateTime
            .$responseCode.$cardToken.TestConstants::$secureCode.$securityData);
        return $hashData;
    }
    static function hash($param): string {
        $upper = strtoupper($param);
        $hashed = hash('sha256', $upper, true);
        $encoded = base64_encode($hashed);
        return $encoded;
    }
}
?>