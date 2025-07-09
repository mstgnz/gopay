<?php
include 'baseInput.php';

class SummaryReconciliationRequest extends BaseInput
{
    public $merchantCode;
    public $reconciliationDate;
    public $totalPostAuthAmount;
    public $totalPostAuthCount;
    public $totalPostAuthReverseAmount;
    public $totalPostAuthReverseCount;
    public $totalPreAuthAmount;
    public $totalPreAuthCount;
    public $totalPreAuthReverseAmount;
    public $totalPreAuthReverseCount;
    public $totalRefundAmount;
    public $totalRefundCount;
    public $totalReverseAmount;
    public $totalReverseCount;
    public $totalSaleAmount;
    public $totalSaleCount;

    public function __construct($extraParameters, $requestHeader, $merchantCode, $reconciliationDate, $totalPostAuthAmount, $totalPostAuthCount, $totalPostAuthReverseAmount, $totalPostAuthReverseCount, $totalPreAuthAmount, $totalPreAuthCount, $totalPreAuthReverseAmount, $totalPreAuthReverseCount, $totalRefundAmount, $totalRefundCount, $totalReverseAmount, $totalReverseCount, $totalSaleAmount, $totalSaleCount)
    {
        parent::__construct($extraParameters, $requestHeader);
        $this->merchantCode = $merchantCode;
        $this->reconciliationDate = $reconciliationDate;
        $this->totalPostAuthAmount = $totalPostAuthAmount;
        $this->totalPostAuthCount = $totalPostAuthCount;
        $this->totalPostAuthReverseAmount = $totalPostAuthReverseAmount;
        $this->totalPostAuthReverseCount = $totalPostAuthReverseCount;
        $this->totalPreAuthAmount = $totalPreAuthAmount;
        $this->totalPreAuthCount = $totalPreAuthCount;
        $this->totalPreAuthReverseAmount = $totalPreAuthReverseAmount;
        $this->totalPreAuthReverseCount = $totalPreAuthReverseCount;
        $this->totalRefundAmount = $totalRefundAmount;
        $this->totalRefundCount = $totalRefundCount;
        $this->totalReverseAmount = $totalReverseAmount;
        $this->totalReverseCount = $totalReverseCount;
        $this->totalSaleAmount = $totalSaleAmount;
        $this->totalSaleCount = $totalSaleCount;
    }
}
?>